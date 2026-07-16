package attendance

import (
	"context"
	"errors"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// EligibilityError is returned by AssertShiftEligibility so callers (this
// package's own handlers, and package order in ticket #3) can map it to an
// HTTP status + structured error code without AssertShiftEligibility itself
// depending on gin.Context.
type EligibilityError struct {
	Status int
	Code   string
}

func (e *EligibilityError) Error() string {
	return fmt.Sprintf("shift eligibility check failed: %s", e.Code)
}

const (
	defaultPageLimit            = 50
	maxPageLimit                = 100
	checkinToleranceBeforeShift = 15 * time.Minute
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate check-in for the same employee/day).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

func parsePagination(c *gin.Context) (limit, offset int32) {
	limit = defaultPageLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

func float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

// haversineKM returns the great-circle distance in kilometers between two
// lat/long points. Duplicated from internal/order/handler_helpers.go, which
// keeps it unexported/package-private — tracked as reuse debt (ticket #12).
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

func currentEmployeeRole(c *gin.Context) string {
	val, _ := c.Get("role")
	role, _ := val.(string)
	return role
}

func currentEmployeeOutletID(c *gin.Context) (outletID pgtype.UUID, ok bool) {
	val, exists := c.Get("outlet_id")
	if !exists {
		return outletID, false
	}
	outletID, ok = val.(pgtype.UUID)
	return outletID, ok && outletID.Valid
}

// resolveEmployeeOutlet fetches the outlet assigned to an employee (for
// check-in geofencing), returning its lat/long alongside the employee row
// itself — callers that also need e.g. FullName for a broadcast payload can
// reuse this instead of issuing a second GetEmployeeByID.
func resolveEmployeeOutlet(ctx context.Context, queries *db.Queries, employeeID pgtype.UUID) (db.Outlet, db.Employee, error) {
	employee, err := queries.GetEmployeeByID(ctx, employeeID)
	if err != nil {
		return db.Outlet{}, db.Employee{}, err
	}
	if !employee.OutletID.Valid {
		return db.Outlet{}, db.Employee{}, pgx.ErrNoRows
	}
	outlet, err := queries.GetOutletByID(ctx, employee.OutletID)
	return outlet, employee, err
}

func toAttendanceResponse(a db.Attendance) AttendanceResponse {
	resp := AttendanceResponse{
		ID:          a.ID.String(),
		EmployeeID:  a.EmployeeID.String(),
		OutletID:    a.OutletID.String(),
		Date:        a.Date.Time.Format("2006-01-02"),
		IsLate:      a.IsLate.Bool,
		LateMinutes: a.LateMinutes.Int32,
		Status:      a.Status.String,
	}
	if a.CheckInTime.Valid {
		resp.CheckInTime = a.CheckInTime.Time.Format(time.RFC3339)
	}
	if a.CheckOutTime.Valid {
		resp.CheckOutTime = a.CheckOutTime.Time.Format(time.RFC3339)
	}
	return resp
}

// AssertShiftEligibility is the exported guard other domains (order/worker,
// ticket #3) call before allowing pipeline actions: the employee must have
// an active shift right now, have checked in today, and not have checked
// out yet. Returns the employee's outlet ID on success.
func AssertShiftEligibility(ctx context.Context, queries *db.Queries, employeeID pgtype.UUID) (pgtype.UUID, error) {
	now := time.Now().In(shift.JakartaLocation)

	_, ws, ok := shift.ResolveEmployeeShiftForDate(ctx, queries, employeeID, now)
	if !ok {
		return pgtype.UUID{}, &EligibilityError{Status: http.StatusForbidden, Code: "no_active_shift"}
	}

	start, end := shift.ResolveShiftWindow(ws, now)
	if now.Before(start) || now.After(end) {
		return pgtype.UUID{}, &EligibilityError{Status: http.StatusForbidden, Code: "no_active_shift"}
	}

	todayDate := pgtype.Date{Time: shift.CivilDateStart(now), Valid: true}
	todayAttendance, err := queries.GetAttendanceByEmployeeAndDate(ctx, db.GetAttendanceByEmployeeAndDateParams{
		EmployeeID: employeeID,
		Date:       todayDate,
	})
	if errors.Is(err, pgx.ErrNoRows) || !todayAttendance.CheckInTime.Valid {
		return pgtype.UUID{}, &EligibilityError{Status: http.StatusForbidden, Code: "not_checked_in"}
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if todayAttendance.CheckOutTime.Valid {
		return pgtype.UUID{}, &EligibilityError{Status: http.StatusForbidden, Code: "already_checked_out"}
	}

	return todayAttendance.OutletID, nil
}
