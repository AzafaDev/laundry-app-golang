package notification

import (
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func toResponse(id pgtype.UUID, title, body, notifType string, relatedEntityID pgtype.UUID, isRead bool, createdAt pgtype.Timestamptz) NotificationResponse {
	resp := NotificationResponse{
		ID:        id.String(),
		Title:     title,
		Body:      body,
		Type:      notifType,
		IsRead:    isRead,
		CreatedAt: createdAt.Time.Format(time.RFC3339),
	}
	if relatedEntityID.Valid {
		resp.RelatedEntityID = relatedEntityID.String()
	}
	return resp
}

func (h *Handler) ListCustomerNotifications(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	limit, offset := parsePagination(c)

	rows, err := h.Queries.ListCustomerNotifications(c.Request.Context(), db.ListCustomerNotificationsParams{
		CustomerID: customerID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountCustomerNotifications(c.Request.Context(), customerID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]NotificationResponse, 0, len(rows))
	for _, n := range rows {
		data = append(data, toResponse(n.ID, n.Title, n.Body, n.Type, n.RelatedEntityID, n.IsRead, n.CreatedAt))
	}

	c.JSON(http.StatusOK, NotificationListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetCustomerUnreadCount(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	count, err := h.Queries.CountUnreadCustomerNotifications(c.Request.Context(), customerID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, UnreadCountResponse{UnreadCount: count})
}

func (h *Handler) MarkCustomerNotificationRead(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var notificationID pgtype.UUID
	if err := notificationID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_notification_id")
		return
	}

	if err := h.Queries.MarkCustomerNotificationRead(c.Request.Context(), db.MarkCustomerNotificationReadParams{
		ID:         notificationID,
		CustomerID: customerID,
	}); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification marked as read"})
}

func (h *Handler) MarkAllCustomerNotificationsRead(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if err := h.Queries.MarkAllCustomerNotificationsRead(c.Request.Context(), customerID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "all notifications marked as read"})
}

func (h *Handler) ListEmployeeNotifications(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	limit, offset := parsePagination(c)

	rows, err := h.Queries.ListEmployeeNotifications(c.Request.Context(), db.ListEmployeeNotificationsParams{
		EmployeeID: employeeID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountEmployeeNotifications(c.Request.Context(), employeeID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]NotificationResponse, 0, len(rows))
	for _, n := range rows {
		data = append(data, toResponse(n.ID, n.Title, n.Body, n.Type, n.RelatedEntityID, n.IsRead, n.CreatedAt))
	}

	c.JSON(http.StatusOK, NotificationListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetEmployeeUnreadCount(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	count, err := h.Queries.CountUnreadEmployeeNotifications(c.Request.Context(), employeeID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, UnreadCountResponse{UnreadCount: count})
}

func (h *Handler) MarkEmployeeNotificationRead(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var notificationID pgtype.UUID
	if err := notificationID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_notification_id")
		return
	}

	if err := h.Queries.MarkEmployeeNotificationRead(c.Request.Context(), db.MarkEmployeeNotificationReadParams{
		ID:         notificationID,
		EmployeeID: employeeID,
	}); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification marked as read"})
}

func (h *Handler) MarkAllEmployeeNotificationsRead(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	if err := h.Queries.MarkAllEmployeeNotificationsRead(c.Request.Context(), employeeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "all notifications marked as read"})
}
