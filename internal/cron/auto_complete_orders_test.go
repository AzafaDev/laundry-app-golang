package cron_test

import (
	"laundry-app-with-golang/internal/cron"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"testing"
	"time"
)

func TestRunAutoCompleteOrders(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusReceivedByCustomer)

	past := time.Now().Add(-1 * time.Hour)
	if _, err := app.Pool.Exec(t.Context(), "UPDATE orders SET auto_confirm_at = $1 WHERE id = $2", past, testOrder.ID); err != nil {
		t.Fatalf("failed to set auto_confirm_at: %v", err)
	}

	completed, err := cron.RunAutoCompleteOrders(t.Context(), app.Pool, app.Queries)
	if err != nil {
		t.Fatalf("RunAutoCompleteOrders returned error: %v", err)
	}
	if completed < 1 {
		t.Errorf("completed = %d, want at least 1", completed)
	}

	var status string
	row := app.Pool.QueryRow(t.Context(), "SELECT status FROM orders WHERE id = $1", testOrder.ID)
	if err := row.Scan(&status); err != nil {
		t.Fatalf("failed to read back order status: %v", err)
	}
	if status != order.StatusCompleted {
		t.Errorf("order status = %q, want %q", status, order.StatusCompleted)
	}

	var changedByType string
	historyRow := app.Pool.QueryRow(t.Context(), "SELECT changed_by_type FROM order_status_histories WHERE order_id = $1 AND new_status = $2", testOrder.ID, order.StatusCompleted)
	if err := historyRow.Scan(&changedByType); err != nil {
		t.Fatalf("failed to read back order_status_histories: %v", err)
	}
	if changedByType != "system" {
		t.Errorf("order_status_histories.changed_by_type = %q, want %q", changedByType, "system")
	}
}
