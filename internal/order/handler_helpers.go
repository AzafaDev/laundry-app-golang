package order

import (
	"context"
	"errors"
	"fmt"
	"laundry-app-with-golang/internal/apphelper"
	"laundry-app-with-golang/internal/attendance"
	db "laundry-app-with-golang/internal/db/generated"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
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
		distance := haversineKM(pickupLat, pickupLon, apphelper.NumericToFloat64(o.Latitude), apphelper.NumericToFloat64(o.Longitude))
		radius := apphelper.NumericToFloat64(o.ServiceRadiusKm)

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
		if !apphelper.IsUniqueViolation(err) {
			return db.Order{}, err
		}
		lastErr = err
	}

	return db.Order{}, lastErr
}
