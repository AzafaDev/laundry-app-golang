package payment

import (
	"encoding/json"
	"errors"
	db "laundry-app-with-golang/internal/db/generated"
	"math"
	"net/http"
	"strconv"
	"time"

	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

// CreateTransaction creates (or re-creates — see UpsertPaymentForOrder's
// doc comment) a Midtrans Snap transaction for an order the caller owns.
func (h *Handler) CreateTransaction(c *gin.Context) {
	customerID, err := apphelper.CurrentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	ord, err := h.Queries.GetOrderByID(c.Request.Context(), db.GetOrderByIDParams{
		ID:         orderID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "order_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalPrice := apphelper.NumericToFloat64(ord.TotalPrice)
	if totalPrice <= 0 {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_total")
		return
	}

	existing, err := h.Queries.GetPaymentByOrderID(c.Request.Context(), orderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondInternalError(c, err)
		return
	}
	if err == nil && existing.Status == "paid" {
		apperr.RespondError(c, http.StatusBadRequest, "order_already_paid")
		return
	}

	customer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	grossAmount := int64(math.Round(totalPrice))
	midtransOrderID := ord.InvoiceNumber + "-" + strconv.FormatInt(time.Now().UnixMilli(), 10)

	snapResp, midtransErr := snap.CreateTransaction(&snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  midtransOrderID,
			GrossAmt: grossAmount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: customer.FullName,
			Email: customer.Email,
			Phone: customer.Phone.String,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    ord.InvoiceNumber,
				Price: grossAmount,
				Qty:   1,
				Name:  "Order " + ord.InvoiceNumber,
			},
		},
	})
	if midtransErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": midtransErr.GetMessage()})
		return
	}

	responseJSON, err := json.Marshal(snapResp)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	amountNumeric, err := apphelper.Float64ToNumeric(totalPrice)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	payment, err := h.Queries.UpsertPaymentForOrder(c.Request.Context(), db.UpsertPaymentForOrderParams{
		OrderID:              orderID,
		Amount:               amountNumeric,
		GatewayName:          pgtype.Text{String: "midtrans", Valid: true},
		GatewayTransactionID: pgtype.Text{String: midtransOrderID, Valid: true},
		GatewayResponse:      responseJSON,
		PaymentLink:          pgtype.Text{String: snapResp.RedirectURL, Valid: true},
		ExpiredAt:            pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toPaymentResponse(payment)
	resp.Message = "payment transaction created successfully"
	c.JSON(http.StatusCreated, resp)
}

// GetPaymentStatus returns the payment row as last recorded — no gateway
// call.
func (h *Handler) GetPaymentStatus(c *gin.Context) {
	customerID, err := apphelper.CurrentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	if _, err := h.Queries.GetOrderByID(c.Request.Context(), db.GetOrderByIDParams{
		ID:         orderID,
		CustomerID: customerID,
	}); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "order_not_found")
		return
	} else if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	payment, err := h.Queries.GetPaymentByOrderID(c.Request.Context(), orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "payment_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toPaymentResponse(payment))
}

// SyncPaymentStatus is the manual fallback for a missed webhook — it calls
// Midtrans directly and reuses resolvePaymentStatus/applyPaymentStatus, the
// exact same logic path the webhook uses, so the two can never diverge.
func (h *Handler) SyncPaymentStatus(c *gin.Context) {
	customerID, err := apphelper.CurrentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	if _, err := h.Queries.GetOrderByID(c.Request.Context(), db.GetOrderByIDParams{
		ID:         orderID,
		CustomerID: customerID,
	}); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "order_not_found")
		return
	} else if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	payment, err := h.Queries.GetPaymentByOrderID(c.Request.Context(), orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "payment_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if payment.Status == "paid" {
		c.JSON(http.StatusOK, toPaymentResponse(payment))
		return
	}

	status, midtransErr := coreapi.CheckTransaction(payment.GatewayTransactionID.String)
	if midtransErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": midtransErr.GetMessage()})
		return
	}

	newStatus := resolvePaymentStatus(payment.Status, status.TransactionStatus, status.FraudStatus)
	responseJSON, err := json.Marshal(status)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	updated, err := applyPaymentStatus(c.Request.Context(), qtx, payment, newStatus, responseJSON)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toPaymentResponse(updated))
}

// HandleWebhook is the public Midtrans notification endpoint — no auth,
// Midtrans calls this directly.
func (h *Handler) HandleWebhook(c *gin.Context) {
	var body webhookNotification
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !verifySignature(body.OrderID, body.StatusCode, body.GrossAmount, h.Config.MidtransServerKey, body.SignatureKey) {
		apperr.RespondError(c, http.StatusForbidden, "invalid_signature")
		return
	}

	payment, err := h.Queries.GetPaymentByGatewayTransactionID(c.Request.Context(), pgtype.Text{String: body.OrderID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "payment_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	// gross_amount cross-check — not present in the TS source, added here:
	// a valid signature alone doesn't guarantee the amount wasn't tampered
	// with in transit/replay, so it must also match what we recorded when
	// the transaction was created.
	webhookGrossAmount, err := strconv.ParseFloat(body.GrossAmount, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gross_amount"})
		return
	}
	if math.Round(webhookGrossAmount) != math.Round(apphelper.NumericToFloat64(payment.Amount)) {
		apperr.RespondError(c, http.StatusForbidden, "amount_mismatch")
		return
	}

	newStatus := resolvePaymentStatus(payment.Status, body.TransactionStatus, body.FraudStatus)

	// Idempotency short-circuit — but only when the order is already in a
	// consistent state. If newStatus already equals the recorded status but
	// the order is still stuck at waiting_payment (an earlier attempt to
	// transition it didn't go through), don't short-circuit: fall through
	// and retry the transition instead of leaving the order stuck forever.
	if newStatus == payment.Status {
		ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), payment.OrderID)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if ord.Status != orderStatusWaitingPayment {
			c.JSON(http.StatusOK, gin.H{"message": "already processed"})
			return
		}
	}

	gatewayResponseJSON, err := json.Marshal(body)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	if _, err := applyPaymentStatus(c.Request.Context(), qtx, payment, newStatus, gatewayResponseJSON); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification processed"})
}
