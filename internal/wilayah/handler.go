// Package wilayah exposes read-only Indonesia province/city/district
// reference data. Public (no auth) — not customer-scoped, so it lives
// outside internal/customer and its routes stay off /api/v1/customer/.
package wilayah

import (
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{Queries: queries}
}

func (h *Handler) ListProvinces(c *gin.Context) {
	provinces, err := h.Queries.ListProvinces(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provinces)
}

func (h *Handler) ListCitiesByProvince(c *gin.Context) {
	provinceID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_province_id")
		return
	}

	cities, err := h.Queries.ListCitiesByProvince(c.Request.Context(), int32(provinceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cities)
}

func (h *Handler) ListDistrictsByCity(c *gin.Context) {
	cityID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_city_id")
		return
	}

	districts, err := h.Queries.ListDistrictsByCity(c.Request.Context(), int32(cityID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, districts)
}
