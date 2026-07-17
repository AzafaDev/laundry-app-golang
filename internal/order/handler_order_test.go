package order_test

import (
	"context"
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
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
