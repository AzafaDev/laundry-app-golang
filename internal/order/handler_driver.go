package order

import (
	"context"
	"errors"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/sse"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// taskDistanceKM computes the outlet<->pickup_address haversine distance
// for a driver_tasks row, rounded to 1 decimal (replicates
// Math.round(km*10)/10 from the TS source). Both pickup and delivery tasks
// use the order's pickup_address_id — there is no separate delivery
// address in this schema.
func (h *Handler) taskDistanceKM(ctx context.Context, ord db.Order) (float64, error) {
	outlet, err := h.Queries.GetOutletByID(ctx, ord.OutletID)
	if err != nil {
		return 0, err
	}
	addr, err := h.Queries.GetAddressByIDAny(ctx, ord.PickupAddressID)
	if err != nil {
		return 0, err
	}

	km := haversineKM(
		numericToFloat64(outlet.Latitude), numericToFloat64(outlet.Longitude),
		numericToFloat64(addr.Latitude), numericToFloat64(addr.Longitude),
	)
	return math.Round(km*10) / 10, nil
}

func (h *Handler) toDriverTaskResponse(ctx context.Context, t db.DriverTask, withDistance bool) (DriverTaskResponse, error) {
	resp := DriverTaskResponse{
		ID:       t.ID.String(),
		OrderID:  t.OrderID.String(),
		TaskType: t.TaskType,
		Status:   t.Status,
	}
	if t.DriverID.Valid {
		resp.DriverID = t.DriverID.String()
	}
	if t.TakenAt.Valid {
		resp.TakenAt = t.TakenAt.Time.Format(time.RFC3339)
	}
	if t.CompletedAt.Valid {
		resp.CompletedAt = t.CompletedAt.Time.Format(time.RFC3339)
	}

	ord, err := h.Queries.GetOrderByIDAny(ctx, t.OrderID)
	if err != nil {
		return DriverTaskResponse{}, err
	}
	resp.InvoiceNumber = ord.InvoiceNumber

	if withDistance {
		distanceKM, err := h.taskDistanceKM(ctx, ord)
		if err != nil {
			return DriverTaskResponse{}, err
		}
		resp.DistanceKM = distanceKM
	}

	return resp, nil
}

func (h *Handler) listAvailableDriverTasks(c *gin.Context, taskType string) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if _, err := assertDriverEligibility(c.Request.Context(), h.Queries, employeeID); err != nil {
		respondEligibilityError(c, err)
		return
	}

	tasks, err := h.Queries.ListAvailableDriverTasksByType(c.Request.Context(), taskType)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]DriverTaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp, err := h.toDriverTaskResponse(c.Request.Context(), t, true)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		data = append(data, resp)
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) GetAvailablePickups(c *gin.Context) {
	h.listAvailableDriverTasks(c, "pickup")
}

func (h *Handler) GetAvailableDeliveries(c *gin.Context) {
	h.listAvailableDriverTasks(c, "delivery")
}

func (h *Handler) GetActiveTask(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	task, err := h.Queries.GetActiveDriverTaskByDriver(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp, err := h.toDriverTaskResponse(c.Request.Context(), task, true)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// ClaimTask atomically assigns an available task to the calling driver.
// Mirrors the TS double-check pattern: a friendly pre-check for a clear
// error message, then the real optimistic-concurrency guard inside the
// transaction (UPDATE ... WHERE status='available' AND driver_id IS NULL)
// — two drivers racing the same task must yield exactly one success.
func (h *Handler) ClaimTask(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if _, err := assertDriverEligibility(c.Request.Context(), h.Queries, employeeID); err != nil {
		respondEligibilityError(c, err)
		return
	}

	var taskID pgtype.UUID
	if err := taskID.Scan(c.Param("taskId")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_task_id")
		return
	}

	preCheck, err := h.Queries.GetDriverTaskByID(c.Request.Context(), taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "task_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if preCheck.Status != "available" || preCheck.DriverID.Valid {
		apperr.RespondError(c, http.StatusConflict, "task_already_claimed")
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	claimed, err := qtx.ClaimDriverTaskIfAvailable(c.Request.Context(), db.ClaimDriverTaskIfAvailableParams{
		DriverID: employeeID,
		ID:       taskID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "task_already_claimed")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	nextStatus := claimNextStatus[claimed.TaskType]
	oldStatus := claimOldStatus[claimed.TaskType]

	pickupSchedule := pgtype.Timestamptz{Valid: false}
	if claimed.TaskType == "pickup" {
		pickupSchedule = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	// No status guard here — replicates TS's runClaimTransaction, which
	// sets the order status unconditionally once the task claim itself
	// (above) has already succeeded atomically.
	updatedOrder, err := qtx.ClaimOrderForTask(c.Request.Context(), db.ClaimOrderForTaskParams{
		Status:         nextStatus,
		PickupSchedule: pickupSchedule,
		ID:             claimed.OrderID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if _, err := qtx.CreateOrderStatusHistory(c.Request.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       updatedOrder.ID,
		OldStatus:     pgtype.Text{String: oldStatus, Valid: true},
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

	sse.Default.Broadcast("outlet:"+updatedOrder.OutletID.String(), "driver:task-claimed", gin.H{
		"taskID":   claimed.ID.String(),
		"driverID": employeeID.String(),
		"orderID":  claimed.OrderID.String(),
		"taskType": claimed.TaskType,
	})

	claimTitle := "Driver dalam perjalanan"
	claimBody := fmt.Sprintf("Driver sedang menuju lokasi penjemputan untuk pesanan %s.", updatedOrder.InvoiceNumber)
	claimNotifType := notification.TypeDriverPickupStarted
	if claimed.TaskType == "delivery" {
		claimBody = fmt.Sprintf("Driver sedang mengantarkan pesanan %s ke lokasi Anda.", updatedOrder.InvoiceNumber)
		claimNotifType = notification.TypeDriverDeliveryStarted
	}
	_ = notification.NotifyCustomer(c.Request.Context(), h.Queries, updatedOrder.CustomerID, claimTitle, claimBody, claimNotifType, updatedOrder.ID)

	resp, err := h.toDriverTaskResponse(c.Request.Context(), claimed, false)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	resp.Message = "task claimed successfully"
	c.JSON(http.StatusOK, resp)
}

// CompleteTask uses two separate optimistic-concurrency guards inside one
// transaction — task (in_progress + owned by this driver) and order
// (status matches what this task type expects) — mirroring TS's
// runCompleteTransaction exactly rather than merging them into one guard.
func (h *Handler) CompleteTask(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var taskID pgtype.UUID
	if err := taskID.Scan(c.Param("taskId")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_task_id")
		return
	}

	preCheck, err := h.Queries.GetDriverTaskByID(c.Request.Context(), taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "task_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if preCheck.Status != "in_progress" || preCheck.DriverID != employeeID {
		apperr.RespondError(c, http.StatusConflict, "task_not_completable")
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	completed, err := qtx.CompleteDriverTaskIfInProgress(c.Request.Context(), db.CompleteDriverTaskIfInProgressParams{
		ID:       taskID,
		DriverID: employeeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "task_not_completable")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	nextStatus := completeNextStatus[completed.TaskType]
	oldStatus := completeOldStatus[completed.TaskType]

	autoConfirmAt := pgtype.Timestamptz{Valid: false}
	if nextStatus == StatusReceivedByCustomer {
		autoConfirmAt = pgtype.Timestamptz{Time: time.Now().Add(48 * time.Hour), Valid: true}
	}

	updatedOrder, err := qtx.CompleteOrderForTaskIfCurrent(c.Request.Context(), db.CompleteOrderForTaskIfCurrentParams{
		Status:        nextStatus,
		AutoConfirmAt: autoConfirmAt,
		ID:            completed.OrderID,
		Status_2:      oldStatus,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "order_status_mismatch")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if _, err := qtx.CreateOrderStatusHistory(c.Request.Context(), db.CreateOrderStatusHistoryParams{
		OrderID:       updatedOrder.ID,
		OldStatus:     pgtype.Text{String: oldStatus, Valid: true},
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

	sse.Default.Broadcast("outlet:"+updatedOrder.OutletID.String(), "driver:task-completed", gin.H{
		"taskID":      completed.ID.String(),
		"taskType":    completed.TaskType,
		"orderID":     completed.OrderID.String(),
		"driverID":    employeeID.String(),
		"completedAt": time.Now(),
	})
	sse.Default.Broadcast("user:"+updatedOrder.CustomerID.String(), "order:status-updated", gin.H{
		"orderID": updatedOrder.ID.String(),
		"status":  nextStatus,
	})

	completeTitle, completeBody, completeNotifType := "Driver telah tiba di outlet", "Laundry Anda telah tiba di outlet dan akan segera diproses.", notification.TypeDriverArrivedOutlet
	if completed.TaskType == "delivery" {
		completeTitle, completeBody, completeNotifType = "Driver telah tiba", "Driver telah tiba di lokasi Anda dengan pesanan laundry Anda.", notification.TypeDriverArrivedCustomer
	}
	_ = notification.NotifyCustomer(c.Request.Context(), h.Queries, updatedOrder.CustomerID, completeTitle, completeBody, completeNotifType, updatedOrder.ID)

	resp, err := h.toDriverTaskResponse(c.Request.Context(), completed, false)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	resp.Message = "task completed successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetTaskHistory(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	limit, offset := parsePagination(c)

	tasks, err := h.Queries.ListDriverTaskHistory(c.Request.Context(), db.ListDriverTaskHistoryParams{
		DriverID: employeeID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountDriverTaskHistory(c.Request.Context(), employeeID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]DriverTaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp, err := h.toDriverTaskResponse(c.Request.Context(), t, false)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		data = append(data, resp)
	}

	c.JSON(http.StatusOK, DriverTaskListResponse{Data: data, TotalCount: totalCount})
}
