package outlet

import (
	"errors"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) ListOutlets(c *gin.Context) {
	outlets, err := h.Queries.ListOutlets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]OutletResponse, 0, len(outlets))
	for _, o := range outlets {
		responses = append(responses, toOutletResponse(o.ID, o.Name, o.Address, o.Latitude, o.Longitude, o.IsActive))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) GetOutletByID(c *gin.Context) {
	var outletID pgtype.UUID
	if err := outletID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outlet id"})
		return
	}

	outlet, err := h.Queries.GetOutletByID(c.Request.Context(), outletID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "outlet not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toOutletResponse(outlet.ID, outlet.Name, outlet.Address, outlet.Latitude, outlet.Longitude, outlet.IsActive))
}

func (h *Handler) CreateOutlet(c *gin.Context) {
	var req OutletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	latitude, err := float64ToNumeric(req.Latitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	longitude, err := float64ToNumeric(req.Longitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.Queries.CreateOutlet(c.Request.Context(), db.CreateOutletParams{
		Name:      req.Name,
		Address:   req.Address,
		Latitude:  latitude,
		Longitude: longitude,
		IsActive:  req.IsActive,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toOutletResponse(created.ID, created.Name, created.Address, created.Latitude, created.Longitude, created.IsActive)
	resp.Message = "outlet created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateOutlet(c *gin.Context) {
	var outletID pgtype.UUID
	if err := outletID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outlet id"})
		return
	}

	var req OutletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	latitude, err := float64ToNumeric(req.Latitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	longitude, err := float64ToNumeric(req.Longitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.Queries.UpdateOutlet(c.Request.Context(), db.UpdateOutletParams{
		Name:      req.Name,
		Address:   req.Address,
		Latitude:  latitude,
		Longitude: longitude,
		IsActive:  req.IsActive,
		ID:        outletID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "outlet not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toOutletResponse(updated.ID, updated.Name, updated.Address, updated.Latitude, updated.Longitude, updated.IsActive)
	resp.Message = "outlet updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SoftDeleteOutlet(c *gin.Context) {
	var outletID pgtype.UUID
	if err := outletID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outlet id"})
		return
	}

	if _, err := h.Queries.GetOutletByID(c.Request.Context(), outletID); errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "outlet not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.SoftDeleteOutlet(c.Request.Context(), outletID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "outlet deleted successfully"})
}
