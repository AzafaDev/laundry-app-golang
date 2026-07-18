package order

import (
	"errors"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetPendingProcessOrders lists orders at this outlet_admin's outlet
// waiting to be processed (status laundry_arrived_outlet).
func (h *Handler) GetPendingProcessOrders(c *gin.Context) {
	outletID, hasOutlet := apphelper.CurrentEmployeeOutletID(c)
	if !hasOutlet {
		apperr.RespondError(c, http.StatusForbidden, "no_outlet_assigned")
		return
	}

	orders, err := h.Queries.ListOrdersByOutletAndStatus(c.Request.Context(), db.ListOrdersByOutletAndStatusParams{
		OutletID: outletID,
		Status:   StatusLaundryArrivedOutlet,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		data = append(data, toOrderResponse(o))
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// ProcessOrder is the outlet_admin action that records actual item/weight
// counts when laundry arrives at the outlet, transitioning the order from
// laundry_arrived_outlet to washing. Replicates processOrderSchema from the
// TS source verbatim, including its odd-but-intentional rule that
// total_weight_kg must be a whole number when >0 (not enforced at the DB
// level — the column is NUMERIC(6,2) — this is an app-level business rule
// carried over as-is, not a bug to fix here).
func (h *Handler) ProcessOrder(c *gin.Context) {
	employeeID, err := apphelper.CurrentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	callerOutletID, hasOutlet := apphelper.CurrentEmployeeOutletID(c)
	if !hasOutlet {
		apperr.RespondError(c, http.StatusForbidden, "no_outlet_assigned")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "order_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if ord.OutletID != callerOutletID {
		apperr.RespondError(c, http.StatusForbidden, "order_not_in_your_outlet")
		return
	}

	if ord.Status != StatusLaundryArrivedOutlet {
		apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_order_status")
		return
	}

	existingItemCount, err := h.Queries.CountOrderItemsByOrder(c.Request.Context(), orderID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if existingItemCount > 0 {
		apperr.RespondError(c, http.StatusConflict, "order_already_processed")
		return
	}

	var req ProcessOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if code := validateProcessOrderRequest(req); code != "" {
		apperr.RespondError(c, http.StatusBadRequest, code)
		return
	}

	type resolvedItem struct {
		laundryItemID pgtype.UUID
		quantity      float64
		basePrice     float64
	}

	resolvedItems := make([]resolvedItem, 0, len(req.Items))
	totalPrice := 0.0

	for _, item := range req.Items {
		var laundryItemID pgtype.UUID
		if err := laundryItemID.Scan(item.LaundryItemID); err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_laundry_item_id")
			return
		}

		li, err := h.Queries.GetLaundryItemByID(c.Request.Context(), laundryItemID)
		if errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_laundry_item")
			return
		}
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if !li.IsActive {
			apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_laundry_item")
			return
		}

		basePrice := apphelper.NumericToFloat64(li.BasePrice)
		totalPrice += basePrice * item.Quantity

		resolvedItems = append(resolvedItems, resolvedItem{
			laundryItemID: laundryItemID,
			quantity:      item.Quantity,
			basePrice:     basePrice,
		})
	}

	type resolvedBreakdown struct {
		clothingTypeID pgtype.UUID
		quantity       int32
	}

	resolvedBreakdowns := make([]resolvedBreakdown, 0, len(req.Breakdown))
	for _, b := range req.Breakdown {
		var clothingTypeID pgtype.UUID
		if err := clothingTypeID.Scan(b.ClothingTypeID); err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_clothing_type_id")
			return
		}

		ct, err := h.Queries.GetClothingTypeByID(c.Request.Context(), clothingTypeID)
		if errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_clothing_type")
			return
		}
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if !ct.IsActive {
			apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_clothing_type")
			return
		}

		resolvedBreakdowns = append(resolvedBreakdowns, resolvedBreakdown{
			clothingTypeID: clothingTypeID,
			quantity:       b.Quantity,
		})
	}

	totalPrice += apphelper.NumericToFloat64(ord.DeliveryFee)

	totalPriceNumeric, err := apphelper.Float64ToNumeric(totalPrice)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	totalWeightNumeric, err := apphelper.Float64ToNumeric(req.TotalWeightKG)
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

	for _, item := range resolvedItems {
		quantityNumeric, err := apphelper.Float64ToNumeric(item.quantity)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		priceNumeric, err := apphelper.Float64ToNumeric(item.basePrice)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if _, err := qtx.CreateOrderItem(c.Request.Context(), db.CreateOrderItemParams{
			OrderID:       orderID,
			LaundryItemID: item.laundryItemID,
			Quantity:      quantityNumeric,
			PriceAtOrder:  priceNumeric,
		}); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	for _, b := range resolvedBreakdowns {
		if _, err := qtx.CreateOrderItemBreakdown(c.Request.Context(), db.CreateOrderItemBreakdownParams{
			OrderID:        orderID,
			ClothingTypeID: b.clothingTypeID,
			Quantity:       b.quantity,
			CreatedBy:      employeeID,
		}); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	updated, err := qtx.ProcessOrderIfCurrent(c.Request.Context(), db.ProcessOrderIfCurrentParams{
		Status:        StatusWashing,
		TotalPrice:    totalPriceNumeric,
		TotalWeightKg: totalWeightNumeric,
		ID:            orderID,
		Status_2:      StatusLaundryArrivedOutlet,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "order_already_processed")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	oldStatus := StatusLaundryArrivedOutlet
	if _, err := qtx.CreateOrderStatusHistory(c.Request.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       orderID,
		OldStatus:     pgtype.Text{String: oldStatus, Valid: true},
		NewStatus:     StatusWashing,
		ChangedByType: "employee",
		ChangedByID:   employeeID,
		Note:          pgtype.Text{Valid: false},
	}); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	_ = notification.NotifyCustomer(c.Request.Context(), h.Queries, updated.CustomerID,
		"Detail pesanan telah diinput",
		fmt.Sprintf("Pesanan %s: laundry Rp%.0f + ongkir Rp%.0f = total Rp%.0f.",
			updated.InvoiceNumber, totalPrice-apphelper.NumericToFloat64(updated.DeliveryFee), apphelper.NumericToFloat64(updated.DeliveryFee), totalPrice),
		notification.TypeOrderDetails, updated.ID)
	_ = notification.NotifyCustomer(c.Request.Context(), h.Queries, updated.CustomerID,
		"Tagihan Pembayaran",
		fmt.Sprintf("Pesanan %s perlu dibayar sebesar Rp%.0f.", updated.InvoiceNumber, totalPrice),
		notification.TypePayment, updated.ID)

	resp := toOrderResponse(updated)
	resp.Message = "order processed successfully"
	c.JSON(http.StatusOK, resp)
}

// validateProcessOrderRequest replicates processOrderSchema's .superRefine
// rules verbatim: at least one item quantity>0, total_weight_kg must be a
// whole number when >0, and breakdown/weight must be filled together
// (either both zero/empty or both present).
func validateProcessOrderRequest(req ProcessOrderRequest) string {
	hasAnyItem := false
	for _, item := range req.Items {
		if item.Quantity > 0 {
			hasAnyItem = true
			break
		}
	}
	if !hasAnyItem {
		return "no_items_with_quantity"
	}

	if req.TotalWeightKG < 0 || req.TotalWeightKG > 999.99 {
		return "invalid_total_weight"
	}
	if req.TotalWeightKG > 0 && req.TotalWeightKG != float64(int64(req.TotalWeightKG)) {
		return "total_weight_must_be_integer"
	}

	hasBreakdown := false
	for _, b := range req.Breakdown {
		if b.Quantity > 0 {
			hasBreakdown = true
			break
		}
	}

	if hasBreakdown && req.TotalWeightKG == 0 {
		return "breakdown_without_weight"
	}
	if req.TotalWeightKG > 0 && !hasBreakdown {
		return "weight_without_breakdown"
	}

	return ""
}
