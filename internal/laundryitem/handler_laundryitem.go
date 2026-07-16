package laundryitem

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// validateRequest applies defaults and business validation shared by create
// and update: base_price must be positive and within Decimal(10,2) range,
// unit must be one of the fixed set consumed downstream by driver-worker
// filtering (unit == "kg" vs unit != "kg").
func validateRequest(req *LaundryItemRequest) string {
	if req.Unit == "" {
		req.Unit = defaultUnit
	}
	if !validUnits[req.Unit] {
		return "invalid_unit"
	}
	if req.BasePrice <= 0 {
		return "invalid_base_price"
	}
	if req.BasePrice > maxBasePrice {
		return "invalid_base_price"
	}
	return ""
}

func (h *Handler) ListLaundryItems(c *gin.Context) {
	limit, offset := parsePagination(c)

	items, err := h.Queries.ListLaundryItems(c.Request.Context(), db.ListLaundryItemsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalCount, err := h.Queries.CountLaundryItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]LaundryItemResponse, 0, len(items))
	for _, item := range items {
		data = append(data, toLaundryItemResponse(item.ID, item.Name, item.Description, item.Unit, item.BasePrice, item.IsActive))
	}

	c.JSON(http.StatusOK, LaundryItemListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetLaundryItemByID(c *gin.Context) {
	var itemID pgtype.UUID
	if err := itemID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_laundry_item_id")
		return
	}

	item, err := h.Queries.GetLaundryItemByID(c.Request.Context(), itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "laundry_item_not_found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toLaundryItemResponse(item.ID, item.Name, item.Description, item.Unit, item.BasePrice, item.IsActive))
}

func (h *Handler) CreateLaundryItem(c *gin.Context) {
	var req LaundryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if code := validateRequest(&req); code != "" {
		apperr.RespondError(c, http.StatusBadRequest, code)
		return
	}

	basePrice, err := float64ToNumeric(req.BasePrice)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.Queries.CreateLaundryItem(c.Request.Context(), db.CreateLaundryItemParams{
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Unit:        req.Unit,
		BasePrice:   basePrice,
		IsActive:    req.IsActive,
	})
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toLaundryItemResponse(created.ID, created.Name, created.Description, created.Unit, created.BasePrice, created.IsActive)
	resp.Message = "laundry item created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateLaundryItem(c *gin.Context) {
	var itemID pgtype.UUID
	if err := itemID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_laundry_item_id")
		return
	}

	var req LaundryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if code := validateRequest(&req); code != "" {
		apperr.RespondError(c, http.StatusBadRequest, code)
		return
	}

	basePrice, err := float64ToNumeric(req.BasePrice)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.Queries.UpdateLaundryItem(c.Request.Context(), db.UpdateLaundryItemParams{
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Unit:        req.Unit,
		BasePrice:   basePrice,
		IsActive:    req.IsActive,
		ID:          itemID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "laundry_item_not_found")
		return
	}
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toLaundryItemResponse(updated.ID, updated.Name, updated.Description, updated.Unit, updated.BasePrice, updated.IsActive)
	resp.Message = "laundry item updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SoftDeleteLaundryItem(c *gin.Context) {
	var itemID pgtype.UUID
	if err := itemID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_laundry_item_id")
		return
	}

	if _, err := h.Queries.GetLaundryItemByID(c.Request.Context(), itemID); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "laundry_item_not_found")
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.SoftDeleteLaundryItem(c.Request.Context(), itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "laundry item deleted successfully"})
}

func (h *Handler) HardDeleteLaundryItem(c *gin.Context) {
	var itemID pgtype.UUID
	if err := itemID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_laundry_item_id")
		return
	}

	existing, err := h.Queries.GetLaundryItemByIDAny(c.Request.Context(), itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "laundry_item_not_found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !existing.DeletedAt.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "laundry_item_must_be_deleted_first")
		return
	}

	if err := h.Queries.HardDeleteLaundryItem(c.Request.Context(), itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "laundry item permanently deleted"})
}

// ListPublicLaundryItems is the unauthenticated customer-facing endpoint —
// active, non-deleted items only, with a slimmer response shape.
func (h *Handler) ListPublicLaundryItems(c *gin.Context) {
	items, err := h.Queries.ListActiveLaundryItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]PublicLaundryItemResponse, 0, len(items))
	for _, item := range items {
		data = append(data, toPublicLaundryItemResponse(item.ID, item.Name, item.Description, item.Unit, item.BasePrice))
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}
