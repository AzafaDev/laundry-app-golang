package payment

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/sse"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

const (
	// StatusWaitingPayment/StatusReadyForDelivery mirror internal/order's
	// constants of the same name — duplicated rather than imported to avoid
	// internal/order and internal/payment needing to import each other
	// (both only need the raw status strings, not each other's types).
	orderStatusWaitingPayment   = "waiting_payment"
	orderStatusReadyForDelivery = "ready_for_delivery"
)

func currentCustomerID(c *gin.Context) (pgtype.UUID, error) {
	var customerUUID pgtype.UUID

	customerIDVal, ok := c.Get("customer_id")
	if !ok {
		return customerUUID, errors.New("something went wrong")
	}
	customerIDStr, ok := customerIDVal.(string)
	if !ok {
		return customerUUID, errors.New("something went wrong")
	}
	if err := customerUUID.Scan(customerIDStr); err != nil {
		return customerUUID, err
	}
	return customerUUID, nil
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

// verifySignature replicates Midtrans's documented SHA-512 signature
// scheme: sha512(order_id + status_code + gross_amount + server_key). The
// SDK has no helper for this — it must be computed manually.
func verifySignature(orderID, statusCode, grossAmount, serverKey, signatureKey string) bool {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	expected := hex.EncodeToString(sum[:])
	return expected == signatureKey
}

// resolvePaymentStatus mirrors the TS state machine exactly: capture only
// counts as paid when fraud_status is "accept" (otherwise it's held as
// pending for manual review); settlement is always paid; expire/deny/
// cancel/failure map directly; anything else keeps the current status.
func resolvePaymentStatus(currentStatus, transactionStatus, fraudStatus string) string {
	switch transactionStatus {
	case "capture":
		if fraudStatus == "accept" {
			return "paid"
		}
		return "pending"
	case "settlement":
		return "paid"
	case "expire":
		return "expired"
	case "deny", "cancel", "failure":
		return "failed"
	case "pending":
		return "pending"
	default:
		return currentStatus
	}
}

// applyPaymentStatus writes the resolved status to the payment row and,
// when it resolves to "paid", advances the order to ready_for_delivery and
// creates its delivery driver_task — all inside the given transaction-scoped
// queries. Shared by both the webhook and SyncPaymentStatus so the two
// entry points can never drift apart.
func applyPaymentStatus(ctx context.Context, qtx *db.Queries, pay db.Payment, newStatus string, gatewayResponse []byte) (db.Payment, error) {
	paidAt := pgtype.Timestamptz{Valid: false}
	if newStatus == "paid" {
		paidAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	updatedPayment, err := qtx.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		Status:          newStatus,
		GatewayResponse: gatewayResponse,
		PaidAt:          paidAt,
		ID:              pay.ID,
	})
	if err != nil {
		return db.Payment{}, err
	}

	if newStatus != "paid" {
		return updatedPayment, nil
	}

	// Customer notification fires unconditionally whenever the payment is
	// confirmed paid — not just when the order successfully transitions
	// (mirrors the TS source, which notifies the customer outside the
	// waiting_payment-only block).
	ord, err := qtx.GetOrderByIDAny(ctx, pay.OrderID)
	if err == nil {
		_ = notification.NotifyCustomer(ctx, qtx, ord.CustomerID, "Pembayaran berhasil",
			fmt.Sprintf("Pembayaran untuk pesanan %s telah berhasil dikonfirmasi.", ord.InvoiceNumber),
			notification.TypePayment, ord.ID)
	}

	// Advance the order only if it's still waiting_payment. If it isn't
	// (order hasn't reached that station yet), this is not an error — the
	// packing-completion retrofit in internal/order will pick up the
	// already-paid payment once the order gets there, and a future webhook
	// delivery with the same "paid" status will retry this transition
	// (idempotency short-circuit only applies once the order is actually
	// consistent — see handler_payment.go).
	updatedOrder, err := qtx.UpdateOrderStatusIfCurrent(ctx, db.UpdateOrderStatusIfCurrentParams{
		Status:   orderStatusReadyForDelivery,
		ID:       pay.OrderID,
		Status_2: orderStatusWaitingPayment,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return updatedPayment, nil
	}
	if err != nil {
		return db.Payment{}, err
	}

	if _, err := qtx.CreateDriverTask(ctx, db.CreateDriverTaskParams{
		OrderID:  updatedOrder.ID,
		TaskType: "delivery",
	}); err != nil && !isUniqueViolation(err) {
		return db.Payment{}, err
	}

	if _, err := qtx.CreateOrderStatusHistory(ctx, db.CreateOrderStatusHistoryParams{
		OrderID:       updatedOrder.ID,
		OldStatus:     pgtype.Text{String: orderStatusWaitingPayment, Valid: true},
		NewStatus:     orderStatusReadyForDelivery,
		ChangedByType: "system",
		ChangedByID:   pgtype.UUID{Valid: false},
		Note:          pgtype.Text{String: "payment confirmed", Valid: true},
	}); err != nil {
		return db.Payment{}, err
	}

	if updatedOrder.OutletID.Valid {
		title := "Pembayaran berhasil"
		body := fmt.Sprintf("Pesanan %s telah dibayar oleh customer.", updatedOrder.InvoiceNumber)
		outletChannel := "outlet:" + updatedOrder.OutletID.String()

		sse.Default.Broadcast(outletChannel, "order:payment-completed", gin.H{
			"orderID":       updatedOrder.ID.String(),
			"invoiceNumber": updatedOrder.InvoiceNumber,
			"timestamp":     time.Now(),
		})
		sse.Default.Broadcast(outletChannel, "outlet:payment-received", gin.H{
			"outletID":        updatedOrder.OutletID.String(),
			"title":           title,
			"body":            body,
			"relatedEntityID": updatedOrder.ID.String(),
		})

		_ = notification.NotifyOutletEmployees(ctx, qtx, updatedOrder.OutletID, []string{"outlet_admin", "driver"},
			title, body, notification.TypePaymentCompleted, updatedOrder.ID)
	}

	return updatedPayment, nil
}

func toPaymentResponse(p db.Payment) PaymentResponse {
	resp := PaymentResponse{
		ID:            p.ID.String(),
		OrderID:       p.OrderID.String(),
		Amount:        numericToFloat64(p.Amount),
		PaymentMethod: p.PaymentMethod,
		GatewayName:   p.GatewayName.String,
		Status:        p.Status,
	}
	if p.GatewayTransactionID.Valid {
		resp.GatewayTransactionID = p.GatewayTransactionID.String
	}
	if p.PaymentLink.Valid {
		resp.PaymentLink = p.PaymentLink.String
	}
	if p.ExpiredAt.Valid {
		resp.ExpiredAt = p.ExpiredAt.Time.Format(time.RFC3339)
	}
	if p.PaidAt.Valid {
		resp.PaidAt = p.PaidAt.Time.Format(time.RFC3339)
	}
	return resp
}
