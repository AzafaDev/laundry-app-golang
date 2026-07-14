package outlet

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// parsePagination reads limit/offset query params, defaulting to 50/0 and
// clamping limit to 100, floor-ing invalid or negative values to their
// defaults.
func parsePagination(c *gin.Context) (limit, offset int32) {
	limit = defaultPageLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

// float64ToNumeric converts a float64 into a pgtype.Numeric. pgx only
// accepts a string representation when scanning into Numeric, so we format
// the float ourselves rather than relying on a direct Scan(float64).
func float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func toOutletResponse(id pgtype.UUID, name, address string, latitude, longitude pgtype.Numeric, isActive bool) OutletResponse {
	return OutletResponse{
		ID:        id.String(),
		Name:      name,
		Address:   address,
		Latitude:  numericToFloat64(latitude),
		Longitude: numericToFloat64(longitude),
		IsActive:  isActive,
	}
}
