package order

import (
	"context"
	"encoding/json"
	"errors"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	"laundry-app-with-golang/internal/attendance"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/sse"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// buildNormalizedItems resolves clothing_type/laundry_item names for the
// given quantity maps, producing the snapshot stored in bypass_requests'
// JSONB columns.
func (h *Handler) buildNormalizedItems(ctx context.Context, breakdown, satuan map[string]int32) ([]NormalizedItem, error) {
	items := make([]NormalizedItem, 0, len(breakdown)+len(satuan))

	for id, qty := range breakdown {
		var ctID pgtype.UUID
		if err := ctID.Scan(id); err != nil {
			return nil, err
		}
		ct, err := h.Queries.GetClothingTypeByID(ctx, ctID)
		if err != nil {
			return nil, err
		}
		items = append(items, NormalizedItem{ItemType: "clothing_type", ItemID: id, Name: ct.Name, Quantity: qty})
	}

	for id, qty := range satuan {
		var liID pgtype.UUID
		if err := liID.Scan(id); err != nil {
			return nil, err
		}
		li, err := h.Queries.GetLaundryItemByIDAny(ctx, liID)
		if err != nil {
			return nil, err
		}
		items = append(items, NormalizedItem{ItemType: "laundry_item", ItemID: id, Name: li.Name, Quantity: qty})
	}

	return items, nil
}

func actualItemMaps(req CreateBypassRequest) (breakdown, satuan map[string]int32) {
	breakdown = map[string]int32{}
	for _, a := range req.ActualItems {
		breakdown[a.ClothingTypeID] = a.ActualQuantity
	}
	satuan = map[string]int32{}
	for _, a := range req.ActualSatuanItems {
		satuan[a.LaundryItemID] = a.ActualQuantity
	}
	return breakdown, satuan
}

func (h *Handler) CreateBypassRequest(c *gin.Context) {
	employeeID, err := apphelper.CurrentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if _, err := attendance.AssertShiftEligibility(c.Request.Context(), h.Queries, employeeID); err != nil {
		respondEligibilityError(c, err)
		return
	}

	role := apphelper.CurrentEmployeeRole(c)
	station, ok := stationForRole[role]
	if !ok {
		apperr.RespondError(c, http.StatusForbidden, "station_access_denied")
		return
	}

	payloadJSON := c.PostForm("payload")
	var req CreateBypassRequest
	if err := json.Unmarshal([]byte(payloadJSON), &req); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_payload")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(req.OrderID); err != nil {
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
	if ord.Status != station {
		apperr.RespondError(c, http.StatusUnprocessableEntity, "invalid_order_status")
		return
	}

	if _, err := h.Queries.GetPendingBypassRequest(c.Request.Context(), db.GetPendingBypassRequestParams{
		OrderID: orderID,
		Station: station,
	}); err == nil {
		apperr.RespondError(c, http.StatusConflict, "bypass_already_pending")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondInternalError(c, err)
		return
	}

	previousCount, err := h.Queries.CountNonPendingBypassRequests(c.Request.Context(), db.CountNonPendingBypassRequestsParams{
		OrderID: orderID,
		Station: station,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if previousCount >= MaxBypassAttemptsPerStation {
		apperr.RespondError(c, http.StatusBadRequest, "bypass_limit_reached")
		return
	}

	photoURLs := []string{}
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		fileHeaders := form.File["photos"]
		if len(fileHeaders) > maxBypassPhotos {
			apperr.RespondError(c, http.StatusBadRequest, "too_many_bypass_photos")
			return
		}
		for _, fh := range fileHeaders {
			if fh.Size > apphelper.MaxImageUploadSize {
				apperr.RespondError(c, http.StatusBadRequest, "bypass_photo_too_large")
				return
			}
			contentType := fh.Header.Get("Content-Type")
			if !apphelper.AllowedImageContentTypes[contentType] {
				apperr.RespondError(c, http.StatusBadRequest, "bypass_photo_invalid_type")
				return
			}
			file, err := fh.Open()
			if err != nil {
				apperr.RespondInternalError(c, err)
				return
			}
			url, err := h.StorageClient.UploadBypassPhoto(c.Request.Context(), file, req.OrderID)
			file.Close()
			if err != nil {
				apperr.RespondInternalError(c, err)
				return
			}
			photoURLs = append(photoURLs, url)
		}
	}
	req.PhotoEvidence = photoURLs

	expectedBreakdown, expectedSatuan, err := h.expectedItems(c.Request.Context(), orderID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	expectedNormalized, err := h.buildNormalizedItems(c.Request.Context(), expectedBreakdown, expectedSatuan)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	actualBreakdown, actualSatuan := actualItemMaps(req)
	actualNormalized, err := h.buildNormalizedItems(c.Request.Context(), actualBreakdown, actualSatuan)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	expectedJSON, err := json.Marshal(expectedNormalized)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	actualJSON, err := json.Marshal(actualNormalized)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	photoEvidence := req.PhotoEvidence
	if photoEvidence == nil {
		photoEvidence = []string{}
	}

	created, err := h.Queries.CreateBypassRequest(c.Request.Context(), db.CreateBypassRequestParams{
		OrderID:                orderID,
		Station:                station,
		RequestedBy:            employeeID,
		ExpectedItems:          expectedJSON,
		ActualItems:            actualJSON,
		DiscrepancyDescription: req.DiscrepancyDescription,
		PhotoEvidence:          photoEvidence,
		AttemptNumber:          int32(previousCount) + 1,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	sse.Default.Broadcast("outlet:"+ord.OutletID.String(), "bypass:created", gin.H{
		"bypassID": created.ID.String(),
		"orderID":  orderID.String(),
		"station":  station,
		"workerID": employeeID.String(),
	})
	sse.Default.Broadcast("user:"+employeeID.String(), "bypass:created", gin.H{
		"bypassID": created.ID.String(),
		"status":   "pending",
	})

	resp, err := h.toBypassResponse(c.Request.Context(), created)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	resp.Message = "bypass request submitted successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetBypassByOrder(c *gin.Context) {
	employeeID, err := apphelper.CurrentEmployeeID(c)
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

	rows, err := h.Queries.ListBypassRequestsByOrder(c.Request.Context(), orderID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]BypassResponse, 0, len(rows))
	for _, row := range rows {
		resp, err := h.toBypassResponse(c.Request.Context(), row)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		data = append(data, resp)
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// bypassListFilter resolves the caller-scoped filter for the two
// admin-facing list endpoints: outlet_admin is scoped to their own outlet,
// super_admin sees every outlet unfiltered.
func bypassListFilter(c *gin.Context) (outletID pgtype.UUID, scoped bool) {
	if apphelper.CurrentEmployeeRole(c) != "outlet_admin" {
		return pgtype.UUID{}, false
	}
	outletID, ok := apphelper.CurrentEmployeeOutletID(c)
	return outletID, ok
}

func (h *Handler) ListBypassRequests(c *gin.Context) {
	limit, offset := apphelper.ParsePagination(c, defaultPageLimit, maxPageLimit)

	outletID, scoped := bypassListFilter(c)
	status := pgtype.Text{Valid: false}
	if v := c.Query("status"); v != "" {
		status = pgtype.Text{String: v, Valid: true}
	}

	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	rows, err := h.Queries.ListBypassRequests(c.Request.Context(), db.ListBypassRequestsParams{
		OutletID: outletFilter,
		Status:   status,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountBypassRequests(c.Request.Context(), db.CountBypassRequestsParams{
		OutletID: outletFilter,
		Status:   status,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]BypassResponse, 0, len(rows))
	for _, row := range rows {
		resp, err := h.toBypassResponse(c.Request.Context(), row)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		data = append(data, resp)
	}

	c.JSON(http.StatusOK, BypassListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetBypassRequest(c *gin.Context) {
	var bypassID pgtype.UUID
	if err := bypassID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_bypass_id")
		return
	}

	bypass, err := h.Queries.GetBypassRequestByID(c.Request.Context(), bypassID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "bypass_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if outletID, scoped := bypassListFilter(c); scoped {
		ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), bypass.OrderID)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if ord.OutletID != outletID {
			apperr.RespondError(c, http.StatusNotFound, "bypass_not_found")
			return
		}
	}

	resp, err := h.toBypassResponse(c.Request.Context(), bypass)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ReviewBypassRequest(c *gin.Context) {
	adminID, err := apphelper.CurrentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	adminOutletID, hasOutlet := apphelper.CurrentEmployeeOutletID(c)
	if !hasOutlet {
		apperr.RespondError(c, http.StatusForbidden, "no_outlet_assigned")
		return
	}

	var bypassID pgtype.UUID
	if err := bypassID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_bypass_id")
		return
	}

	var req ReviewBypassRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bypass, err := h.Queries.GetBypassRequestByID(c.Request.Context(), bypassID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "bypass_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), bypass.OrderID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if ord.OutletID != adminOutletID {
		apperr.RespondError(c, http.StatusNotFound, "bypass_not_found")
		return
	}

	if !req.Approve {
		reviewed, err := h.Queries.ReviewBypassRequest(c.Request.Context(), db.ReviewBypassRequestParams{
			Status:     "rejected",
			ReviewedBy: adminID,
			AdminNotes: pgtype.Text{String: req.AdminNotes, Valid: req.AdminNotes != ""},
			ID:         bypassID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondError(c, http.StatusConflict, "bypass_already_reviewed")
			return
		}
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}

		sse.Default.Broadcast("user:"+reviewed.RequestedBy.String(), "bypass:rejected", gin.H{
			"bypassID":      reviewed.ID.String(),
			"orderID":       reviewed.OrderID.String(),
			"invoiceNumber": ord.InvoiceNumber,
			"adminNotes":    req.AdminNotes,
		})

		resp, err := h.toBypassResponse(c.Request.Context(), reviewed)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		resp.Message = "bypass request rejected"
		c.JSON(http.StatusOK, resp)
		return
	}

	// Approve path: the bypass approval and the station transition it
	// triggers (CompleteStationAfterBypass — same optimistic-concurrency
	// transition as a normal station completion, deliberately skipping
	// compareItems and the pending-bypass check, since this IS the manual
	// override those checks exist to gate) must commit atomically. If the
	// station transition loses the race (409 station_already_processed) or
	// fails for any other reason, the bypass approval must roll back with
	// it — otherwise the bypass is left permanently "approved" with no
	// corresponding station transition and no way to retry.
	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	reviewed, err := qtx.ReviewBypassRequest(c.Request.Context(), db.ReviewBypassRequestParams{
		Status:     "approved",
		ReviewedBy: adminID,
		AdminNotes: pgtype.Text{String: req.AdminNotes, Valid: req.AdminNotes != ""},
		ID:         bypassID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "bypass_already_reviewed")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	updated, nextStatus, paidDeliveryTask, err := h.completeStationTx(c.Request.Context(), qtx, adminID, reviewed.Station, reviewed.OrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusConflict, "station_already_processed")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	sse.Default.Broadcast("user:"+reviewed.RequestedBy.String(), "bypass:approved", gin.H{
		"bypassID":      reviewed.ID.String(),
		"orderID":       reviewed.OrderID.String(),
		"invoiceNumber": ord.InvoiceNumber,
		"adminNotes":    req.AdminNotes,
	})
	h.broadcastStationCompletion(c.Request.Context(), updated, adminID, reviewed.Station, nextStatus, paidDeliveryTask)

	resp := toOrderResponse(updated)
	resp.Message = "station completed successfully"
	c.JSON(http.StatusOK, SubmitItemsResponse{Success: true, Data: &resp})
}

func (h *Handler) toBypassResponse(ctx context.Context, b db.BypassRequest) (BypassResponse, error) {
	var expected, actual []NormalizedItem
	if err := json.Unmarshal(b.ExpectedItems, &expected); err != nil {
		return BypassResponse{}, err
	}
	if err := json.Unmarshal(b.ActualItems, &actual); err != nil {
		return BypassResponse{}, err
	}

	resp := BypassResponse{
		ID:                     b.ID.String(),
		OrderID:                b.OrderID.String(),
		Station:                b.Station,
		RequestedBy:            b.RequestedBy.String(),
		ExpectedItems:          expected,
		ActualItems:            actual,
		DiscrepancyDescription: b.DiscrepancyDescription,
		PhotoEvidence:          b.PhotoEvidence,
		AttemptNumber:          b.AttemptNumber,
		Status:                 b.Status,
		AdminNotes:             b.AdminNotes.String,
	}
	if b.ReviewedBy.Valid {
		resp.ReviewedBy = b.ReviewedBy.String()
	}

	if ord, err := h.Queries.GetOrderByIDAny(ctx, b.OrderID); err == nil {
		resp.InvoiceNumber = ord.InvoiceNumber
	}
	if emp, err := h.Queries.GetEmployeeByIDAny(ctx, b.RequestedBy); err == nil {
		resp.RequestedByName = emp.FullName
	}

	return resp, nil
}
