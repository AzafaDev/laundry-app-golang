package attendance_test

import (
	"fmt"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckIn_WithinGeofenceSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)

	outlet := app.CreateTestOutlet(t)
	worker := app.CreateTestEmployee(t, "washing_worker", outlet.ID)
	app.CreateTestShiftAssignment(t, worker.ID, outlet.ID)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", worker.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	// testOutletLat/Lon aren't exported, but CreateTestOutlet always uses
	// the same fixed coordinates — matching them exactly guarantees 0
	// distance, safely within any geofence radius.
	body := fmt.Sprintf(`{"latitude":%v,"longitude":%v}`, -6.200000, 106.816666)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/attendance/check-in", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var count int
	row := app.Pool.QueryRow(t.Context(), "SELECT count(*) FROM attendances WHERE employee_id = $1 AND check_in_time IS NOT NULL", worker.ID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to check attendance row: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 attendance row with check_in_time set, got %d", count)
	}

	t.Cleanup(func() {
		_, _ = app.Pool.Exec(t.Context(), "DELETE FROM attendances WHERE employee_id = $1", worker.ID)
	})
}

func TestCheckIn_OutsideGeofenceRejected(t *testing.T) {
	app := testutil.NewTestApp(t)

	outlet := app.CreateTestOutlet(t)
	worker := app.CreateTestEmployee(t, "washing_worker", outlet.ID)
	app.CreateTestShiftAssignment(t, worker.ID, outlet.ID)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", worker.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	// ~0.05 degrees (~5.5km) away — well outside the default 500m radius.
	body := fmt.Sprintf(`{"latitude":%v,"longitude":%v}`, -6.250000, 106.866666)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/attendance/check-in", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "outside_geofence") {
		t.Errorf("body = %s, want error containing %q", w.Body.String(), "outside_geofence")
	}

	var count int
	row := app.Pool.QueryRow(t.Context(), "SELECT count(*) FROM attendances WHERE employee_id = $1", worker.ID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to check attendance row: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no attendance row to be created, got %d", count)
	}
}
