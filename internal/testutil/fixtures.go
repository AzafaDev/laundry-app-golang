package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestPassword is the plaintext password used for every fixture created by
// this package.
const TestPassword = "test-password-123"

// CreateTestCustomer inserts a customer row with a unique email and hashed
// TestPassword, and registers cleanup. Customers have no delete query
// generated at all (confirmed via grep of internal/db/queries/customers.sql),
// so cleanup goes through the raw pool instead of db.Queries.
func (a *TestApp) CreateTestCustomer(t *testing.T) db.Customer {
	t.Helper()

	hash, err := auth.HashPassword(TestPassword)
	if err != nil {
		t.Fatalf("failed to hash test password: %v", err)
	}

	email := fmt.Sprintf("test-customer-%d@example.com", time.Now().UnixNano())
	customer, err := a.Queries.CreateCustomer(context.Background(), db.CreateCustomerParams{
		FullName:     "Test Customer",
		Email:        email,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
		Phone:        pgtype.Text{String: "081234567890", Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test customer: %v", err)
	}

	t.Cleanup(func() {
		if _, err := a.Pool.Exec(context.Background(), "DELETE FROM customers WHERE id = $1", customer.ID); err != nil {
			t.Logf("failed to clean up test customer %s: %v", customer.Email, err)
		}
	})

	return customer
}

// CreateTestEmployee inserts an employee row for role with a unique email
// and hashed TestPassword, and registers cleanup via HardDeleteEmployee.
// IsActive is always true — employee Login rejects !IsActive.
func (a *TestApp) CreateTestEmployee(t *testing.T, role string, outletID pgtype.UUID) db.Employee {
	t.Helper()

	hash, err := auth.HashPassword(TestPassword)
	if err != nil {
		t.Fatalf("failed to hash test password: %v", err)
	}

	email := fmt.Sprintf("test-employee-%s-%d@example.com", role, time.Now().UnixNano())
	employee, err := a.Queries.CreateEmployee(context.Background(), db.CreateEmployeeParams{
		FullName:     "Test Employee",
		Email:        email,
		Phone:        pgtype.Text{String: "081234567890", Valid: true},
		PasswordHash: pgtype.Text{String: hash, Valid: true},
		Role:         role,
		IsActive:     true,
		OutletID:     outletID,
	})
	if err != nil {
		t.Fatalf("failed to create test employee: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		// t.Cleanup runs LIFO, so fixtures registered after this one (e.g.
		// a driver_tasks row claimed by this employee) delete *before* this
		// runs — but rows created *before* this employee (e.g. an order's
		// driver_tasks the employee later claims via HTTP) have their
		// cleanup run *after* this one instead, and still FK-reference it.
		// Null out those references first so employee deletion never
		// depends on cleanup ordering across fixtures.
		_, _ = a.Pool.Exec(ctx, "UPDATE driver_tasks SET driver_id = NULL WHERE driver_id = $1", employee.ID)
		// order_item_breakdowns.created_by is NOT NULL, so it can't be
		// nulled out like driver_id above — the referencing rows must go.
		_, _ = a.Pool.Exec(ctx, "DELETE FROM order_item_breakdowns WHERE created_by = $1", employee.ID)
		if err := a.Queries.HardDeleteEmployee(ctx, employee.ID); err != nil {
			t.Logf("failed to clean up test employee %s: %v", employee.Email, err)
		}
	})

	return employee
}

// LoginAs POSTs {email, password} to endpoint and returns the cookies set
// on a successful (200) response.
func LoginAs(t *testing.T, router *gin.Engine, endpoint, email, password string) []*http.Cookie {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		var bodyJSON map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&bodyJSON)
		t.Fatalf("login to %s failed: status %d, body %v", endpoint, resp.StatusCode, bodyJSON)
	}

	return resp.Cookies()
}

// CookieValue returns the value of the named cookie, or "" if absent.
func CookieValue(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}
