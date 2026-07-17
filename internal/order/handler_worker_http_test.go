package order_test

import (
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubmitItems_MismatchRequiresBypass(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusPacking)

	worker := app.CreateTestEmployee(t, "packing_worker", outlet.ID)
	app.EnsureShiftEligibility(t, worker.ID, outlet.ID)

	clothingType := app.CreateTestClothingType(t)
	app.CreateTestOrderItemBreakdown(t, testOrder.ID, clothingType.ID, worker.ID, 3)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", worker.Email, testutil.TestPassword)
	csrfToken := testutil.CookieValue(cookies, "csrf_token")

	// Submit fewer items than expected (3 expected, 2 actual) — must be
	// reported as a bypass-requiring discrepancy, not silently accepted or
	// a 500.
	body := fmt.Sprintf(`{"actual_items":[{"clothing_type_id":%q,"actual_quantity":2}],"actual_satuan_items":[]}`, clothingType.ID.String())
	url := fmt.Sprintf("/api/v1/employee/worker/station/packing/orders/%s/submit-items", testOrder.ID.String())
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusConflict, w.Body.String())
	}

	var resp order.SubmitItemsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if !resp.RequiresBypass {
		t.Error("expected requires_bypass=true")
	}
	if len(resp.Discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(resp.Discrepancies), resp.Discrepancies)
	}
	d := resp.Discrepancies[0]
	if d.Expected != 3 || d.Actual != 2 {
		t.Errorf("discrepancy = %+v, want expected=3 actual=2", d)
	}
	if d.Name != clothingType.Name {
		t.Errorf("discrepancy.Name = %q, want %q (fillDiscrepancyNames should resolve the human-readable name)", d.Name, clothingType.Name)
	}

	// Confirm the order status did NOT advance past packing — a
	// discrepancy must block the station transition entirely.
	var status string
	row := app.Pool.QueryRow(t.Context(), "SELECT status FROM orders WHERE id = $1", testOrder.ID)
	if err := row.Scan(&status); err != nil {
		t.Fatalf("failed to read back order status: %v", err)
	}
	if status != order.StatusPacking {
		t.Errorf("order status = %q, want %q (unchanged)", status, order.StatusPacking)
	}
}
