package cron

import (
	"context"
	"errors"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/order"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunAutoCompleteOrders transitions every order sitting in
// received_by_customer whose auto_confirm_at has passed to completed.
// Unlike the TS source (a plain unconditional update after the candidate
// query), each transition is guarded by UpdateOrderStatusIfCurrent — if a
// customer manually confirmed the same order in the window between the
// candidate query and this update, that order is skipped here rather than
// double-transitioned; it simply won't be selected again on the next tick
// since it's no longer received_by_customer.
func RunAutoCompleteOrders(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) (completed int, err error) {
	candidates, err := queries.ListOrdersReadyForAutoComplete(ctx)
	if err != nil {
		return 0, err
	}

	for _, ord := range candidates {
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Printf("cron: auto-complete order %s: begin tx: %v", ord.ID, err)
			continue
		}

		qtx := queries.WithTx(tx)

		updated, err := qtx.UpdateOrderStatusIfCurrent(ctx, db.UpdateOrderStatusIfCurrentParams{
			Status:   order.StatusCompleted,
			ID:       ord.ID,
			Status_2: order.StatusReceivedByCustomer,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(ctx)
			continue
		}
		if err != nil {
			tx.Rollback(ctx)
			log.Printf("cron: auto-complete order %s: %v", ord.ID, err)
			continue
		}

		if _, err := qtx.CreateOrderStatusHistory(ctx, db.CreateOrderStatusHistoryParams{
			OrderID:       updated.ID,
			OldStatus:     pgtype.Text{String: order.StatusReceivedByCustomer, Valid: true},
			NewStatus:     order.StatusCompleted,
			ChangedByType: "system",
			ChangedByID:   pgtype.UUID{Valid: false},
			Note:          pgtype.Text{String: "Pesanan dikonfirmasi otomatis setelah 2x24 jam.", Valid: true},
		}); err != nil {
			tx.Rollback(ctx)
			log.Printf("cron: auto-complete order %s: history: %v", ord.ID, err)
			continue
		}

		if err := tx.Commit(ctx); err != nil {
			log.Printf("cron: auto-complete order %s: commit: %v", ord.ID, err)
			continue
		}

		if err := notification.NotifyCustomer(ctx, queries, updated.CustomerID, "Pesanan selesai otomatis",
			fmt.Sprintf("Pesanan %s telah dikonfirmasi selesai secara otomatis.", updated.InvoiceNumber),
			notification.TypeOrderUpdate, updated.ID); err != nil {
			log.Printf("cron: auto-complete order %s: notify: %v", ord.ID, err)
		}

		completed++
	}

	return completed, nil
}
