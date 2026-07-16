package cron

import (
	"context"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Start launches the three cron goroutines. In-process time.Ticker/Timer,
// no distributed lock — replicates the TS source's single-instance
// node-cron assumption as-is. ctx is not cancelled anywhere today (no
// graceful shutdown wired up, matching node-cron's own behavior), but is
// threaded through so context.WithCancel can be added later without
// changing this signature.
func Start(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) {
	go runHourly(ctx, "auto-complete-orders", func(ctx context.Context) error {
		_, err := RunAutoCompleteOrders(ctx, pool, queries)
		return err
	})
	go runDailyAt(ctx, "cleanup-tokens", 2, 0, func(ctx context.Context) error {
		return RunCleanupTokens(ctx, queries)
	})
	go runDailyAt(ctx, "absentee-sweep", 23, 55, func(ctx context.Context) error {
		_, err := RunAbsenteeSweep(ctx, queries)
		return err
	})
}

func runHourly(ctx context.Context, name string, job func(context.Context) error) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runJob(ctx, name, job)
		case <-ctx.Done():
			return
		}
	}
}

// runDailyAt fires once at the next occurrence of hour:minute in
// shift.JakartaLocation, then every 24h after that — wall-clock-aligned so
// "daily at 02:00 WIB" doesn't silently drift into "24h after whenever the
// server happened to start."
func runDailyAt(ctx context.Context, name string, hour, minute int, job func(context.Context) error) {
	timer := time.NewTimer(durationUntilNext(hour, minute))
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			runJob(ctx, name, job)
			timer.Reset(24 * time.Hour)
		case <-ctx.Done():
			return
		}
	}
}

func durationUntilNext(hour, minute int) time.Duration {
	now := time.Now().In(shift.JakartaLocation)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, shift.JakartaLocation)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}

// runJob recovers from panics so one bad tick never kills the goroutine —
// mirrors TS's try/catch-per-callback (node-cron keeps scheduling
// regardless of a prior failure).
func runJob(ctx context.Context, name string, job func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("cron: %s panicked: %v", name, r)
		}
	}()
	if err := job(ctx); err != nil {
		log.Printf("cron: %s failed: %v", name, err)
	}
}
