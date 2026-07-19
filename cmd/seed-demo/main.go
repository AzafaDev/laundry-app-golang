package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/database"
	"laundry-app-with-golang/internal/shift"

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
	shift    string // "pagi" or "sore"
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
		{"Laundry Kilat - BSD", "Jl. Pahlawan Seribu, BSD City, Kec. Serpong, Tangerang Selatan, Banten 15321", -6.301930450570917, 106.65282660000001, 8},
	} {
		id, err := ensureOutlet(ctx, pool, o)
		if err != nil {
			log.Fatalf("failed to seed outlet %q: %v", o.name, err)
		}
		outlets[o.name] = id
		log.Printf("outlet ready: %s (%s)", o.name, id)
	}

	morningShiftID, err := ensureWorkShift(ctx, pool, "Shift Pagi", "06:00:00", "14:00:00")
	if err != nil {
		log.Fatalf("failed to seed morning shift: %v", err)
	}
	eveningShiftID, err := ensureWorkShift(ctx, pool, "Shift Sore", "14:00:00", "22:00:00")
	if err != nil {
		log.Fatalf("failed to seed evening shift: %v", err)
	}

	employeeIDs := map[string]string{} // email -> id
	for _, e := range []employeeSeed{
		{"Budi Santoso", "admin@demo.laundry", "super_admin", "", ""},
		{"Siti Aminah", "outlet.admin@demo.laundry", "outlet_admin", "Laundry Kilat - Curug", "pagi"},
		{"Dewi Lestari", "washing@demo.laundry", "washing_worker", "Laundry Kilat - Curug", "pagi"},
		{"Rian Saputra", "washing.sore@demo.laundry", "washing_worker", "Laundry Kilat - Curug", "sore"},
		{"Fitriani", "ironing@demo.laundry", "ironing_worker", "Laundry Kilat - Curug", "pagi"},
		{"Bayu Aji", "ironing.sore@demo.laundry", "ironing_worker", "Laundry Kilat - Curug", "sore"},
		{"Yuni Kartika", "packing@demo.laundry", "packing_worker", "Laundry Kilat - Curug", "pagi"},
		{"Nanda Putri", "packing.sore@demo.laundry", "packing_worker", "Laundry Kilat - Curug", "sore"},
		{"Dedi Kurniawan", "driver@demo.laundry", "driver", "Laundry Kilat - Curug", "pagi"},
		{"Eko Prasetyo", "driver.sore@demo.laundry", "driver", "Laundry Kilat - Curug", "sore"},
		{"Rizky Ramadhan", "outlet.admin.bsd@demo.laundry", "outlet_admin", "Laundry Kilat - BSD", "pagi"},
		{"Sri Wahyuni", "washing.bsd@demo.laundry", "washing_worker", "Laundry Kilat - BSD", "pagi"},
		{"Agus Santoso", "ironing.bsd@demo.laundry", "ironing_worker", "Laundry Kilat - BSD", "pagi"},
		{"Lestari Wulandari", "packing.bsd@demo.laundry", "packing_worker", "Laundry Kilat - BSD", "pagi"},
		{"Hendra Gunawan", "driver.bsd@demo.laundry", "driver", "Laundry Kilat - BSD", "pagi"},
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
			shiftID := morningShiftID
			if e.shift == "sore" {
				shiftID = eveningShiftID
			}
			if err := ensureEmployeeShiftRecurring(ctx, pool, id, shiftID, outlets[e.outlet]); err != nil {
				log.Fatalf("failed to seed shift for %q: %v", e.email, err)
			}
			// Skip attendance for washing.sore employee to demonstrate absent case in report
			if e.email != "washing.sore@demo.laundry" {
				if err := ensureAttendanceToday(ctx, pool, id, outlets[e.outlet]); err != nil {
					log.Fatalf("failed to seed today's attendance for %q: %v", e.email, err)
				}
			}
		}
	}

	// Seed an 'absent' attendance for Rian Saputra (washing.sore) so reviewers see
	// at least one "not present" case in today's attendance report
	if err := seedTodayAbsent(ctx, pool, employeeIDs["washing.sore@demo.laundry"], outlets["Laundry Kilat - Curug"]); err != nil {
		log.Fatalf("failed to seed today's absent attendance: %v", err)
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

	clothingTypeNames := []string{"Kemeja", "Celana", "Jaket", "Selimut", "Handuk", "Gaun", "Kaos", "Jas"}
	var clothingTypeID string
	for _, name := range clothingTypeNames {
		id, err := ensureClothingType(ctx, pool, name)
		if err != nil {
			log.Fatalf("failed to seed clothing type %q: %v", name, err)
		}
		if clothingTypeID == "" {
			clothingTypeID = id // first one stays the default used by seedOrderContents
		}
		log.Printf("clothing type ready: %s", name)
	}

	type laundryItemSeed struct {
		name      string
		unit      string
		basePrice float64
	}
	laundryItemSeeds := []laundryItemSeed{
		{"Cuci Kiloan", "kg", 7000},
		{"Cuci Setrika", "kg", 10000},
		{"Setrika Saja", "kg", 5000},
		{"Cuci Sepatu", "pcs", 25000},
		{"Cuci Boneka", "pcs", 20000},
		{"Dry Clean", "pcs", 35000},
	}
	var laundryItemID string
	for _, li := range laundryItemSeeds {
		id, err := ensureLaundryItem(ctx, pool, li.name, li.unit, li.basePrice)
		if err != nil {
			log.Fatalf("failed to seed laundry item %q: %v", li.name, err)
		}
		if laundryItemID == "" {
			laundryItemID = id // first one stays the default used by seedOrderContents
		}
		log.Printf("laundry item ready: %s (%s, Rp%.0f)", li.name, li.unit, li.basePrice)
	}

	curugOutlet := outlets["Laundry Kilat - Curug"]
	bsdOutlet := outlets["Laundry Kilat - BSD"]
	curugAdmin := employeeIDs["outlet.admin@demo.laundry"]
	rinaID := customerIDs["rina@demo.customer"]
	rinaAddr := addressIDs["rina@demo.customer"]

	// curugActors/bsdActors map a pipeline role to the employee id that
	// plays it at that outlet, so order_status_histories/order_item_breakdowns
	// attribute each transition to a real employee actually assigned to
	// that order's outlet (rather than always crediting Curug's staff, which
	// would be wrong once BSD orders exist).
	curugActors := buildStationActors(employeeIDs, "")
	bsdActors := buildStationActors(employeeIDs, ".bsd")

	type demoOrder struct {
		invoice    string
		customer   string
		outlet     string
		address    string
		status     string
		totalPrice float64
		actors     map[string]string
	}
	orders := []demoOrder{
		{"DEMO-0001", rinaID, curugOutlet, rinaAddr["Rumah"], "waiting_pickup_driver", 35000, curugActors},
		{"DEMO-0002", rinaID, curugOutlet, rinaAddr["Rumah"], "laundry_to_outlet", 38000, curugActors},
		{"DEMO-0003", rinaID, curugOutlet, rinaAddr["Kantor"], "laundry_arrived_outlet", 41000, curugActors},
		{"DEMO-0004", rinaID, curugOutlet, rinaAddr["Kantor"], "washing", 42000, curugActors},
		{"DEMO-0005", rinaID, curugOutlet, rinaAddr["Rumah Orang Tua"], "ironing", 44000, curugActors},
		{"DEMO-0006", rinaID, curugOutlet, rinaAddr["Rumah"], "packing", 56000, curugActors},
		{"DEMO-0007", rinaID, curugOutlet, rinaAddr["Kantor"], "waiting_payment", 63000, curugActors},
		{"DEMO-0007a", rinaID, curugOutlet, rinaAddr["Rumah"], "waiting_payment", 71000, curugActors},
		{"DEMO-0007b", rinaID, curugOutlet, rinaAddr["Kantor"], "waiting_payment", 58000, curugActors},
		{"DEMO-0008", rinaID, curugOutlet, rinaAddr["Rumah"], "ready_for_delivery", 48000, curugActors},
		{"DEMO-0009", rinaID, curugOutlet, rinaAddr["Kantor"], "delivery_to_customer", 52000, curugActors},
		{"DEMO-0010", rinaID, curugOutlet, rinaAddr["Rumah"], "received_by_customer", 45000, curugActors},
		{"DEMO-0011", rinaID, curugOutlet, rinaAddr["Rumah"], "completed", 39000, curugActors},
		{"DEMO-BSD-0001", rinaID, bsdOutlet, rinaAddr["Rumah"], "laundry_arrived_outlet", 40000, bsdActors},
		{"DEMO-BSD-0002", rinaID, bsdOutlet, rinaAddr["Kantor"], "washing", 46000, bsdActors},
	}

	for _, o := range orders {
		orderID, created, err := ensureOrder(ctx, pool, o.invoice, o.customer, o.outlet, o.address, o.status, o.totalPrice)
		if err != nil {
			log.Fatalf("failed to seed order %q: %v", o.invoice, err)
		}
		if created {
			log.Printf("order seeded: %s (%s)", o.invoice, o.status)

			if err := seedOrderContents(ctx, pool, orderID, clothingTypeID, laundryItemID, o.actors["outlet_admin"], o.status); err != nil {
				log.Fatalf("failed to seed contents for order %q: %v", o.invoice, err)
			}
		} else {
			log.Printf("order already exists, skipped: %s", o.invoice)
		}

		// Independent of `created`: earlier runs of this script (before
		// status-history seeding existed) already created these orders
		// without any history rows, so this must backfill pre-existing
		// orders too, not just freshly-created ones.
		if err := seedOrderStatusHistory(ctx, pool, orderID, rinaID, o.actors, o.status, nil); err != nil {
			log.Fatalf("failed to seed status history for order %q: %v", o.invoice, err)
		}
	}

	// One bypass request per outcome (pending/approved/rejected) so the
	// bypass review UI/endpoints have data to test against — attached to
	// the three orders already sitting at a worker station.
	washingOrderID, err := getOrderIDByInvoice(ctx, pool, "DEMO-0004")
	if err != nil {
		log.Fatalf("failed to look up DEMO-0004 for bypass seeding: %v", err)
	}
	ironingOrderID, err := getOrderIDByInvoice(ctx, pool, "DEMO-0005")
	if err != nil {
		log.Fatalf("failed to look up DEMO-0005 for bypass seeding: %v", err)
	}
	packingOrderID, err := getOrderIDByInvoice(ctx, pool, "DEMO-0006")
	if err != nil {
		log.Fatalf("failed to look up DEMO-0006 for bypass seeding: %v", err)
	}

	if err := ensureBypassRequest(ctx, pool, washingOrderID, "washing", employeeIDs["washing@demo.laundry"], clothingTypeID, "pending", "", ""); err != nil {
		log.Fatalf("failed to seed pending bypass request: %v", err)
	}
	if err := ensureBypassRequest(ctx, pool, ironingOrderID, "ironing", employeeIDs["ironing@demo.laundry"], clothingTypeID, "approved", curugAdmin, "Disetujui, selisih wajar"); err != nil {
		log.Fatalf("failed to seed approved bypass request: %v", err)
	}
	if err := ensureBypassRequest(ctx, pool, packingOrderID, "packing", employeeIDs["packing@demo.laundry"], clothingTypeID, "rejected", curugAdmin, "Ditolak, selisih terlalu besar"); err != nil {
		log.Fatalf("failed to seed rejected bypass request: %v", err)
	}
	log.Println("bypass requests ready")

	// A couple of task notifications for the pagi driver so the driver
	// notifications UI/endpoint has something to show.
	if err := ensureEmployeeNotifications(ctx, pool, employeeIDs["driver@demo.laundry"]); err != nil {
		log.Fatalf("failed to seed driver notifications: %v", err)
	}
	log.Println("driver notifications ready")

	// Backfill historical completed orders so sales/employee-performance
	// report endpoints have multi-day data instead of only today's.
	rng := rand.New(rand.NewSource(42)) // fixed seed for reproducible randomization
	if err := seedHistoricalReportData(ctx, pool, "HIST-CRG", curugOutlet, rinaID, rinaAddr["Rumah"], clothingTypeID, laundryItemID, curugActors, historicalReportDays, rng); err != nil {
		log.Fatalf("failed to seed historical report data for Curug: %v", err)
	}
	if err := seedHistoricalReportData(ctx, pool, "HIST-BSD", bsdOutlet, rinaID, rinaAddr["Rumah"], clothingTypeID, laundryItemID, bsdActors, historicalReportDays, rng); err != nil {
		log.Fatalf("failed to seed historical report data for BSD: %v", err)
	}
	log.Printf("historical report data ready (%d days x 2 outlets)", historicalReportDays)

	// Seed historical attendance data (1.5 years back, excluding today which is already seeded)
	if err := seedHistoricalAttendances(ctx, pool, rng); err != nil {
		log.Fatalf("failed to seed historical attendances: %v", err)
	}
	log.Println("historical attendance data ready")

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

func ensureWorkShift(ctx context.Context, pool *pgxpool.Pool, name, startTime, endTime string) (string, error) {
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
		VALUES ($1, $2, $3, TRUE)
		RETURNING id
	`, name, startTime, endTime).Scan(&id)
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

func ensureEmployeeShiftRecurring(ctx context.Context, pool *pgxpool.Pool, employeeID, shiftID, outletID string) error {
	for day := 0; day <= 6; day++ {
		var id string
		err := pool.QueryRow(ctx, "SELECT id FROM employee_shifts WHERE employee_id = $1 AND day_of_week = $2", employeeID, day).Scan(&id)
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO employee_shifts (employee_id, shift_id, outlet_id, day_of_week, is_active)
			VALUES ($1, $2, $3, $4, TRUE)
		`, employeeID, shiftID, outletID, day); err != nil {
			return err
		}
	}
	return nil
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
// matches its pipeline stage. Skipped for laundry_arrived_outlet: that's the
// exact status ProcessOrder expects to act on, and ProcessOrder 409s with
// "order_already_processed" if order_items already exist — pre-seeding
// items there would make the process-order flow untestable in the browser.
func seedOrderContents(ctx context.Context, pool *pgxpool.Pool, orderID, clothingTypeID, laundryItemID, createdByEmployeeID, status string) error {
	if status != "laundry_arrived_outlet" {
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

// ensureAttendanceToday checks the employee in for today (no checkout), so
// AssertShiftEligibility (which every worker-station endpoint calls) finds
// a valid attendance row instead of rejecting with "not_checked_in" during
// manual testing.
func ensureAttendanceToday(ctx context.Context, pool *pgxpool.Pool, employeeID, outletID string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO attendances (employee_id, outlet_id, date, check_in_time, is_late, late_minutes, status)
		VALUES ($1, $2, CURRENT_DATE, now(), FALSE, 0, 'on_time')
		ON CONFLICT (employee_id, date) DO NOTHING
	`, employeeID, outletID)
	if err != nil {
		return fmt.Errorf("attendances: %w", err)
	}
	return nil
}

// seedTodayAbsent creates an 'absent' attendance record for today for a specific employee
// who was skipped in the ensureAttendanceToday loop. This ensures reviewers see at least
// one "not present" case in the attendance report.
func seedTodayAbsent(ctx context.Context, pool *pgxpool.Pool, employeeID, outletID string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO attendances (employee_id, outlet_id, date, check_in_time, is_late, late_minutes, status)
		VALUES ($1, $2, CURRENT_DATE, NULL, FALSE, 0, 'absent')
		ON CONFLICT (employee_id, date) DO NOTHING
	`, employeeID, outletID)
	if err != nil {
		return fmt.Errorf("seed today absent: %w", err)
	}
	return nil
}

// orderStatusStep describes one order_status_histories row in the pipeline
// a demo order walks through, in order. actorRole is a key into an actors
// map (see buildStationActors) for "employee"-type steps; unused for
// "customer"/"system" steps.
type orderStatusStep struct {
	from, to      string
	changedByType string
	actorRole     string
	note          string
}

var orderStatusPipeline = []orderStatusStep{
	{"", "waiting_pickup_driver", "customer", "", ""},
	{"waiting_pickup_driver", "laundry_to_outlet", "employee", "driver", ""},
	{"laundry_to_outlet", "laundry_arrived_outlet", "employee", "driver", ""},
	{"laundry_arrived_outlet", "washing", "employee", "outlet_admin", ""},
	{"washing", "ironing", "employee", "washing_worker", ""},
	{"ironing", "packing", "employee", "ironing_worker", ""},
	{"packing", "waiting_payment", "employee", "packing_worker", ""},
	{"waiting_payment", "ready_for_delivery", "system", "", "payment confirmed"},
	{"ready_for_delivery", "delivery_to_customer", "employee", "driver", ""},
	{"delivery_to_customer", "received_by_customer", "employee", "driver", ""},
	{"received_by_customer", "completed", "system", "", "Pesanan dikonfirmasi otomatis setelah 2x24 jam."},
}

// buildStationActors maps each pipeline role to the id of the employee
// playing that role at one outlet, keyed by the email suffix that
// distinguishes that outlet's accounts (e.g. "" for Curug's pagi accounts,
// ".bsd" for BSD's). Always resolves to the pagi account for a role, since
// history/content attribution doesn't need shift-level precision.
func buildStationActors(employeeIDs map[string]string, suffix string) map[string]string {
	return map[string]string{
		"driver":         employeeIDs["driver"+suffix+"@demo.laundry"],
		"outlet_admin":   employeeIDs["outlet.admin"+suffix+"@demo.laundry"],
		"washing_worker": employeeIDs["washing"+suffix+"@demo.laundry"],
		"ironing_worker": employeeIDs["ironing"+suffix+"@demo.laundry"],
		"packing_worker": employeeIDs["packing"+suffix+"@demo.laundry"],
	}
}

// seedOrderStatusHistory replays orderStatusPipeline up to (and including)
// the transition that landed the order on finalStatus, so GetStationHistory
// and any order-timeline UI have realistic data instead of an empty list.
// If startAt is non-nil, each row's created_at is stamped explicitly,
// spaced 40 minutes apart from startAt — used by historical report-data
// seeding, where rows must land on their order's actual historical day
// (the WorkerPerformanceReport/SalesReportByPeriod queries filter/group by
// these timestamps) instead of all clustering at "now".
func seedOrderStatusHistory(ctx context.Context, pool *pgxpool.Pool, orderID, customerID string, actors map[string]string, finalStatus string, startAt *time.Time) error {
	var existing string
	err := pool.QueryRow(ctx, "SELECT id FROM order_status_histories WHERE order_id = $1 LIMIT 1", orderID).Scan(&existing)
	if err == nil {
		return nil
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("order_status_histories lookup: %w", err)
	}

	t := startAt
	for _, step := range orderStatusPipeline {
		var changedByID *string
		switch step.changedByType {
		case "customer":
			changedByID = &customerID
		case "employee":
			id := actors[step.actorRole]
			changedByID = &id
		}

		var oldStatus *string
		if step.from != "" {
			oldStatus = &step.from
		}
		var note *string
		if step.note != "" {
			note = &step.note
		}

		if t == nil {
			if _, err := pool.Exec(ctx, `
				INSERT INTO order_status_histories (order_id, old_status, new_status, changed_by_type, changed_by_id, note)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, orderID, oldStatus, step.to, step.changedByType, changedByID, note); err != nil {
				return fmt.Errorf("order_status_histories(%s->%s): %w", step.from, step.to, err)
			}
		} else {
			next := t.Add(40 * time.Minute)
			t = &next
			if _, err := pool.Exec(ctx, `
				INSERT INTO order_status_histories (order_id, old_status, new_status, changed_by_type, changed_by_id, note, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, orderID, oldStatus, step.to, step.changedByType, changedByID, note, *t); err != nil {
				return fmt.Errorf("order_status_histories(%s->%s): %w", step.from, step.to, err)
			}
		}

		if step.to == finalStatus {
			break
		}
	}
	return nil
}

func getOrderIDByInvoice(ctx context.Context, pool *pgxpool.Pool, invoiceNumber string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM orders WHERE invoice_number = $1", invoiceNumber).Scan(&id)
	return id, err
}

// historicalReportDays controls how far back synthetic completed orders are
// generated for report/dashboard testing (SalesReportByPeriod groups by
// orders.updated_at, WorkerPerformanceReport filters order_status_histories
// by created_at — both are empty without this).
const historicalReportDays = 90

// ensureHistoricalOrder inserts a `completed` order dated to a specific
// historical day (pickup_date/created_at/updated_at all backdated), unlike
// ensureOrder which always uses CURRENT_DATE/now(). Idempotent on
// invoice_number like ensureOrder.
func ensureHistoricalOrder(ctx context.Context, pool *pgxpool.Pool, invoiceNumber, customerID, outletID, addressID string, totalPrice float64, day time.Time) (id string, created bool, err error) {
	dayDate := day.Format("2006-01-02")
	createdAt := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, day.Location())
	updatedAt := createdAt.Add(7 * time.Hour)

	err = pool.QueryRow(ctx, `
		INSERT INTO orders (invoice_number, customer_id, outlet_id, pickup_address_id, status, pickup_date, delivery_fee, total_price, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'completed', $5, 0, $6, $7, $8)
		ON CONFLICT (invoice_number) DO NOTHING
		RETURNING id
	`, invoiceNumber, customerID, outletID, addressID, dayDate, totalPrice, createdAt, updatedAt).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, err
	}

	err = pool.QueryRow(ctx, "SELECT id FROM orders WHERE invoice_number = $1", invoiceNumber).Scan(&id)
	return id, false, err
}

// seedHistoricalReportData backfills `days` worth of completed orders (1-4
// per day) for one outlet, each with a full order_items/breakdown, a full
// order_status_histories chain stamped to that historical day, and the
// driver_task/payment rows a completed order implies — so sales and
// employee-performance report endpoints have non-empty, multi-day data to
// chart instead of only "today". driver_tasks/payments are still stamped
// "now" via seedOrderContents (not backdated): neither report query reads
// those tables, only orders.updated_at and order_status_histories.created_at,
// so this is a deliberate scope cut, not an oversight.
func seedHistoricalReportData(ctx context.Context, pool *pgxpool.Pool, invoicePrefix, outletID, customerID, addressID, clothingTypeID, laundryItemID string, actors map[string]string, days int, rng *rand.Rand) error {
	for offset := days; offset >= 1; offset-- {
		day := time.Now().AddDate(0, 0, -offset)
		numOrders := rng.Intn(4) + 1 // random 1-4 orders per day

		for i := 0; i < numOrders; i++ {
			invoice := fmt.Sprintf("%s-%s-%d", invoicePrefix, day.Format("20060102"), i+1)
			// random price between 25000-150000 with realistic variation
			totalPrice := 25000.0 + rng.Float64()*125000

			orderID, created, err := ensureHistoricalOrder(ctx, pool, invoice, customerID, outletID, addressID, totalPrice, day)
			if err != nil {
				return fmt.Errorf("order %s: %w", invoice, err)
			}
			if !created {
				continue
			}

			if err := seedOrderContents(ctx, pool, orderID, clothingTypeID, laundryItemID, actors["outlet_admin"], "completed"); err != nil {
				return fmt.Errorf("contents %s: %w", invoice, err)
			}

			dayStart := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, day.Location())
			if err := seedOrderStatusHistory(ctx, pool, orderID, customerID, actors, "completed", &dayStart); err != nil {
				return fmt.Errorf("history %s: %w", invoice, err)
			}
		}
	}
	return nil
}

// normalizedItemSeed mirrors internal/order.NormalizedItem's JSON shape —
// duplicated here rather than imported since cmd/seed-demo talks to the
// database directly and doesn't depend on the order package.
type normalizedItemSeed struct {
	ItemType string `json:"item_type"`
	ItemID   string `json:"item_id"`
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

// ensureBypassRequest seeds one bypass_requests row with a manufactured
// discrepancy across 5 clothing types, in the given final status. Idempotent:
// skipped if a bypass request already exists for this order+station.
func ensureBypassRequest(ctx context.Context, pool *pgxpool.Pool, orderID, station, requestedBy, clothingTypeID, status, reviewedBy, adminNotes string) error {
	var existing string
	err := pool.QueryRow(ctx, "SELECT id FROM bypass_requests WHERE order_id = $1 AND station = $2", orderID, station).Scan(&existing)
	if err == nil {
		return nil
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("bypass_requests lookup: %w", err)
	}

	// Build expected items: 5 different clothing types
	expectedItems := []normalizedItemSeed{
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Kemeja", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Celana", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Jaket", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Kaos", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Rok", Quantity: 5},
	}
	expectedJSON, err := json.Marshal(expectedItems)
	if err != nil {
		return fmt.Errorf("marshal expected_items: %w", err)
	}

	// Build actual items: reduce first item's quantity by 1 (5 -> 4)
	actualItems := []normalizedItemSeed{
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Kemeja", Quantity: 4},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Celana", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Jaket", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Kaos", Quantity: 5},
		{ItemType: "clothing_type", ItemID: clothingTypeID, Name: "Rok", Quantity: 5},
	}
	actualJSON, err := json.Marshal(actualItems)
	if err != nil {
		return fmt.Errorf("marshal actual_items: %w", err)
	}

	// Vary description based on station
	var description string
	switch station {
	case "washing":
		description = "Selisih kemeja saat pencucian ulang, kemungkinan terselip"
	case "ironing":
		description = "Kemeja hilang saat penyetrikaan, setrikaan lain sempurna"
	case "packing":
		description = "Kurang 1 kemeja dalam verifikasi akhir sebelum packing"
	default:
		description = "Selisih 1 kemeja saat dihitung ulang"
	}

	var reviewedByArg *string
	if reviewedBy != "" {
		reviewedByArg = &reviewedBy
	}
	var adminNotesArg *string
	if adminNotes != "" {
		adminNotesArg = &adminNotes
	}
	var resolvedAt any
	if status != "pending" {
		resolvedAt = time.Now()
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO bypass_requests (
			order_id, station, requested_by, expected_items, actual_items,
			discrepancy_description, attempt_number, status, reviewed_by, admin_notes, resolved_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, $9, $10)
	`, orderID, station, requestedBy, expectedJSON, actualJSON,
		description, status, reviewedByArg, adminNotesArg, resolvedAt); err != nil {
		return fmt.Errorf("bypass_requests: %w", err)
	}
	return nil
}

// ensureEmployeeNotifications seeds a couple of task notifications for the
// given employee (used for the demo driver) so the notifications UI/endpoint
// has something to list.
func ensureEmployeeNotifications(ctx context.Context, pool *pgxpool.Pool, employeeID string) error {
	notifications := []struct{ title, body, notifType string }{
		{"Tugas Pickup Baru", "Ada order baru menunggu dijemput di area Curug.", "task_assigned"},
		{"Tugas Pengiriman Selesai", "Pengiriman untuk pesanan DEMO-0009 berhasil diselesaikan.", "order_update"},
	}
	for _, n := range notifications {
		var existing string
		err := pool.QueryRow(ctx, "SELECT id FROM employee_notifications WHERE employee_id = $1 AND title = $2", employeeID, n.title).Scan(&existing)
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("employee_notifications lookup: %w", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO employee_notifications (employee_id, title, body, type)
			VALUES ($1, $2, $3, $4)
		`, employeeID, n.title, n.body, n.notifType); err != nil {
			return fmt.Errorf("employee_notifications: %w", err)
		}
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

// seedHistoricalAttendances backfills 1.5 years of attendance records (excluding today,
// which is already seeded by ensureAttendanceToday). Distributes status: 70% on_time,
// 20% late, 10% absent. Skips weekends and uses batch insert for efficiency.
func seedHistoricalAttendances(ctx context.Context, pool *pgxpool.Pool, rng *rand.Rand) error {
	// Query all non-admin employees with their outlets and morning shift default
	type empRow struct {
		id           string
		outletID     string
		email        string
		shiftStartHr int // 6 for pagi, 14 for sore (queried from employee_shifts)
	}
	var employees []empRow

	rows, err := pool.Query(ctx, `
		SELECT e.id, e.outlet_id, e.email,
		       COALESCE((SELECT extract(hour FROM ws.start_time)::int
		                 FROM employee_shifts es
		                 JOIN work_shifts ws ON ws.id = es.shift_id
		                 WHERE es.employee_id = e.id LIMIT 1), 6) AS shift_start_hr
		FROM employees e
		WHERE e.outlet_id IS NOT NULL AND e.role != 'super_admin'
		ORDER BY e.email
	`)
	if err != nil {
		return fmt.Errorf("query employees: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var emp empRow
		if err := rows.Scan(&emp.id, &emp.outletID, &emp.email, &emp.shiftStartHr); err != nil {
			return fmt.Errorf("scan employee: %w", err)
		}
		employees = append(employees, emp)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	// Generate attendance for last 550 days (roughly 1.5 years), skip weekends
	const historicalDays = 550
	now := time.Now().In(shift.JakartaLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, shift.JakartaLocation)

	type attendanceRecord struct {
		employeeID   string
		outletID     string
		date         time.Time
		checkInTime  *time.Time
		checkOutTime *time.Time
		isLate       bool
		lateMinutes  int
		status       string
	}
	var records []attendanceRecord

	for offset := historicalDays; offset >= 1; offset-- {
		day := today.AddDate(0, 0, -offset)

		// Skip weekends (0=Sunday, 6=Saturday)
		dayOfWeek := day.Weekday()
		if dayOfWeek == 0 || dayOfWeek == 6 {
			continue
		}

		for _, emp := range employees {
			// Determine status: 70% on_time, 20% late, 10% absent
			statusRand := rng.Float64()
			var status string
			var checkInTime *time.Time
			var checkOutTime *time.Time
			var isLate bool
			var lateMinutes int

			shiftStart := emp.shiftStartHr

			if statusRand < 0.10 { // 10% absent
				status = "absent"
				isLate = false
				lateMinutes = 0
			} else if statusRand < 0.30 { // 20% late
				status = "late"
				isLate = true
				lateMinutes = rng.Intn(25) + 5 // 5-29 minutes late

				// Generate late check-in time (after shift start + late minutes)
				baseTime := time.Date(day.Year(), day.Month(), day.Day(), shiftStart, 0, 0, 0, day.Location())
				checkInTimeTmp := baseTime.Add(time.Duration(lateMinutes+rng.Intn(10)) * time.Minute)
				checkInTime = &checkInTimeTmp

				// Generate check-out time (8 hours after check-in)
				checkOutTimeTmp := checkInTimeTmp.Add(8 * time.Hour)
				checkOutTime = &checkOutTimeTmp
			} else { // 70% on_time
				status = "on_time"
				isLate = false
				lateMinutes = 0

				// Generate on-time check-in (near shift start, -30 to +10 minutes)
				baseTime := time.Date(day.Year(), day.Month(), day.Day(), shiftStart, 0, 0, 0, day.Location())
				offsetMinutes := rng.Intn(40) - 30 // -30 to +10 minutes
				checkInTimeTmp := baseTime.Add(time.Duration(offsetMinutes) * time.Minute)
				checkInTime = &checkInTimeTmp

				// Generate check-out time (8 hours after check-in)
				checkOutTimeTmp := checkInTimeTmp.Add(8 * time.Hour)
				checkOutTime = &checkOutTimeTmp
			}

			records = append(records, attendanceRecord{
				employeeID:   emp.id,
				outletID:     emp.outletID,
				date:         day,
				checkInTime:  checkInTime,
				checkOutTime: checkOutTime,
				isLate:       isLate,
				lateMinutes:  lateMinutes,
				status:       status,
			})
		}
	}

	// Batch insert attendance records to avoid hitting Postgres parameter limit (65,535 params)
	if len(records) == 0 {
		return nil
	}

	const batchSize = 500
	for batchStart := 0; batchStart < len(records); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(records) {
			batchEnd = len(records)
		}
		batch := records[batchStart:batchEnd]

		// Build INSERT statement for this batch
		query := `
			INSERT INTO attendances (
				employee_id, outlet_id, date, check_in_time, check_out_time,
				is_late, late_minutes, status, created_at, updated_at
			) VALUES `
		var args []interface{}
		for i, rec := range batch {
			if i > 0 {
				query += ", "
			}
			query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, now(), now())",
				i*8+1, i*8+2, i*8+3, i*8+4, i*8+5, i*8+6, i*8+7, i*8+8)
			args = append(args, rec.employeeID, rec.outletID, rec.date,
				rec.checkInTime, rec.checkOutTime, rec.isLate, rec.lateMinutes, rec.status)
		}

		// Add ON CONFLICT clause to handle re-runs (idempotent)
		query += ` ON CONFLICT (employee_id, date) DO NOTHING`

		_, err = pool.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("batch insert attendances (batch %d-%d): %w", batchStart, batchEnd, err)
		}
	}

	return nil
}
