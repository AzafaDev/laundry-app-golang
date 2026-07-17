package order_test

import (
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestReviewBypassRequest_ApproveRollsBackIfStationTransitionFails is a
// regression test for a real bug: ReviewBypassRequest used to commit the
// bypass approval and call completeStation as two separate transactions.
// If the order had already moved past the bypass's target station by the
// time the approval landed (e.g. a concurrent normal completion), the
// station transition would fail with 409 station_already_processed, but the
// bypass row was already permanently committed as "approved" with no way to
// retry or reconcile it. This asserts the whole thing now rolls back
// together: a failed station transition must leave the bypass "pending".
func TestReviewBypassRequest_ApproveRollsBackIfStationTransitionFails(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWashing)

	worker := app.CreateTestEmployee(t, "washing_worker", outlet.ID)
	admin := app.CreateTestEmployee(t, "outlet_admin", outlet.ID)

	var bypassID string
	err := app.Pool.QueryRow(t.Context(), `
		INSERT INTO bypass_requests (order_id, station, requested_by, expected_items, actual_items, discrepancy_description)
		VALUES ($1, 'washing', $2, '[]', '[]', 'test discrepancy')
		RETURNING id
	`, testOrder.ID, worker.ID).Scan(&bypassID)
	if err != nil {
		t.Fatalf("failed to create test bypass request: %v", err)
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(t.Context(), "DELETE FROM bypass_requests WHERE id = $1", bypassID)
	})

	// Simulate the race: the order moves past "washing" (e.g. via a
	// concurrent normal completion) before the bypass gets approved, so
	// completeStationTx's WHERE status = 'washing' guard will match 0 rows.
	if _, err := app.Pool.Exec(t.Context(), "UPDATE orders SET status = $1 WHERE id = $2", order.StatusIroning, testOrder.ID); err != nil {
		t.Fatalf("failed to advance order status: %v", err)
	}

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", admin.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	body := `{"approve":true,"admin_notes":"looks fine"}`
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/employee/admin/bypass-requests/%s/review", bypassID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (station_already_processed), body = %s", w.Code, http.StatusConflict, w.Body.String())
	}

	var status string
	var reviewedBy pgtype.UUID
	row := app.Pool.QueryRow(t.Context(), "SELECT status, reviewed_by FROM bypass_requests WHERE id = $1", bypassID)
	if err := row.Scan(&status, &reviewedBy); err != nil {
		t.Fatalf("failed to read back bypass request: %v", err)
	}
	if status != "pending" {
		t.Errorf("bypass status = %q, want %q (must roll back with the failed station transition, not be left permanently approved)", status, "pending")
	}
	if reviewedBy.Valid {
		t.Errorf("reviewed_by = %v, want unset (approval must not have committed)", reviewedBy)
	}
}
