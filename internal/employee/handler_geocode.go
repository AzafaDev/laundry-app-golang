package employee

import (
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/geocode"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const defaultGeocodeSearchLimit = 5

func (h *Handler) SearchGeocode(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		apperr.RespondError(c, http.StatusBadRequest, "query_required")
		return
	}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = defaultGeocodeSearchLimit
	}

	results, err := h.geocodeClient.Search(c.Request.Context(), q, limit)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	if results == nil {
		results = []geocode.Result{}
	}

	c.JSON(http.StatusOK, results)
}
