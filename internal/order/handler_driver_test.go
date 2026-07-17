package order_test

import (
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestClaimTask_ConcurrentClaimsOnlyOneSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWaitingPickupDriver)
	task := app.CreateTestDriverTask(t, testOrder.ID, "pickup")

	const numDrivers = 10
	type driver struct {
		cookies   []*http.Cookie
		csrfToken string
	}
	drivers := make([]driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		emp := app.CreateTestEmployee(t, "driver", pgtype.UUID{})
		app.EnsureShiftEligibility(t, emp.ID, outlet.ID)
		cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", emp.Email, testutil.TestPassword)
		drivers[i] = driver{
			cookies:   cookies,
			csrfToken: testutil.CookieValue(cookies, "csrf_token"),
		}
	}

	var wg sync.WaitGroup
	statuses := make([]int, numDrivers)
	for i := 0; i < numDrivers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/employee/driver/tasks/%s/claim", task.ID.String()), nil)
			for _, c := range drivers[i].cookies {
				req.AddCookie(c)
			}
			req.Header.Set("X-CSRF-Token", drivers[i].csrfToken)

			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)
			statuses[i] = w.Code
		}(i)
	}
	wg.Wait()

	successCount, conflictCount := 0, 0
	for _, status := range statuses {
		switch status {
		case http.StatusOK:
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("unexpected status %d (want 200 or 409)", status)
		}
	}

	if successCount != 1 {
		t.Errorf("successCount = %d, want exactly 1", successCount)
	}
	if conflictCount != numDrivers-1 {
		t.Errorf("conflictCount = %d, want %d", conflictCount, numDrivers-1)
	}

	// Confirm the DB row itself ended up with exactly one driver assigned,
	// not corrupted by the race.
	var driverID pgtype.UUID
	var status string
	row := app.Pool.QueryRow(t.Context(), "SELECT driver_id, status FROM driver_tasks WHERE id = $1", task.ID)
	if err := row.Scan(&driverID, &status); err != nil {
		t.Fatalf("failed to read back driver task: %v", err)
	}
	if !driverID.Valid {
		t.Error("expected driver_id to be set after the race")
	}
	if status != "in_progress" {
		t.Errorf("task status = %q, want %q", status, "in_progress")
	}
}
