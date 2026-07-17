package apphelper

import (
	"encoding/csv"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNoSession = errors.New("something went wrong")

// IsUniqueViolation reports whether err is a Postgres unique-constraint
// violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// ParsePagination keeps each caller's own default/max limits — those values
// differ per endpoint (10/20/50 across the codebase) and must not be
// collapsed into one shared constant.
func ParsePagination(c *gin.Context, defaultLimit, maxLimit int32) (limit, offset int32) {
	limit = defaultLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxLimit {
			limit = maxLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

func CurrentEmployeeID(c *gin.Context) (pgtype.UUID, error) {
	var id pgtype.UUID
	val, ok := c.Get("employee_id")
	if !ok {
		return id, ErrNoSession
	}
	str, ok := val.(string)
	if !ok {
		return id, ErrNoSession
	}
	if err := id.Scan(str); err != nil {
		return id, err
	}
	return id, nil
}

func CurrentCustomerID(c *gin.Context) (pgtype.UUID, error) {
	var id pgtype.UUID
	val, ok := c.Get("customer_id")
	if !ok {
		return id, ErrNoSession
	}
	str, ok := val.(string)
	if !ok {
		return id, ErrNoSession
	}
	if err := id.Scan(str); err != nil {
		return id, err
	}
	return id, nil
}

func CurrentEmployeeRole(c *gin.Context) string {
	val, _ := c.Get("role")
	role, _ := val.(string)
	return role
}

func CurrentEmployeeOutletID(c *gin.Context) (outletID pgtype.UUID, ok bool) {
	val, exists := c.Get("outlet_id")
	if !exists {
		return outletID, false
	}
	outletID, ok = val.(pgtype.UUID)
	return outletID, ok && outletID.Valid
}

func NumericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func Float64ToNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n, err
}

// WriteCSV streams a CSV response. contentType lets callers preserve their
// existing header exactly (report: "text/csv; charset=utf-8", attendance:
// "text/csv") — ticket #8 made that distinction on purpose, so it is a
// parameter here rather than collapsed to one literal.
func WriteCSV(c *gin.Context, contentType, filename string, header []string, rows [][]string, withBOM bool) {
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	if withBOM {
		_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))
	}
	w := csv.NewWriter(c.Writer)
	_ = w.Write(header)
	_ = w.WriteAll(rows)
	w.Flush()
}
