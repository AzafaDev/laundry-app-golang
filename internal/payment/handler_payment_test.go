package payment_test

import (
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func postWebhook(t *testing.T, app *testutil.TestApp, orderID, statusCode, grossAmount, transactionStatus, fraudStatus string) int {
	t.Helper()

	signature := app.MidtransSignature(orderID, statusCode, grossAmount)
	body := fmt.Sprintf(
		`{"order_id":%q,"status_code":%q,"gross_amount":%q,"signature_key":%q,"transaction_status":%q,"fraud_status":%q}`,
		orderID, statusCode, grossAmount, signature, transactionStatus, fraudStatus,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/notification", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w.Code
}

func orderStatus(t *testing.T, app *testutil.TestApp, orderID any) string {
	t.Helper()
	var status string
	row := app.Pool.QueryRow(t.Context(), "SELECT status FROM orders WHERE id = $1", orderID)
	if err := row.Scan(&status); err != nil {
		t.Fatalf("failed to read back order status: %v", err)
	}
	return status
}

func TestHandleWebhook_DuplicateNotificationIsIdempotent(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusWaitingPayment)

	gatewayTxID := fmt.Sprintf("test-tx-%d", time.Now().UnixNano())
	app.CreateTestPayment(t, testOrder.ID, gatewayTxID, 50000)

	code := postWebhook(t, app, gatewayTxID, "200", "50000.00", "settlement", "")
	if code != http.StatusOK {
		t.Fatalf("first webhook call: status = %d, want %d", code, http.StatusOK)
	}
	if got := orderStatus(t, app, testOrder.ID); got != order.StatusReadyForDelivery {
		t.Fatalf("order status after first webhook = %q, want %q", got, order.StatusReadyForDelivery)
	}

	// Same notification delivered again (Midtrans retries are expected) —
	// must not error and must not re-run the side effects a second time.
	code = postWebhook(t, app, gatewayTxID, "200", "50000.00", "settlement", "")
	if code != http.StatusOK {
		t.Fatalf("duplicate webhook call: status = %d, want %d", code, http.StatusOK)
	}
	if got := orderStatus(t, app, testOrder.ID); got != order.StatusReadyForDelivery {
		t.Fatalf("order status after duplicate webhook = %q, want %q (must not change)", got, order.StatusReadyForDelivery)
	}
}

// TestPackingCompletionRaceWithPaymentWebhook exercises the real race found
// via manual concurrent-curl testing this session: a customer's payment
// confirmation (webhook) and a packing worker's station completion can
// arrive concurrently. Both code paths are meant to reconcile regardless of
// ordering (completeStation's paid-before-packing retrofit, and the
// webhook's fallthrough-on-stuck-order idempotency check) — this asserts
// the order deterministically reaches ready_for_delivery, run repeatedly to
// increase the chance of hitting any narrow interleaving window.
func TestPackingCompletionRaceWithPaymentWebhook(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			app := testutil.NewTestApp(t)

			customer := app.CreateTestCustomer(t)
			outlet := app.CreateTestOutlet(t)
			address := app.CreateTestAddress(t, customer.ID)
			testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusPacking)

			gatewayTxID := fmt.Sprintf("test-tx-%d", time.Now().UnixNano())
			app.CreateTestPayment(t, testOrder.ID, gatewayTxID, 50000)

			worker := app.CreateTestEmployee(t, "packing_worker", outlet.ID)
			app.EnsureShiftEligibility(t, worker.ID, outlet.ID)
			cookies := testutil.LoginAs(t, app.Router, "/api/v1/employee/auth/login", worker.Email, testutil.TestPassword)
			csrfToken := testutil.CookieValue(cookies, "csrf_token")

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				url := fmt.Sprintf("/api/v1/employee/worker/station/packing/orders/%s/complete", testOrder.ID.String())
				req := httptest.NewRequest(http.MethodPatch, url, nil)
				req.Header.Set("X-CSRF-Token", csrfToken)
				for _, c := range cookies {
					req.AddCookie(c)
				}
				w := httptest.NewRecorder()
				app.Router.ServeHTTP(w, req)
			}()

			go func() {
				defer wg.Done()
				postWebhook(t, app, gatewayTxID, "200", "50000.00", "settlement", "")
			}()

			wg.Wait()

			if got := orderStatus(t, app, testOrder.ID); got != order.StatusReadyForDelivery {
				t.Errorf("order status after concurrent completion+webhook = %q, want %q (order should never be stuck at waiting_payment)", got, order.StatusReadyForDelivery)
			}
		})
	}
}
