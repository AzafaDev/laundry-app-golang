package order

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/sse"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CreateComplaint(c *gin.Context) {
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

	complaintType := strings.TrimSpace(c.PostForm("complaint_type"))
	description := strings.TrimSpace(c.PostForm("description"))
	if complaintType == "" || description == "" {
		apperr.RespondError(c, http.StatusBadRequest, "complaint_type_and_description_required")
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

	photoURLs := []string{}
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		fileHeaders := form.File["photos"]
		if len(fileHeaders) > maxComplaintPhotos {
			apperr.RespondError(c, http.StatusBadRequest, "too_many_complaint_photos")
			return
		}

		for _, fileHeader := range fileHeaders {
			if fileHeader.Size > apphelper.MaxImageUploadSize {
				apperr.RespondError(c, http.StatusBadRequest, "complaint_photo_too_large")
				return
			}

			contentType := fileHeader.Header.Get("Content-Type")
			if !apphelper.AllowedImageContentTypes[contentType] {
				apperr.RespondError(c, http.StatusBadRequest, "complaint_photo_invalid_type")
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				apperr.RespondInternalError(c, err)
				return
			}

			photoURL, err := h.StorageClient.UploadComplaintPhoto(c.Request.Context(), file, c.Param("id"))
			file.Close()
			if err != nil {
				apperr.RespondInternalError(c, err)
				return
			}

			photoURLs = append(photoURLs, photoURL)
		}
	}

	created, err := h.Queries.CreateComplaint(c.Request.Context(), db.CreateComplaintParams{
		OrderID:       orderID,
		CustomerID:    customerID,
		ComplaintType: complaintType,
		Description:   description,
		PhotoUrls:     photoURLs,
	})
	if apphelper.IsUniqueViolation(err) {
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
