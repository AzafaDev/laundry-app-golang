package report_test

import (
	"encoding/json"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/report"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func loginAsSuperAdmin(t *testing.T, app *testutil.TestApp) []*http.Cookie {
	t.Helper()
	admin := app.CreateTestEmployee(t, "super_admin", pgtype.UUID{})
	return testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", admin.Email, testutil.TestPassword)
}

func TestGetSalesReport_ReflectsCompletedOrders(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusCompleted)

	const totalPrice = 75000.0
	if _, err := app.Pool.Exec(t.Context(), "UPDATE orders SET total_price = $1 WHERE id = $2", totalPrice, testOrder.ID); err != nil {
		t.Fatalf("failed to set total_price: %v", err)
	}

	cookies := loginAsSuperAdmin(t, app)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/employee/admin/reports/sales", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp report.SalesReportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Summary.TotalOrders < 1 {
		t.Errorf("total_orders = %d, want >= 1", resp.Summary.TotalOrders)
	}
	if resp.Summary.TotalIncome < totalPrice {
		t.Errorf("total_income = %v, want >= %v (fixture order should be included)", resp.Summary.TotalIncome, totalPrice)
	}
}

func TestExportSalesReport_CSVHeaderContentType(t *testing.T) {
	app := testutil.NewTestApp(t)
	cookies := loginAsSuperAdmin(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employee/admin/reports/sales/export", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	want := "text/csv; charset=utf-8"
	if got := w.Header().Get("Content-Type"); got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

func TestGetEmployeePerformanceReport(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusIroning)

	worker := app.CreateTestEmployee(t, "washing_worker", outlet.ID)

	history, err := app.Queries.CreateOrderStatusHistory(t.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       testOrder.ID,
		OldStatus:     pgtype.Text{String: order.StatusWashing, Valid: true},
		NewStatus:     order.StatusIroning,
		ChangedByType: "employee",
		ChangedByID:   worker.ID,
		Note:          pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test order status history: %v", err)
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(t.Context(), "DELETE FROM order_status_histories WHERE id = $1", history.ID)
	})

	cookies := loginAsSuperAdmin(t, app)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/employee/admin/reports/employee-performance", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp report.EmployeePerformanceListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var found *report.EmployeePerformanceResponse
	for i, e := range resp.Data {
		if e.EmployeeID == worker.ID.String() {
			found = &resp.Data[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("worker %s not found in employee performance report: %+v", worker.ID.String(), resp.Data)
	}
	if found.WorkerJobs < 1 {
		t.Errorf("worker_jobs = %d, want >= 1", found.WorkerJobs)
	}
}
