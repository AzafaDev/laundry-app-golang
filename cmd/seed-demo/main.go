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
}

type addressSeed struct {
	customerEmail string
	label         string
	address       string
	lat, lon      float64
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
		{"Laundry Kilat - Curug", "Jl. Komp. Tataka Puri Blok J5 No.10, RT.3/RW.5, Kadu, Kec. Curug, Kabupaten Tangerang, Banten 15810", -6.229296504362473, 106.5674849964895, 8},
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
		{"Siti Aminah", "outlet.admin@demo.laundry", "outlet_admin", "Laundry Kilat - Curug"},
		{"Dewi Lestari", "washing@demo.laundry", "washing_worker", "Laundry Kilat - Curug"},
		{"Fitriani", "ironing@demo.laundry", "ironing_worker", "Laundry Kilat - Curug"},
		{"Yuni Kartika", "packing@demo.laundry", "packing_worker", "Laundry Kilat - Curug"},
		{"Dedi Kurniawan", "driver@demo.laundry", "driver", "Laundry Kilat - Curug"},
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

	customers := []customerSeed{
		{"Rina Marlina", "rina@demo.customer", "081277778888"},
		{"Clean Testing", "clean@demo.customer", "081200000000"}, // sengaja 0 order, buat test alur Pickup baru
	}

	// Jarak dihitung dari outlet Curug (-6.229296504362473, 106.5674849964895,
	// radius layanan 8km): near ~3km (gratis ongkir, di dalam radius), mid
	// ~6.8km (kena ongkir flat, masih di dalam radius), far ~16.6km (di luar
	// radius outlet sama sekali -> no_outlet_in_range saat dipakai bikin order baru).
	addresses := []addressSeed{
		{"rina@demo.customer", "Rumah", "Jl. Curug Wetan Raya, Kec. Curug, Kab. Tangerang, Banten", -6.2103, 106.5875},
		{"rina@demo.customer", "Kantor", "Jl. Raya Legok, Kec. Legok, Kab. Tangerang, Banten", -6.1850, 106.6100},
		{"rina@demo.customer", "Rumah Orang Tua", "Jl. Raya Serpong, BSD City, Kec. Serpong, Tangerang Selatan, Banten", -6.1300, 106.6800},
		{"clean@demo.customer", "Rumah", "Jl. Curug Wetan Raya, Kec. Curug, Kab. Tangerang, Banten", -6.2103, 106.5875},
	}

	customerIDs := map[string]string{}
	addressIDs := map[string]map[string]string{} // email -> label -> address id

	for _, c := range customers {
		id, err := ensureCustomer(ctx, pool, c.fullName, c.email, hash, c.phone)
		if err != nil {
			log.Fatalf("failed to seed customer %q: %v", c.email, err)
		}
		customerIDs[c.email] = id
		addressIDs[c.email] = map[string]string{}
		log.Printf("customer ready: %s <%s>", c.fullName, c.email)
	}

	for _, a := range addresses {
		addrID, err := ensureCustomerAddress(ctx, pool, customerIDs[a.customerEmail], a.label, a.address, provinceID, cityID, districtID, a.lat, a.lon)
		if err != nil {
			log.Fatalf("failed to seed address %q for %q: %v", a.label, a.customerEmail, err)
		}
		addressIDs[a.customerEmail][a.label] = addrID
	}

	clothingTypeID, err := ensureClothingType(ctx, pool, "Kemeja")
	if err != nil {
		log.Fatalf("failed to seed clothing type: %v", err)
	}
	laundryItemID, err := ensureLaundryItem(ctx, pool, "Cuci Kiloan", "kg", 7000)
	if err != nil {
		log.Fatalf("failed to seed laundry item: %v", err)
	}

	curugOutlet := outlets["Laundry Kilat - Curug"]
	curugAdmin := employeeIDs["outlet.admin@demo.laundry"]
	rinaID := customerIDs["rina@demo.customer"]
	rinaAddr := addressIDs["rina@demo.customer"]

	type demoOrder struct {
		invoice    string
		customer   string
		outlet     string
		address    string
		status     string
		totalPrice float64
	}
	orders := []demoOrder{
		{"DEMO-0001", rinaID, curugOutlet, rinaAddr["Rumah"], "waiting_pickup_driver", 35000},
		{"DEMO-0002", rinaID, curugOutlet, rinaAddr["Rumah"], "laundry_to_outlet", 38000},
		{"DEMO-0003", rinaID, curugOutlet, rinaAddr["Kantor"], "laundry_arrived_outlet", 41000},
		{"DEMO-0004", rinaID, curugOutlet, rinaAddr["Kantor"], "washing", 42000},
		{"DEMO-0005", rinaID, curugOutlet, rinaAddr["Rumah Orang Tua"], "ironing", 44000},
		{"DEMO-0006", rinaID, curugOutlet, rinaAddr["Rumah"], "packing", 56000},
		{"DEMO-0007", rinaID, curugOutlet, rinaAddr["Kantor"], "waiting_payment", 63000},
		{"DEMO-0008", rinaID, curugOutlet, rinaAddr["Rumah"], "ready_for_delivery", 48000},
		{"DEMO-0009", rinaID, curugOutlet, rinaAddr["Kantor"], "delivery_to_customer", 52000},
		{"DEMO-0010", rinaID, curugOutlet, rinaAddr["Rumah"], "received_by_customer", 45000},
		{"DEMO-0011", rinaID, curugOutlet, rinaAddr["Rumah"], "completed", 39000},
	}

	for _, o := range orders {
		orderID, created, err := ensureOrder(ctx, pool, o.invoice, o.customer, o.outlet, o.address, o.status, o.totalPrice)
		if err != nil {
			log.Fatalf("failed to seed order %q: %v", o.invoice, err)
		}
		if !created {
			log.Printf("order already exists, skipped: %s", o.invoice)
			continue
		}
		log.Printf("order seeded: %s (%s)", o.invoice, o.status)

		if err := seedOrderContents(ctx, pool, orderID, clothingTypeID, laundryItemID, curugAdmin, o.status); err != nil {
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

func ensureCustomerAddress(ctx context.Context, pool *pgxpool.Pool, customerID, label, address string, provinceID, cityID, districtID int32, lat, lon float64) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM customer_addresses WHERE customer_id = $1 AND label = $2", customerID, label).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	isPrimary := label == "Rumah" // first/home address stays primary; others aren't

	err = pool.QueryRow(ctx, `
		INSERT INTO customer_addresses (customer_id, label, address, province_id, city_id, district_id, latitude, longitude, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, customerID, label, address, provinceID, cityID, districtID, lat, lon, isPrimary).Scan(&id)
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
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "available"); err != nil {
			return err
		}

	case "laundry_to_outlet":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "in_progress"); err != nil {
			return err
		}

	case "laundry_arrived_outlet", "washing", "ironing", "packing":
		// These stages imply the order has already been picked up from the
		// customer, so a bare order with no driver_tasks at all would be
		// incoherent with its own status.
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "completed"); err != nil {
			return err
		}

	case "waiting_payment":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "completed"); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "pending", false); err != nil {
			return err
		}

	case "ready_for_delivery":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "completed"); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "paid", true); err != nil {
			return err
		}
		if err := seedDriverTask(ctx, pool, orderID, "delivery", "available"); err != nil {
			return err
		}

	case "delivery_to_customer":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "completed"); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "paid", true); err != nil {
			return err
		}
		if err := seedDriverTask(ctx, pool, orderID, "delivery", "in_progress"); err != nil {
			return err
		}

	case "received_by_customer", "completed":
		if err := seedDriverTask(ctx, pool, orderID, "pickup", "completed"); err != nil {
			return err
		}
		if err := seedPayment(ctx, pool, orderID, "paid", true); err != nil {
			return err
		}
		if err := seedDriverTask(ctx, pool, orderID, "delivery", "completed"); err != nil {
			return err
		}
	}

	return nil
}

func seedDriverTask(ctx context.Context, pool *pgxpool.Pool, orderID, taskType, stage string) error {
	var err error
	switch stage {
	case "completed":
		_, err = pool.Exec(ctx, `
			INSERT INTO driver_tasks (order_id, task_type, status, taken_at, completed_at)
			VALUES ($1, $2, 'completed', now(), now())
		`, orderID, taskType)
	case "in_progress":
		_, err = pool.Exec(ctx, `
			INSERT INTO driver_tasks (order_id, task_type, status, taken_at)
			VALUES ($1, $2, 'in_progress', now())
		`, orderID, taskType)
	default:
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
