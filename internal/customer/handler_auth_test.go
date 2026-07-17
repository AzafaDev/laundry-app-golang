package customer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	app := testutil.NewTestApp(t)

	t.Run("new email succeeds", func(t *testing.T) {
		email := fmt.Sprintf("test-register-%d@example.com", time.Now().UnixNano())
		body := fmt.Sprintf(`{"full_name":"New Customer","email":%q,"phone":"081234567890","password":"password123","confirm_password":"password123"}`, email)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
		}

		t.Cleanup(func() {
			// Not t.Context(): that context is already canceled by the time
			// Cleanup funcs run, which silently no-ops this DELETE.
			if _, err := app.Pool.Exec(context.Background(), "DELETE FROM customers WHERE email = $1", email); err != nil {
				t.Logf("failed to clean up test customer %s: %v", email, err)
			}
		})
	})

	t.Run("duplicate email fails", func(t *testing.T) {
		customer := app.CreateTestCustomer(t)
		body := fmt.Sprintf(`{"full_name":"Dup Customer","email":%q,"phone":"081234567890","password":"password123","confirm_password":"password123"}`, customer.Email)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want %d, body = %s", w.Code, http.StatusConflict, w.Body.String())
		}
	})
}

func TestLogin(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)

	t.Run("correct password succeeds and sets cookies", func(t *testing.T) {
		cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

		if testutil.CookieValue(cookies, "access_token") == "" {
			t.Error("expected access_token cookie to be set")
		}
		if testutil.CookieValue(cookies, "refresh_token") == "" {
			t.Error("expected refresh_token cookie to be set")
		}
		if testutil.CookieValue(cookies, "csrf_token") == "" {
			t.Error("expected csrf_token cookie to be set")
		}
	})

	t.Run("wrong password fails", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, customer.Email)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

func TestRefresh(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)
	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/refresh", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	newCookies := w.Result().Cookies()
	if testutil.CookieValue(newCookies, "access_token") == "" {
		t.Error("expected a new access_token cookie after refresh")
	}
}

func TestLogout(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)
	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// The refresh token is revoked server-side; a second refresh attempt
	// with the same (now-stale) cookie must fail.
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer/auth/refresh", nil)
	for _, c := range cookies {
		refreshReq.AddCookie(c)
	}
	refreshW := httptest.NewRecorder()
	app.Router.ServeHTTP(refreshW, refreshReq)

	if refreshW.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want %d", refreshW.Code, http.StatusUnauthorized)
	}
}

func TestProfile(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)

	t.Run("without cookie is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/customer/profile", nil)
		w := httptest.NewRecorder()

		app.Router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("with valid cookie succeeds", func(t *testing.T) {
		cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/customer/profile", nil)
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
		if resp["email"] != customer.Email {
			t.Errorf("profile email = %v, want %v", resp["email"], customer.Email)
		}
	})
}
