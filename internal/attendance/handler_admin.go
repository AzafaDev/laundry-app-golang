package attendance

import (
	"context"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) ListAttendanceReport(c *gin.Context) {
	limit, offset := parsePagination(c)

	filter := reportFilterFromQuery(c)

	logs, err := h.Queries.ListAttendanceReport(c.Request.Context(), db.ListAttendanceReportParams{
		OutletID:   filter.outletID,
		EmployeeID: filter.employeeID,
		Status:     filter.status,
		DateFrom:   filter.dateFrom,
		DateTo:     filter.dateTo,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountAttendanceReport(c.Request.Context(), db.CountAttendanceReportParams{
		OutletID:   filter.outletID,
		EmployeeID: filter.employeeID,
		Status:     filter.status,
		DateFrom:   filter.dateFrom,
		DateTo:     filter.dateTo,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]AttendanceResponse, 0, len(logs))
	for _, a := range logs {
		data = append(data, toAttendanceResponse(a))
	}

	c.JSON(http.StatusOK, AttendanceListResponse{Data: data, TotalCount: totalCount})
}

// exportAttendanceRowCap mirrors the TS source's exportAttendanceReport:
// refuse to export more than this many rows in one go, asking the caller to
// narrow their filter instead of silently truncating or streaming a huge
// file.
const exportAttendanceRowCap = 5000

// ExportAttendanceReport is the CSV counterpart of ListAttendanceReport —
// same query, same outlet-scoping, no BOM prefix (unlike sales/employee-
// performance exports in internal/report, which do prepend one — replicated
// from the TS source's distinction as-is, not standardized).
func (h *Handler) ExportAttendanceReport(c *gin.Context) {
	filter := reportFilterFromQuery(c)

	logs, err := h.Queries.ListAttendanceReport(c.Request.Context(), db.ListAttendanceReportParams{
		OutletID:   filter.outletID,
		EmployeeID: filter.employeeID,
		Status:     filter.status,
		DateFrom:   filter.dateFrom,
		DateTo:     filter.dateTo,
		Limit:      exportAttendanceRowCap,
		Offset:     0,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if len(logs) >= exportAttendanceRowCap {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "data terlalu besar, persempit filter"})
		return
	}

	header := []string{"employee_id", "outlet_id", "date", "check_in_time", "check_out_time", "is_late", "late_minutes", "status"}
	rows := make([][]string, 0, len(logs))
	for _, a := range logs {
		resp := toAttendanceResponse(a)
		rows = append(rows, []string{
			resp.EmployeeID,
			resp.OutletID,
			resp.Date,
			resp.CheckInTime,
			resp.CheckOutTime,
			strconv.FormatBool(resp.IsLate),
			strconv.Itoa(int(resp.LateMinutes)),
			resp.Status,
		})
	}

	filename := fmt.Sprintf("attendance_report_%s.csv", time.Now().Format("2006-01-02"))
	writeCSV(c, filename, header, rows, false)
}

type reportFilter struct {
	outletID   pgtype.UUID
	employeeID pgtype.UUID
	status     pgtype.Text
	dateFrom   pgtype.Date
	dateTo     pgtype.Date
}

// reportFilterFromQuery enforces outlet-scoping: outlet_admin is forced to
// their own outlet (any ?outlet_id= is ignored — they cannot see other
// outlets' attendance by guessing the param); super_admin may optionally
// pass ?outlet_id= to filter, or leave it unscoped. Mirrors the pattern in
// internal/order's complaintListFilter (ticket #7), copied rather than
// imported since the two packages don't share this helper.
func reportFilterFromQuery(c *gin.Context) reportFilter {
	var f reportFilter

	if currentEmployeeRole(c) == "outlet_admin" {
		if outletID, ok := currentEmployeeOutletID(c); ok {
			f.outletID = outletID
		}
	} else if v := c.Query("outlet_id"); v != "" {
		_ = f.outletID.Scan(v)
	}
	if v := c.Query("employee_id"); v != "" {
		_ = f.employeeID.Scan(v)
	}
	if v := c.Query("status"); v != "" {
		f.status = pgtype.Text{String: v, Valid: true}
	}
	if v := c.Query("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.dateFrom = pgtype.Date{Time: t, Valid: true}
		}
	}
	if v := c.Query("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.dateTo = pgtype.Date{Time: t, Valid: true}
		}
	}

	return f
}

// TriggerSweep is a super_admin-only manual trigger for RunEndOfDaySweep,
// so the end-of-day absentee/auto-checkout logic can be exercised now
// without waiting for the scheduler (deferred to ticket #10).
func (h *Handler) TriggerSweep(c *gin.Context) {
	var req SweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetDate, err := time.ParseInLocation("2006-01-02", req.Date, shift.JakartaLocation)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}

	result, err := RunEndOfDaySweep(c.Request.Context(), h.Queries, targetDate)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// RunEndOfDaySweep marks absent any employee scheduled to work targetDate
// (recurring or date-specific — via shift.ListEmployeeShiftsForDate, which
// resolves the same override-priority as ResolveEmployeeShiftForDate) who
// never checked in, and auto-checks-out anyone who checked in but never
// checked out. Exposed as a plain function, not only an HTTP handler, so
// ticket #10's scheduler can call it directly once it exists.
func RunEndOfDaySweep(ctx context.Context, queries *db.Queries, targetDate time.Time) (SweepResponse, error) {
	dayOfWeek := int16(targetDate.In(shift.JakartaLocation).Weekday())
	dateOnly := pgtype.Date{Time: shift.CivilDateStart(targetDate.In(shift.JakartaLocation)), Valid: true}

	scheduled, err := queries.ListEmployeeShiftsForDate(ctx, db.ListEmployeeShiftsForDateParams{
		Date:      dateOnly,
		DayOfWeek: pgtype.Int2{Int16: dayOfWeek, Valid: true},
	})
	if err != nil {
		return SweepResponse{}, err
	}

	result := SweepResponse{Date: targetDate.Format("2006-01-02")}

	for _, es := range scheduled {
		attendanceRow, err := queries.GetAttendanceByEmployeeAndDate(ctx, db.GetAttendanceByEmployeeAndDateParams{
			EmployeeID: es.EmployeeID,
			Date:       dateOnly,
		})

		switch {
		case err != nil:
			// No attendance row at all — never checked in.
			if _, createErr := queries.CreateAbsentAttendance(ctx, db.CreateAbsentAttendanceParams{
				EmployeeID: es.EmployeeID,
				OutletID:   es.OutletID,
				Date:       dateOnly,
			}); createErr == nil {
				result.MarkedAbsent++
			}
		case attendanceRow.CheckInTime.Valid && !attendanceRow.CheckOutTime.Valid:
			ws, wsErr := queries.GetWorkShiftByID(ctx, es.ShiftID)
			if wsErr != nil {
				continue
			}
			_, shiftEnd := shift.ResolveShiftWindow(ws, targetDate)
			if autoCheckoutErr := queries.AutoCheckoutAttendance(ctx, db.AutoCheckoutAttendanceParams{
				CheckOutTime: pgtype.Timestamptz{Time: shiftEnd, Valid: true},
				EmployeeID:   es.EmployeeID,
				Date:         dateOnly,
			}); autoCheckoutErr == nil {
				result.AutoCheckedOut++
			}
		}
	}

	return result, nil
}
