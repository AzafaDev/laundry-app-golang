package cron_test

import (
	"fmt"
	"laundry-app-with-golang/internal/cron"
	"laundry-app-with-golang/internal/testutil"
	"testing"
	"time"
)

func TestRunCleanupTokens(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	ctx := t.Context()

	// refresh_tokens: expires_at < now() OR revoked_at IS NOT NULL
	var expiredRefreshID, validRefreshID string
	if err := app.Pool.QueryRow(ctx,
		`INSERT INTO refresh_tokens (customer_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		customer.ID, fmt.Sprintf("expired-refresh-%d", time.Now().UnixNano()), time.Now().Add(-1*time.Hour),
	).Scan(&expiredRefreshID); err != nil {
		t.Fatalf("failed to insert expired refresh token: %v", err)
	}
	if err := app.Pool.QueryRow(ctx,
		`INSERT INTO refresh_tokens (customer_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		customer.ID, fmt.Sprintf("valid-refresh-%d", time.Now().UnixNano()), time.Now().Add(24*time.Hour),
	).Scan(&validRefreshID); err != nil {
		t.Fatalf("failed to insert valid refresh token: %v", err)
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE id IN ($1, $2)", expiredRefreshID, validRefreshID)
	})

	// password_reset_tokens: expires_at < now() OR used_at IS NOT NULL
	var expiredResetID, validResetID string
	if err := app.Pool.QueryRow(ctx,
		`INSERT INTO password_reset_tokens (customer_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		customer.ID, fmt.Sprintf("expired-reset-%d", time.Now().UnixNano()), time.Now().Add(-1*time.Hour),
	).Scan(&expiredResetID); err != nil {
		t.Fatalf("failed to insert expired password reset token: %v", err)
	}
	if err := app.Pool.QueryRow(ctx,
		`INSERT INTO password_reset_tokens (customer_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		customer.ID, fmt.Sprintf("valid-reset-%d", time.Now().UnixNano()), time.Now().Add(24*time.Hour),
	).Scan(&validResetID); err != nil {
		t.Fatalf("failed to insert valid password reset token: %v", err)
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(ctx, "DELETE FROM password_reset_tokens WHERE id IN ($1, $2)", expiredResetID, validResetID)
	})

	if err := cron.RunCleanupTokens(ctx, app.Queries); err != nil {
		t.Fatalf("RunCleanupTokens returned error: %v", err)
	}

	assertGone := func(table, id string) {
		var count int
		row := app.Pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE id = $1", table), id)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to check %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("expired row in %s was not cleaned up", table)
		}
	}
	assertSurvives := func(table, id string) {
		var count int
		row := app.Pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE id = $1", table), id)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to check %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("valid row in %s was incorrectly deleted (false-positive cleanup)", table)
		}
	}

	assertGone("refresh_tokens", expiredRefreshID)
	assertSurvives("refresh_tokens", validRefreshID)
	assertGone("password_reset_tokens", expiredResetID)
	assertSurvives("password_reset_tokens", validResetID)
}
