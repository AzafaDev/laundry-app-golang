package order

import (
	"context"
	"errors"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/attendance"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/sse"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// respondEligibilityError maps an AssertShiftEligibility error to its HTTP
// status/code, or falls back to 500 for anything unexpected.
func respondEligibilityError(c *gin.Context, err error) {
	var shiftElig *attendance.EligibilityError
	if errors.As(err, &shiftElig) {
		apperr.RespondError(c, shiftElig.Status, shiftElig.Code)
		return
	}

	var driverElig *EligibilityError
	if errors.As(err, &driverElig) {
		apperr.RespondError(c, driverElig.Status, driverElig.Code)
		return
	}

	apperr.RespondInternalError(c, err)
}

// assertStationAccess rejects the request unless the caller's role is the
// one allowed to operate on :station.
func assertStationAccess(c *gin.Context, station string) bool {
	role := currentEmployeeRole(c)
	allowed, ok := stationForRole[role]
	if !ok || allowed != station {
		apperr.RespondError(c, http.StatusForbidden, "station_access_denied")
		return false
	}
	return true
}

func isValidStation(station string) bool {
	_, ok := stationNextStatus[station]
	return ok
}

func (h *Handler) GetStationOrders(c *gin.Context) {
	station := c.Param("station")
	if !isValidStation(station) {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_station")
		return
	}
	if !assertStationAccess(c, station) {
		return
	}

	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	outletID, err := attendance.AssertShiftEligibility(c.Request.Context(), h.Queries, employeeID)
	if err != nil {
		respondEligibilityError(c, err)
		return
	}

	orders, err := h.Queries.ListOrdersByOutletAndStatus(c.Request.Context(), db.ListOrdersByOutletAndStatusParams{
		OutletID: outletID,
		Status:   station,
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

// expectedItems builds the expected breakdown/satuan quantities for an
// order from order_item_breakdowns (clothing_type) and order_items whose
// laundry_item.unit == "pcs" (satuan) — kg-unit order_items describe the
// bulk weight, not a per-item count to compare.
func (h *Handler) expectedItems(ctx context.Context, orderID pgtype.UUID) (breakdown map[string]int32, satuan map[string]int32, err error) {
	breakdown = map[string]int32{}
	satuan = map[string]int32{}

	breakdowns, err := h.Queries.ListOrderItemBreakdownsByOrder(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	for _, b := range breakdowns {
		breakdown[b.ClothingTypeID.String()] = b.Quantity
	}

	items, err := h.Queries.ListOrderItemsByOrder(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range items {
		li, err := h.Queries.GetLaundryItemByIDAny(ctx, item.LaundryItemID)
		if err != nil {
			return nil, nil, err
		}
		if li.Unit == "pcs" {
			satuan[item.LaundryItemID.String()] = int32(numericToFloat64(item.Quantity))
		}
	}

	return breakdown, satuan, nil
}

// compareItems diffs actual submissions against expected quantities. A
// missing submission for an expected item is treated as actual=0, matching
// the TS source's compareItems behavior.
func compareItems(expectedBreakdown, expectedSatuan map[string]int32, req SubmitItemsRequest) []Discrepancy {
	actualBreakdown := map[string]int32{}
	for _, a := range req.ActualItems {
		actualBreakdown[a.ClothingTypeID] = a.ActualQuantity
	}
	actualSatuan := map[string]int32{}
	for _, a := range req.ActualSatuanItems {
		actualSatuan[a.LaundryItemID] = a.ActualQuantity
	}

	var discrepancies []Discrepancy

	for id, expected := range expectedBreakdown {
		actual := actualBreakdown[id]
		if actual != expected {
			discrepancies = append(discrepancies, Discrepancy{
				ItemType: "clothing_type", ItemID: id, Expected: expected, Actual: actual,
			})
		}
	}
	for id, expected := range expectedSatuan {
		actual := actualSatuan[id]
		if actual != expected {
			discrepancies = append(discrepancies, Discrepancy{
				ItemType: "laundry_item", ItemID: id, Expected: expected, Actual: actual,
			})
		}
	}

	return discrepancies
}

// fillDiscrepancyNames resolves the human-readable clothing_type/laundry_item
// name for each discrepancy, so a worker sees "Kemeja" instead of a raw UUID
// when a mismatch is reported.
func (h *Handler) fillDiscrepancyNames(ctx context.Context, discrepancies []Discrepancy) error {
	for i, d := range discrepancies {
		var id pgtype.UUID
		if err := id.Scan(d.ItemID); err != nil {
			return err
		}

		switch d.ItemType {
		case "clothing_type":
			ct, err := h.Queries.GetClothingTypeByID(ctx, id)
			if err != nil {
				return err
			}
			discrepancies[i].Name = ct.Name
		case "laundry_item":
			li, err := h.Queries.GetLaundryItemByIDAny(ctx, id)
			if err != nil {
				return err
			}
			discrepancies[i].Name = li.Name
		}
	}
	return nil
}

func (h *Handler) SubmitItems(c *gin.Context) {
	station := c.Param("station")
	if !isValidStation(station) {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_station")
		return
	}
	if !assertStationAccess(c, station) {
		return
	}

	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if _, err := attendance.AssertShiftEligibility(c.Request.Context(), h.Queries, employeeID); err != nil {
		respondEligibilityError(c, err)
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("orderId")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	var req SubmitItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	expectedBreakdown, expectedSatuan, err := h.expectedItems(c.Request.Context(), orderID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	discrepancies := compareItems(expectedBreakdown, expectedSatuan, req)
	if len(discrepancies) > 0 {
		if err := h.fillDiscrepancyNames(c.Request.Context(), discrepancies); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		c.JSON(http.StatusConflict, SubmitItemsResponse{
			Success:        false,
			RequiresBypass: true,
			Discrepancies:  discrepancies,
		})
		return
	}

	h.completeStation(c, employeeID, station, orderID)
}

func (h *Handler) CompleteStation(c *gin.Context) {
	station := c.Param("station")
	if !isValidStation(station) {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_station")
		return
	}
	if !assertStationAccess(c, station) {
		return
	}

	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if _, err := attendance.AssertShiftEligibility(c.Request.Context(), h.Queries, employeeID); err != nil {
		respondEligibilityError(c, err)
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("orderId")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	h.completeStation(c, employeeID, station, orderID)
}

// completeStation performs the optimistic-concurrency status transition
// shared by SubmitItems (on match) and the direct CompleteStation endpoint:
// UPDATE ... WHERE status = <station> — if no row matched, the order was
// already moved past this station by a concurrent request, so this loses
// the race and reports 409 rather than double-processing.
func (h *Handler) completeStation(c *gin.Context, employeeID pgtype.UUID, station string, orderID pgtype.UUID) {
	nextStatus := stationNextStatus[station]
	paidDeliveryTask := false

	// Retrofit (ticket #2): a customer may pay before packing finishes. If
	// so, skip waiting_payment entirely and go straight to
	// ready_for_delivery, creating the delivery driver_task here instead of
	// leaving the order stuck waiting for a webhook that already fired.
	if station == StatusPacking {
		pay, err := h.Queries.GetPaymentByOrderID(c.Request.Context(), orderID)
		if err == nil && pay.Status == "paid" {
			nextStatus = StatusReadyForDelivery
			paidDeliveryTask = true
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	updated, err := qtx.UpdateOrderStatusIfCurrent(c.Request.Context(), db.UpdateOrderStatusIfCurrentParams{
		Status:   nextStatus,
		ID:       orderID,
		Status_2: station,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "station_already_processed")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if paidDeliveryTask {
		if _, err := qtx.CreateDriverTask(c.Request.Context(), db.CreateDriverTaskParams{
			OrderID:  orderID,
			TaskType: "delivery",
		}); err != nil && !isUniqueViolation(err) {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	if _, err := qtx.CreateOrderStatusHistory(c.Request.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       orderID,
		OldStatus:     pgtype.Text{String: station, Valid: true},
		NewStatus:     nextStatus,
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

	outletChannel := "outlet:" + updated.OutletID.String()
	sse.Default.Broadcast(outletChannel, "station:order-completed", gin.H{
		"orderID":   updated.ID.String(),
		"station":   station,
		"newStatus": nextStatus,
		"workerID":  employeeID.String(),
		"outletID":  updated.OutletID.String(),
		"timestamp": time.Now(),
	})
	if nextStatus == StatusWashing || nextStatus == StatusIroning || nextStatus == StatusPacking {
		sse.Default.Broadcast(outletChannel, "station:new-order", gin.H{
			"station": nextStatus,
			"orderID": updated.ID.String(),
		})
	}
	sse.Default.Broadcast("user:"+updated.CustomerID.String(), "order:status-updated", gin.H{
		"orderID": updated.ID.String(),
		"status":  nextStatus,
	})
	if paidDeliveryTask {
		sse.Default.Broadcast(outletChannel, "order:payment-completed", gin.H{
			"orderID":       updated.ID.String(),
			"invoiceNumber": updated.InvoiceNumber,
			"timestamp":     time.Now(),
		})
	}

	// Customer-facing notification only fires when this completion resulted
	// in waiting_payment or ready_for_delivery — which, per stationNextStatus,
	// only ever happens at packing (washing/ironing produce other statuses).
	// This mirrors the TS source's emitStationEvents exactly without needing
	// a separate station==packing check.
	if nextStatus == StatusWaitingPayment || nextStatus == StatusReadyForDelivery {
		title, body := "Pembayaran Diperlukan", fmt.Sprintf("Pesanan %s sudah selesai diproses. Silakan lakukan pembayaran.", updated.InvoiceNumber)
		if nextStatus == StatusReadyForDelivery {
			title, body = "Pesanan Siap Dikirim", fmt.Sprintf("Pesanan %s sudah selesai dan siap untuk dikirim.", updated.InvoiceNumber)
		}
		_ = notification.NotifyCustomer(c.Request.Context(), h.Queries, updated.CustomerID, title, body, notification.TypeOrderUpdate, updated.ID)
		_ = notification.NotifyOutletEmployees(c.Request.Context(), h.Queries, updated.OutletID, []string{"outlet_admin"},
			"Pesanan Selesai Diproses", fmt.Sprintf("Pesanan %s selesai di packing.", updated.InvoiceNumber), notification.TypeOrderUpdate, updated.ID)
	}

	resp := toOrderResponse(updated)
	resp.Message = "station completed successfully"
	c.JSON(http.StatusOK, SubmitItemsResponse{Success: true, Data: &resp})
}
