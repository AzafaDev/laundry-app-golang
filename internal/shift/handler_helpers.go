package shift

import (
	"context"
	"errors"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// JakartaLocation is the single source of truth for WIB-relative time math
// in this codebase — loaded once at package init rather than reimplementing
// manual UTC+7 offset arithmetic at each call site.
var JakartaLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Printf("warning: failed to load Asia/Jakarta timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}
	JakartaLocation = loc
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation.
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

// parseTimeOfDay parses an "HH:MM" string into a pgtype.Time (microseconds
// since midnight).
func parseTimeOfDay(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	micros := (t.Hour()*3600 + t.Minute()*60) * 1_000_000
	return pgtype.Time{Microseconds: int64(micros), Valid: true}, nil
}

// formatTimeOfDay renders a pgtype.Time back to "HH:MM".
func formatTimeOfDay(t pgtype.Time) string {
	totalSeconds := t.Microseconds / 1_000_000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// CivilDateStart returns midnight of t's calendar date in t's own Location.
// time.Time.Truncate(24*time.Hour) is NOT equivalent — Truncate rounds to a
// multiple of 24h since the Unix epoch (UTC-aligned), so applying it to a
// WIB-located time silently rounds to UTC midnight (07:00 WIB) instead of
// WIB midnight, corrupting "today" for any time between 00:00-06:59 WIB.
func CivilDateStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ResolveShiftWindow re-anchors a TIME-only shift's start/end onto
// targetDate in JakartaLocation. When end_time < start_time the shift spans
// midnight, so end is pushed 24h forward. This is the single implementation
// of overnight-shift resolution — every caller (check-in, check-out,
// AssertShiftEligibility, the end-of-day sweep) shares it rather than each
// reimplementing the "add 24h if overnight" rule independently.
func ResolveShiftWindow(ws db.WorkShift, targetDate time.Time) (start, end time.Time) {
	y, m, d := targetDate.In(JakartaLocation).Date()

	startOfDay := time.Date(y, m, d, 0, 0, 0, 0, JakartaLocation)
	start = startOfDay.Add(time.Duration(ws.StartTime.Microseconds) * time.Microsecond)
	end = startOfDay.Add(time.Duration(ws.EndTime.Microseconds) * time.Microsecond)

	if ws.EndTime.Microseconds < ws.StartTime.Microseconds {
		end = end.Add(24 * time.Hour)
	}

	return start, end
}

// ResolveEmployeeShiftForDate finds which shift an employee is scheduled
// for on targetDate: a date-specific override takes priority, falling back
// to the recurring day_of_week assignment. This single function is shared
// by check-in, check-out, AssertShiftEligibility, and the end-of-day sweep
// — the TS source reimplemented this resolution 3 times, and its cron job
// only checked day_of_week, silently skipping date-specific overrides.
func ResolveEmployeeShiftForDate(ctx context.Context, queries *db.Queries, employeeID pgtype.UUID, targetDate time.Time) (db.EmployeeShift, db.WorkShift, bool) {
	dateOnly := pgtype.Date{Time: CivilDateStart(targetDate.In(JakartaLocation)), Valid: true}

	es, err := queries.GetEmployeeShiftByEmployeeAndDate(ctx, db.GetEmployeeShiftByEmployeeAndDateParams{
		EmployeeID: employeeID,
		Date:       dateOnly,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return db.EmployeeShift{}, db.WorkShift{}, false
	}
	if err == nil {
		ws, err := queries.GetWorkShiftByID(ctx, es.ShiftID)
		if err != nil {
			return db.EmployeeShift{}, db.WorkShift{}, false
		}
		return es, ws, true
	}

	dayOfWeek := int16(targetDate.In(JakartaLocation).Weekday())
	es, err = queries.GetEmployeeShiftByEmployeeAndDayOfWeek(ctx, db.GetEmployeeShiftByEmployeeAndDayOfWeekParams{
		EmployeeID: employeeID,
		DayOfWeek:  pgtype.Int2{Int16: dayOfWeek, Valid: true},
	})
	if err != nil {
		return db.EmployeeShift{}, db.WorkShift{}, false
	}

	ws, err := queries.GetWorkShiftByID(ctx, es.ShiftID)
	if err != nil {
		return db.EmployeeShift{}, db.WorkShift{}, false
	}

	return es, ws, true
}

func toWorkShiftResponse(ws db.WorkShift) WorkShiftResponse {
	return WorkShiftResponse{
		ID:          ws.ID.String(),
		Name:        ws.Name,
		StartTime:   formatTimeOfDay(ws.StartTime),
		EndTime:     formatTimeOfDay(ws.EndTime),
		Description: ws.Description.String,
		IsActive:    ws.IsActive,
	}
}

func toEmployeeShiftResponse(es db.EmployeeShift) EmployeeShiftResponse {
	resp := EmployeeShiftResponse{
		ID:         es.ID.String(),
		EmployeeID: es.EmployeeID.String(),
		ShiftID:    es.ShiftID.String(),
		OutletID:   es.OutletID.String(),
		IsActive:   es.IsActive,
	}
	if es.DayOfWeek.Valid {
		resp.DayOfWeek = &es.DayOfWeek.Int16
	}
	if es.Date.Valid {
		resp.Date = es.Date.Time.Format("2006-01-02")
	}
	return resp
}
