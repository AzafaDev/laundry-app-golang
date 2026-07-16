package order

import (
	"context"
	"errors"
	"fmt"
	"laundry-app-with-golang/internal/attendance"
	db "laundry-app-with-golang/internal/db/generated"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// EligibilityError is order package's own eligibility failure — distinct
// from *attendance.EligibilityError because "driver already holds an
// active task" is a driver_tasks-state reason, not a shift-eligibility
// reason, even though both map to a 403 the same way.
type EligibilityError struct {
	Status int
	Code   string
}

func (e *EligibilityError) Error() string {
	return fmt.Sprintf("driver eligibility check failed: %s", e.Code)
}

// assertDriverEligibility layers a driver_tasks check on top of
// attendance.AssertShiftEligibility: a driver may not hold more than one
// in_progress task at a time.
func assertDriverEligibility(ctx context.Context, queries *db.Queries, employeeID pgtype.UUID) (pgtype.UUID, error) {
	outletID, err := attendance.AssertShiftEligibility(ctx, queries, employeeID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	if _, err := queries.GetActiveDriverTaskByDriver(ctx, employeeID); err == nil {
		return pgtype.UUID{}, &EligibilityError{Status: http.StatusForbidden, Code: "driver_has_active_task"}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, err
	}

	return outletID, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate invoice number, duplicate complaint).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// currentCustomerID reads the "customer_id" set by the auth middleware and
// converts it into a pgtype.UUID.
func currentCustomerID(c *gin.Context) (pgtype.UUID, error) {
	var customerUUID pgtype.UUID

	customerIDVal, ok := c.Get("customer_id")
	if !ok {
		return customerUUID, errors.New("something went wrong")
	}

	customerIDStr, ok := customerIDVal.(string)
	if !ok {
		return customerUUID, errors.New("something went wrong")
	}

	if err := customerUUID.Scan(customerIDStr); err != nil {
		return customerUUID, err
	}

	return customerUUID, nil
}

// currentEmployeeID reads the "employee_id" set by EmployeeAuthMiddleware
// and converts it into a pgtype.UUID.
func currentEmployeeID(c *gin.Context) (pgtype.UUID, error) {
	var employeeUUID pgtype.UUID

	employeeIDVal, ok := c.Get("employee_id")
	if !ok {
		return employeeUUID, errors.New("something went wrong")
	}

	employeeIDStr, ok := employeeIDVal.(string)
	if !ok {
		return employeeUUID, errors.New("something went wrong")
	}

	if err := employeeUUID.Scan(employeeIDStr); err != nil {
		return employeeUUID, err
	}

	return employeeUUID, nil
}

// currentEmployeeOutletID reads the "outlet_id" set by EmployeeAuthMiddleware.
// ok is false when the caller has no outlet assigned (e.g. super_admin).
func currentEmployeeOutletID(c *gin.Context) (outletID pgtype.UUID, ok bool) {
	val, exists := c.Get("outlet_id")
	if !exists {
		return outletID, false
	}
	outletID, ok = val.(pgtype.UUID)
	return outletID, ok && outletID.Valid
}

// currentEmployeeRole reads the "role" set by EmployeeAuthMiddleware.
func currentEmployeeRole(c *gin.Context) string {
	val, _ := c.Get("role")
	role, _ := val.(string)
	return role
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

// haversineKM returns the great-circle distance in kilometers between two
// lat/long points.
func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0

	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKM * c
}

// nearestOutlet finds the closest active outlet whose service_radius_km
// covers the given pickup coordinates. Returns the outlet and its distance
// in km, or ok=false if no active outlet covers the address at all.
func nearestOutlet(outlets []db.Outlet, pickupLat, pickupLon float64) (outlet db.Outlet, distanceKM float64, ok bool) {
	bestDistance := math.MaxFloat64

	for _, o := range outlets {
		distance := haversineKM(pickupLat, pickupLon, numericToFloat64(o.Latitude), numericToFloat64(o.Longitude))
		radius := numericToFloat64(o.ServiceRadiusKm)

		if distance <= radius && distance < bestDistance {
			bestDistance = distance
			outlet = o
			ok = true
		}
	}

	return outlet, bestDistance, ok
}

// calculateDeliveryFee applies the flat-fee model: free within
// FreeDeliveryRadiusKM, flat FlatDeliveryFee beyond it.
func calculateDeliveryFee(distanceKM float64) float64 {
	if distanceKM <= FreeDeliveryRadiusKM {
		return 0
	}
	return FlatDeliveryFee
}

// generateInvoiceNumber builds an INV-YYYYMMDD-XXXXXX invoice number using
// today's date and a random 6-digit suffix.
func generateInvoiceNumber() string {
	random := rand.Intn(900_000) + 100_000
	return fmt.Sprintf("INV-%s-%d", time.Now().Format("20060102"), random)
}

// createOrderWithUniqueInvoice retries order creation with a freshly
// generated invoice number whenever a unique-constraint collision occurs,
// rather than assuming collisions can't happen.
func createOrderWithUniqueInvoice(ctx context.Context, qtx *db.Queries, params db.CreateOrderParams) (db.Order, error) {
	var lastErr error

	for attempt := 0; attempt < invoiceNumberMaxAttempts; attempt++ {
		params.InvoiceNumber = generateInvoiceNumber()

		created, err := qtx.CreateOrder(ctx, params)
		if err == nil {
			return created, nil
		}
		if !isUniqueViolation(err) {
			return db.Order{}, err
		}
		lastErr = err
	}

	return db.Order{}, lastErr
}
