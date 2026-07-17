package employee_test

import (
	"context"
	"encoding/json"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestEmployeeLogin(t *testing.T) {
	app := testutil.NewTestApp(t)
	emp := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})

	t.Run("correct password succeeds and sets cookies", func(t *testing.T) {
		cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", emp.Email, testutil.TestPassword)

		if testutil.CookieValue(cookies, "staff_access_token") == "" {
			t.Error("expected staff_access_token cookie to be set")
		}
		if testutil.CookieValue(cookies, "staff_refresh_token") == "" {
			t.Error("expected staff_refresh_token cookie to be set")
		}
		if testutil.CookieValue(cookies, "csrf_token") == "" {
			t.Error("expected csrf_token cookie to be set")
		}
	})

	t.Run("wrong password fails", func(t *testing.T) {
		body := `{"email":"` + emp.Email + `","password":"wrong-password"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("inactive employee is rejected", func(t *testing.T) {
		inactive := app.CreateTestEmployee(t, "outlet_admin", pgtype.UUID{})
		// No sqlc query toggles is_active on its own — go through the raw
		// pool for this one fixture mutation, same pattern used for
		// customer cleanup (which also has no matching generated query).
		if _, err := app.Pool.Exec(context.Background(), "UPDATE employees SET is_active = false WHERE id = $1", inactive.ID); err != nil {
			t.Fatalf("failed to deactivate test employee: %v", err)
		}

		body := `{"email":"` + inactive.Email + `","password":"` + testutil.TestPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d, body = %s", w.Code, http.StatusUnauthorized, w.Body.String())
		}
	})
}

func TestEmployeeRefresh(t *testing.T) {
	app := testutil.NewTestApp(t)
	emp := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})
	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", emp.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/auth/refresh", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	newCookies := w.Result().Cookies()
	if testutil.CookieValue(newCookies, "staff_access_token") == "" {
		t.Error("expected a new staff_access_token cookie after refresh")
	}
}

func TestEmployeeLogout(t *testing.T) {
	app := testutil.NewTestApp(t)
	emp := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})
	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", emp.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/employee/auth/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/employee/auth/refresh", nil)
	for _, c := range cookies {
		refreshReq.AddCookie(c)
	}
	refreshW := httptest.NewRecorder()
	app.Router.ServeHTTP(refreshW, refreshReq)

	if refreshW.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want %d", refreshW.Code, http.StatusUnauthorized)
	}
}

func TestEmployeeProfile(t *testing.T) {
	app := testutil.NewTestApp(t)
	emp := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})

	t.Run("without cookie is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/employee/profile", nil)
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("with valid cookie succeeds", func(t *testing.T) {
		cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", emp.Email, testutil.TestPassword)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/employee/profile", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["email"] != emp.Email {
			t.Errorf("profile email = %v, want %v", resp["email"], emp.Email)
		}
	})
}
