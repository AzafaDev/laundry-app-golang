package testutil

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func numeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%v", v)); err != nil {
		panic(err)
	}
	return n
}

// testOutletLat/Lon are shared by every fixture outlet/address so
// nearestOutlet's haversine-distance check always resolves to ~0km, well
// within any reasonable ServiceRadiusKm.
const testOutletLat = -6.200000
const testOutletLon = 106.816666

// CreateTestOutlet inserts an active outlet at the shared test coordinates
// with a generous service radius.
func (a *TestApp) CreateTestOutlet(t *testing.T) db.Outlet {
	t.Helper()

	outlet, err := a.Queries.CreateOutlet(context.Background(), db.CreateOutletParams{
		Name:            fmt.Sprintf("Test Outlet %d", time.Now().UnixNano()),
		Address:         "Test Address",
		Latitude:        numeric(testOutletLat),
		Longitude:       numeric(testOutletLon),
		IsActive:        true,
		ServiceRadiusKm: numeric(50),
	})
	if err != nil {
		t.Fatalf("failed to create test outlet: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM outlets WHERE id = $1", outlet.ID); err != nil {
			t.Logf("failed to clean up test outlet %s: %v", outlet.ID, err)
		}
	})

	return outlet
}

// testWilayahIDs returns one existing (province_id, city_id, district_id)
// triple from the pre-seeded wilayah reference tables (real Indonesian
// administrative data loaded once via migration) — any valid row works,
// nothing about the specific location matters to these tests.
func (a *TestApp) testWilayahIDs(t *testing.T) (provinceID, cityID, districtID int32) {
	t.Helper()

	row := a.Pool.QueryRow(context.Background(), `
		SELECT p.id, c.id, d.id
		FROM districts d
		JOIN cities c ON c.id = d.city_id
		JOIN provinces p ON p.id = c.province_id
		LIMIT 1
	`)
	if err := row.Scan(&provinceID, &cityID, &districtID); err != nil {
		t.Fatalf("failed to look up wilayah reference IDs: %v", err)
	}
	return provinceID, cityID, districtID
}

// CreateTestAddress inserts a primary address for customerID at the shared
// test coordinates, so it always resolves to CreateTestOutlet as the
// nearest outlet.
func (a *TestApp) CreateTestAddress(t *testing.T, customerID pgtype.UUID) db.CreateAddressRow {
	t.Helper()

	provinceID, cityID, districtID := a.testWilayahIDs(t)

	address, err := a.Queries.CreateAddress(context.Background(), db.CreateAddressParams{
		CustomerID: customerID,
		Label:      "Test Address",
		Address:    "Test Street 1",
		ProvinceID: provinceID,
		CityID:     cityID,
		DistrictID: districtID,
		PostalCode: pgtype.Text{String: "12345", Valid: true},
		Latitude:   numeric(testOutletLat),
		Longitude:  numeric(testOutletLon),
		IsPrimary:  true,
	})
	if err != nil {
		t.Fatalf("failed to create test address: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM customer_addresses WHERE id = $1", address.ID); err != nil {
			t.Logf("failed to clean up test address %s: %v", address.ID, err)
		}
	})

	return address
}

// CreateTestClothingType inserts an active clothing type.
func (a *TestApp) CreateTestClothingType(t *testing.T) db.ClothingType {
	t.Helper()

	ct, err := a.Queries.CreateClothingType(context.Background(), db.CreateClothingTypeParams{
		Name:     fmt.Sprintf("Test Clothing Type %d", time.Now().UnixNano()),
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("failed to create test clothing type: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM clothing_types WHERE id = $1", ct.ID); err != nil {
			t.Logf("failed to clean up test clothing type %s: %v", ct.ID, err)
		}
	})

	return ct
}

// CreateTestLaundryItem inserts an active laundry item. unit must be "pcs"
// or "kg" — the laundry_items.unit CHECK constraint rejects anything else
// (the same field behind the #16b compareItems unit bug).
func (a *TestApp) CreateTestLaundryItem(t *testing.T, unit string) db.LaundryItem {
	t.Helper()

	li, err := a.Queries.CreateLaundryItem(context.Background(), db.CreateLaundryItemParams{
		Name:        fmt.Sprintf("Test Laundry Item %d", time.Now().UnixNano()),
		Description: pgtype.Text{Valid: false},
		Unit:        unit,
		BasePrice:   numeric(5000),
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("failed to create test laundry item: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM laundry_items WHERE id = $1", li.ID); err != nil {
			t.Logf("failed to clean up test laundry item %s: %v", li.ID, err)
		}
	})

	return li
}

// CreateTestOrder inserts an order row directly (bypassing the HTTP
// CreateOrder pipeline, which derives outlet_id/fees/etc. itself), so tests
// can start from an arbitrary order status.
func (a *TestApp) CreateTestOrder(t *testing.T, customerID, outletID, addressID pgtype.UUID, status string) db.Order {
	t.Helper()

	order, err := a.Queries.CreateOrder(context.Background(), db.CreateOrderParams{
		InvoiceNumber:   fmt.Sprintf("INV-TEST-%d", time.Now().UnixNano()),
		CustomerID:      customerID,
		OutletID:        outletID,
		PickupAddressID: addressID,
		Status:          status,
		PickupDate:      pgtype.Date{Time: time.Now(), Valid: true},
		DeliveryFee:     numeric(0),
		TotalPrice:      numeric(0),
	})
	if err != nil {
		t.Fatalf("failed to create test order: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = a.Pool.Exec(ctx, "DELETE FROM order_status_histories WHERE order_id = $1", order.ID)
		_, _ = a.Pool.Exec(ctx, "DELETE FROM order_item_breakdowns WHERE order_id = $1", order.ID)
		_, _ = a.Pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = $1", order.ID)
		_, _ = a.Pool.Exec(ctx, "DELETE FROM driver_tasks WHERE order_id = $1", order.ID)
		_, _ = a.Pool.Exec(ctx, "DELETE FROM payments WHERE order_id = $1", order.ID)
		if _, err := a.Pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", order.ID); err != nil {
			t.Logf("failed to clean up test order %s: %v", order.ID, err)
		}
	})

	return order
}

// CreateTestDriverTask inserts a driver_tasks row (status always starts
// "available", per CreateDriverTask's SQL). taskType is "pickup" or
// "delivery"; a given order can have at most one of each (unique index).
func (a *TestApp) CreateTestDriverTask(t *testing.T, orderID pgtype.UUID, taskType string) db.DriverTask {
	t.Helper()

	task, err := a.Queries.CreateDriverTask(context.Background(), db.CreateDriverTaskParams{
		OrderID:  orderID,
		TaskType: taskType,
	})
	if err != nil {
		t.Fatalf("failed to create test driver task: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM driver_tasks WHERE id = $1", task.ID); err != nil {
			t.Logf("failed to clean up test driver task %s: %v", task.ID, err)
		}
	})

	return task
}

// CreateTestOrderItemBreakdown inserts an order_item_breakdowns row.
// createdBy must be an employee ID (NOT NULL FK).
func (a *TestApp) CreateTestOrderItemBreakdown(t *testing.T, orderID, clothingTypeID, createdBy pgtype.UUID, quantity int32) db.OrderItemBreakdown {
	t.Helper()

	b, err := a.Queries.CreateOrderItemBreakdown(context.Background(), db.CreateOrderItemBreakdownParams{
		OrderID:        orderID,
		ClothingTypeID: clothingTypeID,
		Quantity:       quantity,
		CreatedBy:      createdBy,
	})
	if err != nil {
		t.Fatalf("failed to create test order item breakdown: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM order_item_breakdowns WHERE id = $1", b.ID); err != nil {
			t.Logf("failed to clean up test order item breakdown %s: %v", b.ID, err)
		}
	})

	return b
}

// CreateTestOrderItem inserts an order_items row.
func (a *TestApp) CreateTestOrderItem(t *testing.T, orderID, laundryItemID pgtype.UUID, quantity float64) db.OrderItem {
	t.Helper()

	item, err := a.Queries.CreateOrderItem(context.Background(), db.CreateOrderItemParams{
		OrderID:       orderID,
		LaundryItemID: laundryItemID,
		Quantity:      numeric(quantity),
		PriceAtOrder:  numeric(5000),
	})
	if err != nil {
		t.Fatalf("failed to create test order item: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM order_items WHERE id = $1", item.ID); err != nil {
			t.Logf("failed to clean up test order item %s: %v", item.ID, err)
		}
	})

	return item
}

// CreateTestPayment inserts a pending payment for orderID with
// gatewayTransactionID as its Midtrans "order_id" (the field the webhook
// payload's order_id must match to be looked up) and the given amount.
func (a *TestApp) CreateTestPayment(t *testing.T, orderID pgtype.UUID, gatewayTransactionID string, amount float64) db.Payment {
	t.Helper()

	payment, err := a.Queries.UpsertPaymentForOrder(context.Background(), db.UpsertPaymentForOrderParams{
		OrderID:              orderID,
		Amount:               numeric(amount),
		GatewayName:          pgtype.Text{String: "midtrans", Valid: true},
		GatewayTransactionID: pgtype.Text{String: gatewayTransactionID, Valid: true},
		GatewayResponse:      []byte("{}"),
		PaymentLink:          pgtype.Text{String: "https://example.com/pay", Valid: true},
		ExpiredAt:            pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test payment: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM payments WHERE id = $1", payment.ID); err != nil {
			t.Logf("failed to clean up test payment %s: %v", payment.ID, err)
		}
	})

	return payment
}

// MidtransSignature replicates verifySignature's algorithm
// (internal/payment/handler_helpers.go) so tests can build validly-signed
// webhook payloads: hex(sha512(orderID + statusCode + grossAmount + serverKey)).
func (a *TestApp) MidtransSignature(orderID, statusCode, grossAmount string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + a.Cfg.MidtransServerKey))
	return hex.EncodeToString(sum[:])
}

// CreateTestShiftAssignment gives employeeID a work shift covering the
// entire current civil day (so "now" always falls inside it regardless of
// when the test runs) plus a date-specific EmployeeShift assignment for
// today — the two pieces of shift eligibility that exist independently of
// whether the employee has actually checked in yet (see EnsureShiftEligibility,
// which adds the attendance row on top of this for callers that need full
// attendance.AssertShiftEligibility eligibility rather than just a schedule
// to check in against).
func (a *TestApp) CreateTestShiftAssignment(t *testing.T, employeeID, outletID pgtype.UUID) {
	t.Helper()

	now := time.Now().In(shift.JakartaLocation)
	ctx := context.Background()

	ws, err := a.Queries.CreateWorkShift(ctx, db.CreateWorkShiftParams{
		Name:        fmt.Sprintf("Test Shift %d", now.UnixNano()),
		StartTime:   pgtype.Time{Microseconds: 0, Valid: true},
		EndTime:     pgtype.Time{Microseconds: 23*3600*1_000_000 + 59*60*1_000_000 + 59*1_000_000, Valid: true},
		Description: pgtype.Text{Valid: false},
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("failed to create test work shift: %v", err)
	}
	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM work_shifts WHERE id = $1", ws.ID); err != nil {
			t.Logf("failed to clean up test work shift %s: %v", ws.ID, err)
		}
	})

	today := pgtype.Date{Time: shift.CivilDateStart(now), Valid: true}

	es, err := a.Queries.CreateEmployeeShift(ctx, db.CreateEmployeeShiftParams{
		EmployeeID: employeeID,
		ShiftID:    ws.ID,
		OutletID:   outletID,
		DayOfWeek:  pgtype.Int2{Valid: false},
		Date:       today,
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("failed to create test employee shift: %v", err)
	}
	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM employee_shifts WHERE id = $1", es.ID); err != nil {
			t.Logf("failed to clean up test employee shift %s: %v", es.ID, err)
		}
	})
}

// EnsureShiftEligibility gives employeeID everything
// attendance.AssertShiftEligibility requires: CreateTestShiftAssignment's
// work shift + employee shift for today, plus a checked-in (not
// checked-out) Attendance row for today. Every worker/driver-role HTTP
// endpoint (ClaimTask, SubmitItems, CompleteStation, ...) gates on this, so
// any test hitting those routes needs it — a gap not covered by #16a-c,
// whose fixtures never touched these roles.
func (a *TestApp) EnsureShiftEligibility(t *testing.T, employeeID, outletID pgtype.UUID) {
	t.Helper()

	a.CreateTestShiftAssignment(t, employeeID, outletID)

	now := time.Now().In(shift.JakartaLocation)
	ctx := context.Background()
	today := pgtype.Date{Time: shift.CivilDateStart(now), Valid: true}

	att, err := a.Queries.CreateAttendance(ctx, db.CreateAttendanceParams{
		EmployeeID:       employeeID,
		OutletID:         outletID,
		Date:             today,
		CheckInTime:      pgtype.Timestamptz{Time: now, Valid: true},
		CheckInLatitude:  numeric(testOutletLat),
		CheckInLongitude: numeric(testOutletLon),
		IsLate:           pgtype.Bool{Bool: false, Valid: true},
		LateMinutes:      pgtype.Int4{Int32: 0, Valid: true},
		Status:           pgtype.Text{String: "on_time", Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test attendance: %v", err)
	}
	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM attendances WHERE id = $1", att.ID); err != nil {
			t.Logf("failed to clean up test attendance %s: %v", att.ID, err)
		}
	})
}
