package clothingtype

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate active name).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
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

func toClothingTypeResponse(id pgtype.UUID, name string, isActive bool) ClothingTypeResponse {
	return ClothingTypeResponse{
		ID:       id.String(),
		Name:     name,
		IsActive: isActive,
	}
}
