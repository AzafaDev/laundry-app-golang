package order

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/sse"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// validComplaintTransitions mirrors the TS state machine exactly: resolved
// and rejected are terminal (no map entry, so any transition out of them is
// rejected).
var validComplaintTransitions = map[string][]string{
	"open":        {"in_progress", "rejected"},
	"in_progress": {"resolved", "rejected"},
}

// complaintListFilter resolves the caller-scoped outlet filter for admin
// complaint endpoints: outlet_admin is scoped to their own outlet;
// super_admin is unscoped unless they explicitly pass ?outlet_id=.
func complaintListFilter(c *gin.Context) (outletID pgtype.UUID, scoped bool) {
	if apphelper.CurrentEmployeeRole(c) == "outlet_admin" {
		outletID, ok := apphelper.CurrentEmployeeOutletID(c)
		return outletID, ok
	}
	if v := c.Query("outlet_id"); v != "" {
		var id pgtype.UUID
		if err := id.Scan(v); err == nil {
			return id, true
		}
	}
	return pgtype.UUID{}, false
}

func (h *Handler) ListComplaints(c *gin.Context) {
	limit, offset := apphelper.ParsePagination(c, defaultPageLimit, maxPageLimit)

	outletID, scoped := complaintListFilter(c)
	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	status := pgtype.Text{Valid: false}
	if v := c.Query("status"); v != "" {
		status = pgtype.Text{String: v, Valid: true}
	}

	search := pgtype.Text{Valid: false}
	if v := c.Query("search"); v != "" {
		search = pgtype.Text{String: v, Valid: true}
	}

	rows, err := h.Queries.ListComplaintsForAdmin(c.Request.Context(), db.ListComplaintsForAdminParams{
		OutletID: outletFilter,
		Status:   status,
		Search:   search,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountComplaints(c.Request.Context(), db.CountComplaintsParams{
		OutletID: outletFilter,
		Status:   status,
		Search:   search,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]AdminComplaintResponse, 0, len(rows))
	for _, cm := range rows {
		data = append(data, toAdminComplaintResponseFromList(cm))
	}

	c.JSON(http.StatusOK, AdminComplaintListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetComplaintStats(c *gin.Context) {
	outletID, scoped := complaintListFilter(c)
	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	rows, err := h.Queries.CountComplaintsByStatus(c.Request.Context(), outletFilter)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	stats := ComplaintStatsResponse{}
	for _, row := range rows {
		switch row.Status {
		case "open":
			stats.Open = row.Total
		case "in_progress":
			stats.InProgress = row.Total
		case "resolved":
			stats.Resolved = row.Total
		case "rejected":
			stats.Rejected = row.Total
		}
	}

	c.JSON(http.StatusOK, stats)
}

// complaintOutletMatch fetches the complaint's order to check outlet
// scoping. Returns 404 (not 403) on mismatch — consistent with
// GetBypassRequest's pattern of not leaking resource existence to other
// outlets.
func (h *Handler) complaintOutletMatch(c *gin.Context, cm db.Complaint) bool {
	outletID, scoped := complaintListFilter(c)
	if !scoped {
		return true
	}
	ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), cm.OrderID)
	if err != nil {
		return false
	}
	return ord.OutletID == outletID
}

func (h *Handler) complaintOutletMatchForAdmin(c *gin.Context, orderID pgtype.UUID) bool {
	outletID, scoped := complaintListFilter(c)
	if !scoped {
		return true
	}
	ord, err := h.Queries.GetOrderByIDAny(c.Request.Context(), orderID)
	if err != nil {
		return false
	}
	return ord.OutletID == outletID
}

func (h *Handler) GetComplaintByID(c *gin.Context) {
	var complaintID pgtype.UUID
	if err := complaintID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_complaint_id")
		return
	}

	cm, err := h.Queries.GetComplaintByIDForAdmin(c.Request.Context(), complaintID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "complaint_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if !h.complaintOutletMatchForAdmin(c, cm.OrderID) {
		apperr.RespondError(c, http.StatusNotFound, "complaint_not_found")
		return
	}

	c.JSON(http.StatusOK, toAdminComplaintResponse(cm))
}

func (h *Handler) UpdateComplaintStatus(c *gin.Context) {
	employeeID, err := apphelper.CurrentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var complaintID pgtype.UUID
	if err := complaintID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_complaint_id")
		return
	}

	var req UpdateComplaintStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cm, err := h.Queries.GetComplaintByID(c.Request.Context(), complaintID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "complaint_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if !h.complaintOutletMatch(c, cm) {
		apperr.RespondError(c, http.StatusNotFound, "complaint_not_found")
		return
	}

	allowed := validComplaintTransitions[cm.Status]
	validTransition := false
	for _, s := range allowed {
		if s == req.Status {
			validTransition = true
			break
		}
	}
	if !validTransition {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_status_transition")
		return
	}

	resolutionNotes := pgtype.Text{Valid: false}
	if req.ResolutionNotes != "" {
		resolutionNotes = pgtype.Text{String: req.ResolutionNotes, Valid: true}
	}

	expectedResolutionDate := pgtype.Date{Valid: false}
	if req.ExpectedResolutionDate != "" {
		parsed, err := time.Parse("2006-01-02", req.ExpectedResolutionDate)
		if err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_expected_resolution_date")
			return
		}
		expectedResolutionDate = pgtype.Date{Time: parsed, Valid: true}
	}

	resolvedBy := pgtype.UUID{Valid: false}
	resolvedAt := pgtype.Timestamptz{Valid: false}
	if req.Status == "resolved" || req.Status == "rejected" {
		resolvedBy = employeeID
		resolvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	title, body := complaintStatusNotificationText(req.Status)

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	updated, err := qtx.UpdateComplaintStatus(c.Request.Context(), db.UpdateComplaintStatusParams{
		Status:                 req.Status,
		ResolutionNotes:        resolutionNotes,
		ExpectedResolutionDate: expectedResolutionDate,
		ResolvedBy:             resolvedBy,
		ResolvedAt:             resolvedAt,
		ID:                     complaintID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := notification.NotifyCustomer(c.Request.Context(), qtx, updated.CustomerID, title, body, notification.TypeComplaintUpdate, updated.OrderID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	sse.Default.Broadcast("user:"+updated.CustomerID.String(), "complaint:updated", gin.H{
		"complaintID": updated.ID.String(),
		"orderID":     updated.OrderID.String(),
		"status":      updated.Status,
	})

	// Fetch full admin details for response
	adminComplaint, err := h.Queries.GetComplaintByIDForAdmin(c.Request.Context(), complaintID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toAdminComplaintResponse(adminComplaint)
	resp.Message = "complaint status updated successfully"
	c.JSON(http.StatusOK, resp)
}

func complaintStatusNotificationText(status string) (title, body string) {
	switch status {
	case "in_progress":
		return "Komplain sedang diproses", "Komplain Anda sedang kami tindak lanjuti."
	case "resolved":
		return "Komplain telah diselesaikan", "Komplain Anda telah diselesaikan. Terima kasih atas kesabaran Anda."
	case "rejected":
		return "Komplain ditolak", "Mohon maaf, komplain Anda tidak dapat kami proses lebih lanjut."
	default:
		return "Status komplain diperbarui", "Status komplain Anda telah diperbarui."
	}
}

func toAdminComplaintResponse(cm db.GetComplaintByIDForAdminRow) AdminComplaintResponse {
	resp := AdminComplaintResponse{
		ID:            cm.ID.String(),
		OrderID:       cm.OrderID.String(),
		InvoiceNumber: cm.InvoiceNumber,
		CustomerID:    cm.CustomerID.String(),
		CustomerName:  cm.CustomerName,
		CustomerPhone: cm.CustomerPhone.String,
		ComplaintType: cm.ComplaintType,
		Description:   cm.Description,
		PhotoURLs:     cm.PhotoUrls,
		Status:        cm.Status,
		CreatedAt:     cm.CreatedAt.Time.Format(time.RFC3339),
	}

	if cm.ExpectedResolutionDate.Valid {
		resp.ExpectedResolutionDate = cm.ExpectedResolutionDate.Time.Format("2006-01-02")
	}

	if cm.ResolutionNotes.Valid {
		resp.ResolutionNotes = cm.ResolutionNotes.String
	}

	if cm.ResolvedBy.Valid {
		resp.ResolvedBy = cm.ResolvedBy.String()
	}

	if cm.ResolvedByName.Valid {
		resp.ResolvedByName = cm.ResolvedByName.String
	}

	if cm.ResolvedAt.Valid {
		resp.ResolvedAt = cm.ResolvedAt.Time.Format(time.RFC3339)
	}

	return resp
}

func toAdminComplaintResponseFromList(cm db.ListComplaintsForAdminRow) AdminComplaintResponse {
	resp := AdminComplaintResponse{
		ID:            cm.ID.String(),
		OrderID:       cm.OrderID.String(),
		InvoiceNumber: cm.InvoiceNumber,
		CustomerID:    cm.CustomerID.String(),
		CustomerName:  cm.CustomerName,
		CustomerPhone: cm.CustomerPhone.String,
		ComplaintType: cm.ComplaintType,
		Description:   cm.Description,
		PhotoURLs:     cm.PhotoUrls,
		Status:        cm.Status,
		CreatedAt:     cm.CreatedAt.Time.Format(time.RFC3339),
	}

	if cm.ExpectedResolutionDate.Valid {
		resp.ExpectedResolutionDate = cm.ExpectedResolutionDate.Time.Format("2006-01-02")
	}

	if cm.ResolutionNotes.Valid {
		resp.ResolutionNotes = cm.ResolutionNotes.String
	}

	if cm.ResolvedBy.Valid {
		resp.ResolvedBy = cm.ResolvedBy.String()
	}

	if cm.ResolvedByName.Valid {
		resp.ResolvedByName = cm.ResolvedByName.String
	}

	if cm.ResolvedAt.Valid {
		resp.ResolvedAt = cm.ResolvedAt.Time.Format(time.RFC3339)
	}

	return resp
}

func (h *Handler) GetDashboardStats(c *gin.Context) {
	outletID, scoped := complaintListFilter(c)
	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	orderStats, err := h.Queries.CountDashboardOrderStats(c.Request.Context(), outletFilter)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	openComplaints, err := h.Queries.CountOpenComplaints(c.Request.Context(), outletFilter)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	bypassPending, err := h.Queries.CountPendingBypassRequests(c.Request.Context(), outletFilter)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := DashboardStatsResponse{
		NeedsProcessing: orderStats.NeedsProcessing,
		AwaitingPayment: orderStats.AwaitingPayment,
		ComplaintsOpen:  openComplaints,
		BypassPending:   bypassPending,
	}

	c.JSON(http.StatusOK, resp)
}
