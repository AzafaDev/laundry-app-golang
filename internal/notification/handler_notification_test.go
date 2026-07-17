package notification_test

import (
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestCustomerNotifications(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)

	var notificationIDs []string
	for i := 0; i < 3; i++ {
		if err := notification.NotifyCustomer(t.Context(), app.Queries, customer.ID,
			fmt.Sprintf("Title %d", i), fmt.Sprintf("Body %d", i), notification.TypeOrderUpdate, pgtype.UUID{}); err != nil {
			t.Fatalf("failed to create test notification: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(t.Context(), "DELETE FROM customer_notifications WHERE customer_id = $1", customer.ID)
	})

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	doRequest := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if method != http.MethodGet {
			req.Header.Set("X-CSRF-Token", csrfToken)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		return w
	}

	t.Run("list returns all 3", func(t *testing.T) {
		w := doRequest(http.MethodGet, "/api/v1/customer/notifications")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}
		var resp notification.NotificationListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 3 {
			t.Fatalf("expected 3 notifications, got %d", len(resp.Data))
		}
		for _, n := range resp.Data {
			notificationIDs = append(notificationIDs, n.ID)
		}
	})

	t.Run("unread count is 3 before marking", func(t *testing.T) {
		w := doRequest(http.MethodGet, "/api/v1/customer/notifications/unread-count")
		var resp notification.UnreadCountResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.UnreadCount != 3 {
			t.Errorf("unread_count = %d, want 3", resp.UnreadCount)
		}
	})

	t.Run("mark one read decrements unread count by exactly 1", func(t *testing.T) {
		w := doRequest(http.MethodPatch, "/api/v1/customer/notifications/"+notificationIDs[0]+"/read")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var isRead bool
		row := app.Pool.QueryRow(t.Context(), "SELECT is_read FROM customer_notifications WHERE id = $1", notificationIDs[0])
		if err := row.Scan(&isRead); err != nil {
			t.Fatalf("failed to read back is_read: %v", err)
		}
		if !isRead {
			t.Error("expected is_read = true after marking read")
		}

		countW := doRequest(http.MethodGet, "/api/v1/customer/notifications/unread-count")
		var countResp notification.UnreadCountResponse
		_ = json.Unmarshal(countW.Body.Bytes(), &countResp)
		if countResp.UnreadCount != 2 {
			t.Errorf("unread_count after marking 1 = %d, want 2 (not all 3)", countResp.UnreadCount)
		}
	})

	t.Run("mark all read zeroes unread count", func(t *testing.T) {
		w := doRequest(http.MethodPatch, "/api/v1/customer/notifications/read-all")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		countW := doRequest(http.MethodGet, "/api/v1/customer/notifications/unread-count")
		var countResp notification.UnreadCountResponse
		_ = json.Unmarshal(countW.Body.Bytes(), &countResp)
		if countResp.UnreadCount != 0 {
			t.Errorf("unread_count after mark-all-read = %d, want 0", countResp.UnreadCount)
		}
	})
}

func TestEmployeeNotifications(t *testing.T) {
	app := testutil.NewTestApp(t)
	employee := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})

	var notificationIDs []string
	for i := 0; i < 3; i++ {
		if err := notification.NotifyEmployee(t.Context(), app.Queries, employee.ID,
			fmt.Sprintf("Title %d", i), fmt.Sprintf("Body %d", i), notification.TypeOrderUpdate, pgtype.UUID{}); err != nil {
			t.Fatalf("failed to create test notification: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(t.Context(), "DELETE FROM employee_notifications WHERE employee_id = $1", employee.ID)
	})

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", employee.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	doRequest := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if method != http.MethodGet {
			req.Header.Set("X-CSRF-Token", csrfToken)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		return w
	}

	t.Run("list returns all 3", func(t *testing.T) {
		w := doRequest(http.MethodGet, "/api/v1/employee/notifications")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}
		var resp notification.NotificationListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 3 {
			t.Fatalf("expected 3 notifications, got %d", len(resp.Data))
		}
		for _, n := range resp.Data {
			notificationIDs = append(notificationIDs, n.ID)
		}
	})

	t.Run("mark one read decrements unread count by exactly 1", func(t *testing.T) {
		w := doRequest(http.MethodPatch, "/api/v1/employee/notifications/"+notificationIDs[0]+"/read")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		countW := doRequest(http.MethodGet, "/api/v1/employee/notifications/unread-count")
		var countResp notification.UnreadCountResponse
		_ = json.Unmarshal(countW.Body.Bytes(), &countResp)
		if countResp.UnreadCount != 2 {
			t.Errorf("unread_count after marking 1 = %d, want 2", countResp.UnreadCount)
		}
	})

	t.Run("mark all read zeroes unread count", func(t *testing.T) {
		w := doRequest(http.MethodPatch, "/api/v1/employee/notifications/read-all")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		countW := doRequest(http.MethodGet, "/api/v1/employee/notifications/unread-count")
		var countResp notification.UnreadCountResponse
		_ = json.Unmarshal(countW.Body.Bytes(), &countResp)
		if countResp.UnreadCount != 0 {
			t.Errorf("unread_count after mark-all-read = %d, want 0", countResp.UnreadCount)
		}
	})
}
