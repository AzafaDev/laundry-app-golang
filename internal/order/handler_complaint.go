package order

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/sse"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CreateComplaint(c *gin.Context) {
	customerID, err := currentCustomerID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var orderID pgtype.UUID
	if err := orderID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_order_id")
		return
	}

	var req CreateComplaintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if ord.Status != StatusReceivedByCustomer {
		apperr.RespondError(c, http.StatusBadRequest, "order_not_received")
		return
	}

	if req.PhotoURLs == nil {
		req.PhotoURLs = []string{}
	}

	created, err := h.Queries.CreateComplaint(c.Request.Context(), db.CreateComplaintParams{
		OrderID:       orderID,
		CustomerID:    customerID,
		ComplaintType: req.ComplaintType,
		Description:   req.Description,
		PhotoUrls:     req.PhotoURLs,
	})
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "complaint_already_exists")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	sse.Default.Broadcast("outlet:"+ord.OutletID.String(), "order:complaint-submitted", gin.H{
		"orderID":       orderID.String(),
		"invoiceNumber": ord.InvoiceNumber,
		"complaintType": created.ComplaintType,
		"timestamp":     time.Now(),
	})

	resp := toComplaintResponse(created)
	resp.Message = "complaint submitted successfully"
	c.JSON(http.StatusCreated, resp)
}

func toComplaintResponse(cm db.Complaint) ComplaintResponse {
	return ComplaintResponse{
		ID:            cm.ID.String(),
		OrderID:       cm.OrderID.String(),
		ComplaintType: cm.ComplaintType,
		Description:   cm.Description,
		PhotoURLs:     cm.PhotoUrls,
		Status:        cm.Status,
	}
}
