package report

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func currentEmployeeRole(c *gin.Context) string {
	val, _ := c.Get("role")
	role, _ := val.(string)
	return role
}

func currentEmployeeOutletID(c *gin.Context) (outletID pgtype.UUID, ok bool) {
	val, exists := c.Get("outlet_id")
	if !exists {
		return outletID, false
	}
	outletID, ok = val.(pgtype.UUID)
	return outletID, ok && outletID.Valid
}

// reportOutletFilter mirrors complaintListFilter (internal/order, ticket
// #7): outlet_admin is forced to their own outlet; super_admin is unscoped
// unless they explicitly pass ?outlet_id=.
func reportOutletFilter(c *gin.Context) (outletID pgtype.UUID, scoped bool) {
	if currentEmployeeRole(c) == "outlet_admin" {
		outletID, ok := currentEmployeeOutletID(c)
		return outletID, ok
	}
	if v := c.Query("outlet_id"); v != "" {
		var id pgtype.UUID
		if err := id.Scan(v); err == nil {
			return id, true
		}
	}
	return pgtype.UUID{}, false
}

func dateRangeFilter(c *gin.Context) (dateFrom, dateTo pgtype.Timestamptz) {
	if v := c.Query("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			dateFrom = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := c.Query("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			dateTo = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Nanosecond), Valid: true}
		}
	}
	return dateFrom, dateTo
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func formatMoney(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
