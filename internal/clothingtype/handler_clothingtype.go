package clothingtype

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) ListClothingTypes(c *gin.Context) {
	limit, offset := apphelper.ParsePagination(c, defaultPageLimit, maxPageLimit)

	types, err := h.Queries.ListClothingTypes(c.Request.Context(), db.ListClothingTypesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountClothingTypes(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]ClothingTypeResponse, 0, len(types))
	for _, t := range types {
		data = append(data, toClothingTypeResponse(t.ID, t.Name, t.IsActive))
	}

	c.JSON(http.StatusOK, ClothingTypeListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetClothingTypeByID(c *gin.Context) {
	var typeID pgtype.UUID
	if err := typeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_clothing_type_id")
		return
	}

	t, err := h.Queries.GetClothingTypeByID(c.Request.Context(), typeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "clothing_type_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toClothingTypeResponse(t.ID, t.Name, t.IsActive))
}

func (h *Handler) CreateClothingType(c *gin.Context) {
	var req ClothingTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.Queries.CreateClothingType(c.Request.Context(), db.CreateClothingTypeParams{
		Name:     req.Name,
		IsActive: req.IsActive,
	})
	if apphelper.IsUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toClothingTypeResponse(created.ID, created.Name, created.IsActive)
	resp.Message = "clothing type created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateClothingType(c *gin.Context) {
	var typeID pgtype.UUID
	if err := typeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_clothing_type_id")
		return
	}

	var req ClothingTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.Queries.UpdateClothingType(c.Request.Context(), db.UpdateClothingTypeParams{
		Name:     req.Name,
		IsActive: req.IsActive,
		ID:       typeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "clothing_type_not_found")
		return
	}
	if apphelper.IsUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toClothingTypeResponse(updated.ID, updated.Name, updated.IsActive)
	resp.Message = "clothing type updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SoftDeleteClothingType(c *gin.Context) {
	var typeID pgtype.UUID
	if err := typeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_clothing_type_id")
		return
	}

	if _, err := h.Queries.GetClothingTypeByID(c.Request.Context(), typeID); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "clothing_type_not_found")
		return
	} else if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := h.Queries.SoftDeleteClothingType(c.Request.Context(), typeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "clothing type deleted successfully"})
}
