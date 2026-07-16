package cron

import (
	"context"
	"laundry-app-with-golang/internal/attendance"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"time"
)

// RunAbsenteeSweep is a thin wrapper around attendance.RunEndOfDaySweep
// (ticket #5) purely for call-site symmetry with the other two jobs in this
// package — the sweep logic itself is not reimplemented here.
func RunAbsenteeSweep(ctx context.Context, queries *db.Queries) (attendance.SweepResponse, error) {
	return attendance.RunEndOfDaySweep(ctx, queries, time.Now().In(shift.JakartaLocation))
}
