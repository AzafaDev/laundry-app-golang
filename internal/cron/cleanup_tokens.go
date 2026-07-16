package cron

import (
	"context"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
)

// RunCleanupTokens hard-deletes expired/revoked/used rows across all 6 Go
// token tables — TS only covers 2 (refreshToken, passwordResetToken); the Go
// schema is more granular (separate customer/employee token tables, plus
// email-verification and email-change), so cleanup covers all of them.
// Every table is attempted even if an earlier one fails, mirroring the
// TS source's Promise.all-style "don't let one failure block the rest."
func RunCleanupTokens(ctx context.Context, queries *db.Queries) error {
	jobs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"refresh_tokens", queries.DeleteExpiredOrRevokedRefreshTokens},
		{"employee_refresh_tokens", queries.DeleteExpiredOrRevokedEmployeeRefreshTokens},
		{"email_verification_tokens", queries.DeleteExpiredOrUsedEmailVerificationTokens},
		{"password_reset_tokens", queries.DeleteExpiredOrUsedPasswordResetTokens},
		{"email_change_tokens", queries.DeleteExpiredOrUsedEmailChangeTokens},
		{"employee_password_reset_tokens", queries.DeleteExpiredOrUsedEmployeePasswordResetTokens},
	}

	var firstErr error
	for _, j := range jobs {
		if err := j.fn(ctx); err != nil {
			log.Printf("cron: cleanup %s: %v", j.name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
