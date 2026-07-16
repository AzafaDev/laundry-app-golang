package laundryitem

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate active name).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
	defaultUnit      = "pcs"
	maxBasePrice     = 99_999_999.99
)

var validUnits = map[string]bool{
	"pcs": true,
	"kg":  true,
}

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

func float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func toLaundryItemResponse(id pgtype.UUID, name string, description pgtype.Text, unit string, basePrice pgtype.Numeric, isActive bool) LaundryItemResponse {
	return LaundryItemResponse{
		ID:          id.String(),
		Name:        name,
		Description: description.String,
		Unit:        unit,
		BasePrice:   numericToFloat64(basePrice),
		IsActive:    isActive,
	}
}

func toPublicLaundryItemResponse(id pgtype.UUID, name string, description pgtype.Text, unit string, basePrice pgtype.Numeric) PublicLaundryItemResponse {
	return PublicLaundryItemResponse{
		ID:          id.String(),
		Name:        name,
		Description: description.String,
		Unit:        unit,
		BasePrice:   numericToFloat64(basePrice),
	}
}
