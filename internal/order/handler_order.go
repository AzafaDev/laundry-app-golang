package order

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CreateOrder(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pickupAddressID pgtype.UUID
	if err := pickupAddressID.Scan(req.PickupAddressID); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_pickup_address_id")
		return
	}

	pickupDate, err := time.Parse("2006-01-02", req.PickupDate)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_pickup_date")
		return
	}

	todayStart := time.Now().Truncate(24 * time.Hour)
	maxPickupDate := todayStart.AddDate(0, 0, MaxPickupDaysAhead)
	if pickupDate.Before(todayStart) || pickupDate.After(maxPickupDate) {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_pickup_date_window")
		return
	}

	address, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         pickupAddressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "address_not_found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	activeOutlets, err := h.Queries.ListActiveOutlets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outlet, distanceKM, ok := nearestOutlet(activeOutlets, numericToFloat64(address.Latitude), numericToFloat64(address.Longitude))
	if !ok {
		apperr.RespondError(c, http.StatusBadRequest, "no_outlet_in_range")
		return
	}

	deliveryFee, err := float64ToNumeric(calculateDeliveryFee(distanceKM))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPrice, err := float64ToNumeric(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pickupDatePg := pgtype.Date{Time: pickupDate, Valid: true}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	created, err := createOrderWithUniqueInvoice(c.Request.Context(), qtx, db.CreateOrderParams{
		CustomerID:      customerID,
		OutletID:        outlet.ID,
		PickupAddressID: pickupAddressID,
		Status:          StatusWaitingPickupDriver,
		PickupDate:      pickupDatePg,
		DeliveryFee:     deliveryFee,
		TotalPrice:      totalPrice,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, err := qtx.CreateOrderStatusHistory(c.Request.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       created.ID,
		OldStatus:     pgtype.Text{Valid: false},
		NewStatus:     StatusWaitingPickupDriver,
		ChangedByType: "customer",
		ChangedByID:   customerID,
		Note:          pgtype.Text{Valid: false},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Retrofit for ticket #4: a pickup driver_task must exist for every
	// order or it can never leave waiting_pickup_driver. TS creates this in
	// the same transaction as the order (order.create.service.ts:75).
	if _, err := qtx.CreateDriverTask(c.Request.Context(), db.CreateDriverTaskParams{
		OrderID:  created.ID,
		TaskType: "pickup",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toOrderResponse(created)
	resp.Message = "order created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListOrders(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	limit, offset := parsePagination(c)

	status := pgtype.Text{Valid: false}
	if v := c.Query("status"); v != "" {
		status = pgtype.Text{String: v, Valid: true}
	}

	search := pgtype.Text{Valid: false}
	if v := c.Query("search"); v != "" {
		search = pgtype.Text{String: v, Valid: true}
	}

	dateFrom := pgtype.Timestamptz{Valid: false}
	if v := c.Query("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			dateFrom = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	dateTo := pgtype.Timestamptz{Valid: false}
	if v := c.Query("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			dateTo = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Nanosecond), Valid: true}
		}
	}

	orders, err := h.Queries.ListOrders(c.Request.Context(), db.ListOrdersParams{
		CustomerID: customerID,
		Status:     status,
		Search:     search,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalCount, err := h.Queries.CountOrders(c.Request.Context(), db.CountOrdersParams{
		CustomerID: customerID,
		Status:     status,
		Search:     search,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		data = append(data, toOrderResponse(o))
	}

	c.JSON(http.StatusOK, OrderListResponse{Data: data, TotalCount: totalCount})
}

const (
	defaultPageLimit = 10
	maxPageLimit     = 100
)

func parsePagination(c *gin.Context) (limit, offset int32) {
	limit = defaultPageLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

func toOrderResponse(o db.Order) OrderResponse {
	return OrderResponse{
		ID:              o.ID.String(),
		InvoiceNumber:   o.InvoiceNumber,
		OutletID:        o.OutletID.String(),
		PickupAddressID: o.PickupAddressID.String(),
		Status:          o.Status,
		PickupDate:      o.PickupDate.Time.Format("2006-01-02"),
		DeliveryFee:     numericToFloat64(o.DeliveryFee),
		TotalPrice:      numericToFloat64(o.TotalPrice),
		CreatedAt:       o.CreatedAt.Time.Format(time.RFC3339),
	}
}
