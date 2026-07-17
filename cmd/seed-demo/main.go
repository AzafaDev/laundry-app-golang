package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// demoPassword is shared by every seeded employee/customer so a portfolio
// reviewer can log in as anyone with one memorized password (documented in
// README.md alongside `make seed-demo`).
const demoPassword = "demo123"

type outletSeed struct {
	name     string
	address  string
	lat, lon float64
	radiusKM float64
}

type employeeSeed struct {
	fullName string
	email    string
	role     string
	outlet   string // outlet name, "" for none (super_admin)
}

type customerSeed struct {
	fullName string
	email    string
	phone    string
	label    string
	address  string
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		log.Fatalf("failed to hash demo password: %v", err)
	}

	outlets := map[string]string{} // name -> id
	for _, o := range []outletSeed{
		{"Laundry Kilat - Kemang", "Jl. Kemang Raya No. 10, Jakarta Selatan", -6.263300, 106.814400, 8},
		{"Laundry Kilat - Sudirman", "Jl. Jend. Sudirman Kav. 25, Jakarta Pusat", -6.208200, 106.822400, 6},
	} {
		id, err := ensureOutlet(ctx, pool, o)
		if err != nil {
			log.Fatalf("failed to seed outlet %q: %v", o.name, err)
		}
		outlets[o.name] = id
		log.Printf("outlet ready: %s (%s)", o.name, id)
	}

	shiftID, err := ensureWorkShift(ctx, pool, "Shift Harian Demo")
	if err != nil {
		log.Fatalf("failed to seed work shift: %v", err)
	}

	employeeIDs := map[string]string{} // email -> id
	for _, e := range []employeeSeed{
		{"Budi Santoso", "admin@demo.laundry", "super_admin", ""},
		{"Siti Aminah", "kemang.admin@demo.laundry", "outlet_admin", "Laundry Kilat - Kemang"},
		{"Rahmat Hidayat", "sudirman.admin@demo.laundry", "outlet_admin", "Laundry Kilat - Sudirman"},
		{"Dewi Lestari", "kemang.washing@demo.laundry", "washing_worker", "Laundry Kilat - Kemang"},
		{"Agus Wijaya", "sudirman.washing@demo.laundry", "washing_worker", "Laundry Kilat - Sudirman"},
		{"Fitriani", "kemang.ironing@demo.laundry", "ironing_worker", "Laundry Kilat - Kemang"},
		{"Hendra Gunawan", "sudirman.ironing@demo.laundry", "ironing_worker", "Laundry Kilat - Sudirman"},
		{"Yuni Kartika", "kemang.packing@demo.laundry", "packing_worker", "Laundry Kilat - Kemang"},
		{"Joko Prasetyo", "sudirman.packing@demo.laundry", "packing_worker", "Laundry Kilat - Sudirman"},
		{"Dedi Kurniawan", "kemang.driver@demo.laundry", "driver", "Laundry Kilat - Kemang"},
		{"Rudi Setiawan", "sudirman.driver@demo.laundry", "driver", "Laundry Kilat - Sudirman"},
	} {
		var outletID *string
		if e.outlet != "" {
			id := outlets[e.outlet]
			outletID = &id
		}

		id, err := ensureEmployee(ctx, pool, e.fullName, e.email, hash, e.role, outletID)
		if err != nil {
			log.Fatalf("failed to seed employee %q: %v", e.email, err)
		}
		employeeIDs[e.email] = id
		log.Printf("employee ready: %s <%s> (%s)", e.fullName, e.email, e.role)

		if e.role != "super_admin" {
			if err := ensureEmployeeShiftToday(ctx, pool, id, shiftID, outlets[e.outlet]); err != nil {
				log.Fatalf("failed to seed today's shift for %q: %v", e.email, err)
			}
		}
	}

	provinceID, cityID, districtID, err := lookupWilayahIDs(ctx, pool)
	if err != nil {
		log.Fatalf("failed to look up wilayah reference IDs: %v", err)
	}

	customerIDs := map[string]string{} // email -> id
	addressIDs := map[string]string{}  // email -> address id
	for _, c := range []customerSeed{
		{"Andi Saputra", "andi@demo.customer", "081211112222", "Rumah", "Jl. Melati No. 5, Jakarta Selatan"},
		{"Maya Puspita", "maya@demo.customer", "081233334444", "Rumah", "Jl. Anggrek No. 12, Jakarta Selatan"},
		{"Bayu Firmansyah", "bayu@demo.customer", "081255556666", "Kantor", "Jl. Mawar No. 8, Jakarta Pusat"},
		{"Rina Marlina", "rina@demo.customer", "081277778888", "Rumah", "Jl. Kenanga No. 3, Jakarta Pusat"},
	} {
		id, err := ensureCustomer(ctx, pool, c.fullName, c.email, hash, c.phone)
		if err != nil {
			log.Fatalf("failed to seed customer %q: %v", c.email, err)
		}
		customerIDs[c.email] = id
		log.Printf("customer ready: %s <%s>", c.fullName, c.email)

		addrID, err := ensureCustomerAddress(ctx, pool, id, c.label, c.address, provinceID, cityID, districtID)
		if err != nil {
			log.Fatalf("failed to seed address for %q: %v", c.email, err)
		}
		addressIDs[c.email] = addrID
	}

	clothingTypeID, err := ensureClothingType(ctx, pool, "Kemeja")
	if err != nil {
		log.Fatalf("failed to seed clothing type: %v", err)
	}
	laundryItemID, err := ensureLaundryItem(ctx, pool, "Cuci Kiloan", "kg", 7000)
	if err != nil {
		log.Fatalf("failed to seed laundry item: %v", err)
	}

	kemangOutlet := outlets["Laundry Kilat - Kemang"]
	sudirmanOutlet := outlets["Laundry Kilat - Sudirman"]
	kemangAdmin := employeeIDs["kemang.admin@demo.laundry"]

	type demoOrder struct {
		invoice    string
		customer   string
		outlet     string
		address    string
		status     string
		totalPrice float64
	}
	orders := []demoOrder{
		{"DEMO-0001", "andi@demo.customer", kemangOutlet, "andi@demo.customer", "waiting_pickup_driver", 35000},
		{"DEMO-0002", "maya@demo.customer", kemangOutlet, "maya@demo.customer", "washing", 42000},
		{"DEMO-0003", "bayu@demo.customer", sudirmanOutlet, "bayu@demo.customer", "packing", 56000},
		{"DEMO-0004", "rina@demo.customer", sudirmanOutlet, "rina@demo.customer", "waiting_payment", 63000},
		{"DEMO-0005", "andi@demo.customer", kemangOutlet, "andi@demo.customer", "ready_for_delivery", 48000},
		{"DEMO-0006", "maya@demo.customer", kemangOutlet, "maya@demo.customer", "completed", 39000},
	}

	for _, o := range orders {
		orderID, created, err := ensureOrder(ctx, pool, o.invoice, customerIDs[o.customer], o.outlet, addressIDs[o.address], o.status, o.totalPrice)
		if err != nil {
			log.Fatalf("failed to seed order %q: %v", o.invoice, err)
		}
		if !created {
			log.Printf("order already exists, skipped: %s", o.invoice)
			continue
		}
		log.Printf("order seeded: %s (%s)", o.invoice, o.status)

		if err := seedOrderContents(ctx, pool, orderID, clothingTypeID, laundryItemID, kemangAdmin, o.status); err != nil {
			log.Fatalf("failed to seed contents for order %q: %v", o.invoice, err)
		}
	}

	log.Println("demo data seeding complete")
}

func ensureOutlet(ctx context.Context, pool *pgxpool.Pool, o outletSeed) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM outlets WHERE name = $1", o.name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO outlets (name, address, latitude, longitude, is_active, service_radius_km)
		VALUES ($1, $2, $3, $4, TRUE, $5)
		RETURNING id
	`, o.name, o.address, o.lat, o.lon, o.radiusKM).Scan(&id)
	return id, err
}

func ensureWorkShift(ctx context.Context, pool *pgxpool.Pool, name string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM work_shifts WHERE name = $1 AND deleted_at IS NULL", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO work_shifts (name, start_time, end_time, is_active)
		VALUES ($1, '00:00:00', '23:59:59', TRUE)
		RETURNING id
	`, name).Scan(&id)
	return id, err
}

func ensureEmployee(ctx context.Context, pool *pgxpool.Pool, fullName, email, passwordHash, role string, outletID *string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO employees (full_name, email, password_hash, role, is_active, outlet_id)
		VALUES ($1, $2, $3, $4, TRUE, $5)
		ON CONFLICT (email) DO NOTHING
		RETURNING id
	`, fullName, email, passwordHash, role, outletID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, "SELECT id FROM employees WHERE email = $1", email).Scan(&id)
	return id, err
}

func ensureEmployeeShiftToday(ctx context.Context, pool *pgxpool.Pool, employeeID, shiftID, outletID string) error {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM employee_shifts WHERE employee_id = $1 AND date = CURRENT_DATE", employeeID).Scan(&id)
	if err == nil {
		return nil
	}
	if err != pgx.ErrNoRows {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO employee_shifts (employee_id, shift_id, outlet_id, date, is_active)
		VALUES ($1, $2, $3, CURRENT_DATE, TRUE)
	`, employeeID, shiftID, outletID)
	return err
}

func ensureCustomer(ctx context.Context, pool *pgxpool.Pool, fullName, email, passwordHash, phone string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO customers (full_name, email, password_hash, phone, is_verified)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT (email) DO NOTHING
		RETURNING id
	`, fullName, email, passwordHash, phone).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, "SELECT id FROM customers WHERE email = $1", email).Scan(&id)
	return id, err
}

func ensureCustomerAddress(ctx context.Context, pool *pgxpool.Pool, customerID, label, address string, provinceID, cityID, districtID int32) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM customer_addresses WHERE customer_id = $1 LIMIT 1", customerID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	// Same coordinates as the Kemang outlet — keeps every seeded order
	// within its nearest-outlet radius without needing per-address tuning.
	err = pool.QueryRow(ctx, `
		INSERT INTO customer_addresses (customer_id, label, address, province_id, city_id, district_id, latitude, longitude, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6, -6.263300, 106.814400, TRUE)
		RETURNING id
	`, customerID, label, address, provinceID, cityID, districtID).Scan(&id)
	return id, err
}

func ensureClothingType(ctx context.Context, pool *pgxpool.Pool, name string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM clothing_types WHERE name = $1 AND deleted_at IS NULL", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, "INSERT INTO clothing_types (name, is_active) VALUES ($1, TRUE) RETURNING id", name).Scan(&id)
	return id, err
}

func ensureLaundryItem(ctx context.Context, pool *pgxpool.Pool, name, unit string, basePrice float64) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM laundry_items WHERE name = $1 AND deleted_at IS NULL", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO laundry_items (name, unit, base_price, is_active)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id
	`, name, unit, basePrice).Scan(&id)
	return id, err
}

// ensureOrder returns the order's id and whether it was newly created in
// this run (false means it already existed and its contents were already
// seeded by a prior run, so the caller should skip re-seeding children).
func ensureOrder(ctx context.Context, pool *pgxpool.Pool, invoiceNumber, customerID, outletID, addressID, status string, totalPrice float64) (id string, created bool, err error) {
	err = pool.QueryRow(ctx, `
		INSERT INTO orders (invoice_number, customer_id, outlet_id, pickup_address_id, status, pickup_date, delivery_fee, total_price)
		VALUES ($1, $2, $3, $4, $5, CURRENT_DATE, 0, $6)
		ON CONFLICT (invoice_number) DO NOTHING
		RETURNING id
	`, invoiceNumber, customerID, outletID, addressID, status, totalPrice).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, err
	}

	err = pool.QueryRow(ctx, "SELECT id FROM orders WHERE invoice_number = $1", invoiceNumber).Scan(&id)
	return id, false, err
}

// seedOrderContents adds order_items/order_item_breakdowns so the order
// doesn't look empty in the UI, plus whatever driver_tasks/payments row
// matches its pipeline stage.
func seedOrderContents(ctx context.Context, pool *pgxpool.Pool, orderID, clothingTypeID, laundryItemID, createdByEmployeeID, status string) error {
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, laundry_item_id, quantity, price_at_order)
		VALUES ($1, $2, 3, 7000)
	`, orderID, laundryItemID); err != nil {
		return fmt.Errorf("order_items: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO order_item_breakdowns (order_id, clothing_type_id, quantity, created_by)
		VALUES ($1, $2, 5, $3)
	`, orderID, clothingTypeID, createdByEmployeeID); err != nil {
		return fmt.Errorf("order_item_breakdowns: %w", err)
	}

	switch status {
	case "waiting_pickup_driver":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", false); err != nil {
			return err
		}

	case "washing", "packing":
		// These stages imply the order has already been picked up from the
		// customer, so a bare order with no driver_tasks at all would be
		// incoherent with its own status.
		if err := seedDriverTask(ctx, pool, orderID, "pickup", true); err != nil {
			return err
		}

	case "waiting_payment":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", true); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "pending", false); err != nil {
			return err
		}

	case "ready_for_delivery":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", true); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "paid", true); err != nil {
			return err
		}
		if err := seedDriverTask(ctx, pool, orderID, "delivery", false); err != nil {
			return err
		}

	case "completed":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", true); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "paid", true); err != nil {
			return err
		}
		if err := seedDriverTask(ctx, pool, orderID, "delivery", true); err != nil {
			return err
		}
	}

	return nil
}

func seedDriverTask(ctx context.Context, pool *pgxpool.Pool, orderID, taskType string, completed bool) error {
	var err error
	if completed {
		_, err = pool.Exec(ctx, `
			INSERT INTO driver_tasks (order_id, task_type, status, taken_at, completed_at)
			VALUES ($1, $2, 'completed', now(), now())
		`, orderID, taskType)
	} else {
		_, err = pool.Exec(ctx, `
			INSERT INTO driver_tasks (order_id, task_type, status)
			VALUES ($1, $2, 'available')
		`, orderID, taskType)
	}
	if err != nil {
		return fmt.Errorf("driver_tasks(%s): %w", taskType, err)
	}
	return nil
}

func seedPayment(ctx context.Context, pool *pgxpool.Pool, orderID, status string, paid bool) error {
	var paidAt any
	if paid {
		paidAt = time.Now()
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO payments (order_id, amount, gateway_name, gateway_transaction_id, payment_link, status, expired_at, paid_at)
		VALUES ($1, (SELECT total_price FROM orders WHERE id = $1), 'midtrans', $1::text, 'https://example.com/pay/demo', $2, now() + interval '24 hours', $3)
		ON CONFLICT (order_id) DO NOTHING
	`, orderID, status, paidAt)
	if err != nil {
		return fmt.Errorf("payments: %w", err)
	}
	return nil
}

func lookupWilayahIDs(ctx context.Context, pool *pgxpool.Pool) (provinceID, cityID, districtID int32, err error) {
	row := pool.QueryRow(ctx, `
		SELECT p.id, c.id, d.id
		FROM districts d
		JOIN cities c ON c.id = d.city_id
		JOIN provinces p ON p.id = c.province_id
		WHERE p.name = 'DKI JAKARTA'
		LIMIT 1
	`)
	err = row.Scan(&provinceID, &cityID, &districtID)
	return provinceID, cityID, districtID, err
}
