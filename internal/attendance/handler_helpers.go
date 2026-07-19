package attendance

import (
	"context"
	"errors"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
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

// toAttendanceResponseWithOutlet maps an attendances LEFT JOIN outlets row
// (db.ListAttendancesByEmployeeRow) into AttendanceResponse, including the
// joined outlet name and the notes column (present on the table since it
// was created, but never surfaced in a response until now).
func toAttendanceResponseWithOutlet(a db.ListAttendancesByEmployeeRow) AttendanceResponse {
	resp := AttendanceResponse{
		ID:          a.ID.String(),
		EmployeeID:  a.EmployeeID.String(),
		OutletID:    a.OutletID.String(),
		Date:        a.Date.Time.Format("2006-01-02"),
		IsLate:      a.IsLate.Bool,
		LateMinutes: a.LateMinutes.Int32,
		Status:      a.Status.String,
		Notes:       a.Notes.String,
	}
	if a.OutletName.Valid {
		resp.OutletName = a.OutletName.String
	}
	if a.CheckInTime.Valid {
		resp.CheckInTime = a.CheckInTime.Time.Format(time.RFC3339)
	}
	if a.CheckOutTime.Valid {
		resp.CheckOutTime = a.CheckOutTime.Time.Format(time.RFC3339)
	}
	return resp
}

func toAttendanceReportResponse(a db.ListAttendanceReportRow) AttendanceResponse {
	resp := AttendanceResponse{
		ID:          a.ID.String(),
		EmployeeID:  a.EmployeeID.String(),
		OutletID:    a.OutletID.String(),
		Date:        a.Date.Time.Format("2006-01-02"),
		IsLate:      a.IsLate.Bool,
		LateMinutes: a.LateMinutes.Int32,
		Status:      a.Status.String,
		Notes:       a.Notes.String,
	}
	if a.EmployeeName.Valid {
		resp.EmployeeName = a.EmployeeName.String
	}
	if a.OutletName.Valid {
		resp.OutletName = a.OutletName.String
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
