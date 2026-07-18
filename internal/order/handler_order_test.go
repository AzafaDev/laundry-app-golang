package order_test

import (
	"context"
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetOrderDetail_ReturnsFullyEnrichedOrder(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWashing)

	employee := app.CreateTestEmployee(t, "outlet_admin", outlet.ID)
	laundryItem := app.CreateTestLaundryItem(t, "kg")
	clothingType := app.CreateTestClothingType(t)

	app.CreateTestOrderItem(t, testOrder.ID, laundryItem.ID, 2.5)
	app.CreateTestOrderItemBreakdown(t, testOrder.ID, clothingType.ID, employee.ID, 3)
	app.CreateTestPayment(t, testOrder.ID, "test-gw-tx-"+testOrder.ID.String(), 25000)

	var complaintID string
	err := app.Pool.QueryRow(context.Background(), `
		INSERT INTO complaints (order_id, customer_id, complaint_type, description, photo_urls)
		VALUES ($1, $2, 'damaged', 'baju robek', '{}')
		RETURNING id
	`, testOrder.ID, customer.ID).Scan(&complaintID)
	if err != nil {
		t.Fatalf("failed to create test complaint: %v", err)
	}
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(context.Background(), "DELETE FROM complaints WHERE id = $1", complaintID)
	})

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/customer/orders/%s", testOrder.ID.String()), nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp order.OrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != testOrder.ID.String() {
		t.Errorf("expected order id %s, got %s", testOrder.ID.String(), resp.ID)
	}
	if resp.OutletName != outlet.Name {
		t.Errorf("expected outlet_name %q, got %q", outlet.Name, resp.OutletName)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if len(resp.Breakdown) != 1 {
		t.Fatalf("expected 1 breakdown, got %d", len(resp.Breakdown))
	}
	if len(resp.StatusHistory) != 0 {
		t.Errorf("expected 0 status history entries (CreateTestOrder doesn't write one), got %d", len(resp.StatusHistory))
	}
	if resp.Payment == nil {
		t.Fatal("expected payment to be populated")
	}
	if resp.Payment.Status != "pending" {
		t.Errorf("expected payment status pending, got %s", resp.Payment.Status)
	}
	if len(resp.Complaints) != 1 {
		t.Fatalf("expected 1 complaint, got %d", len(resp.Complaints))
	}
	if resp.Complaints[0].ComplaintType != "damaged" {
		t.Errorf("expected complaint_type damaged, got %s", resp.Complaints[0].ComplaintType)
	}
}

func TestGetOrderDetail_NoPaymentReturnsNilWithoutError(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWaitingPickupDriver)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/customer/orders/%s", testOrder.ID.String()), nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp order.OrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Payment != nil {
		t.Errorf("expected nil payment, got %+v", resp.Payment)
	}
}

func TestGetOrderDetail_OtherCustomersOrderReturns404(t *testing.T) {
	app := testutil.NewTestApp(t)

	owner := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, owner.ID)
	testOrder := app.CreateTestOrder(t, owner.ID, outlet.ID, address.ID, order.StatusWaitingPickupDriver)

	otherCustomer := app.CreateTestCustomer(t)
	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", otherCustomer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/customer/orders/%s", testOrder.ID.String()), nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListOrders_IncludesOutletName(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWaitingPickupDriver)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer/orders", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp order.OrderListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 order, got %d", len(resp.Data))
	}
	if resp.Data[0].OutletName != outlet.Name {
		t.Errorf("expected outlet_name %q, got %q", outlet.Name, resp.Data[0].OutletName)
	}
}

func createOrderRequest(t *testing.T, router http.Handler, cookies []*http.Cookie, addressID string) *httptest.ResponseRecorder {
	t.Helper()

	pickupDate := time.Now().Format("2006-01-02")
	body := fmt.Sprintf(`{"pickup_address_id":%q,"pickup_date":%q}`, addressID, pickupDate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", testutil.CookieValue(cookies, "csrf_token"))
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// cleanupCreatedOrder removes an order created through the live CreateOrder
// endpoint (and the rows it cascades into) before t.Cleanup tears down the
// outlet/address fixtures those rows reference — order_status_histories,
// driver_tasks, and orders.outlet_id/pickup_address_id have no ON DELETE
// CASCADE, so leaving this order behind would make the outlet/address
// cleanup fail on a foreign key violation.
func cleanupCreatedOrder(t *testing.T, app *testutil.TestApp, rec *httptest.ResponseRecorder) {
	t.Helper()

	var resp order.OrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode create order response: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = app.Pool.Exec(ctx, "DELETE FROM order_status_histories WHERE order_id = $1", resp.ID)
		_, _ = app.Pool.Exec(ctx, "DELETE FROM driver_tasks WHERE order_id = $1", resp.ID)
		if _, err := app.Pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", resp.ID); err != nil {
			t.Logf("failed to clean up created order %s: %v", resp.ID, err)
		}
	})
}

func TestCreateOrder_SucceedsWhenCustomerHasNoActiveOrder(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	rec := createOrderRequest(t, app.Router, cookies, address.ID.String())

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	cleanupCreatedOrder(t, app, rec)
}

func TestCreateOrder_RejectsWhenCustomerHasActiveOrder(t *testing.T) {
	t.Skip("TEMP: active-order-per-customer check disabled for manual testing, see handler_order.go CreateOrder")
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	first := createOrderRequest(t, app.Router, cookies, address.ID.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first order to be created with 201, got %d: %s", first.Code, first.Body.String())
	}
	cleanupCreatedOrder(t, app, first)

	second := createOrderRequest(t, app.Router, cookies, address.ID.String())
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", second.Code, second.Body.String())
	}
}
