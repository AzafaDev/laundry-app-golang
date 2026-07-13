package customer

import (
	"errors"
	"laundry-app-with-golang/internal/geocode"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const defaultGeocodeSearchLimit = 5

func (h *Handler) Geocode(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	result, err := h.geocodeClient.Geocode(c.Request.Context(), q)
	if errors.Is(err, geocode.ErrNoResults) {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) SearchGeocode(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = defaultGeocodeSearchLimit
	}

	results, err := h.geocodeClient.Search(c.Request.Context(), q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []geocode.Result{}
	}

	c.JSON(http.StatusOK, results)
}
