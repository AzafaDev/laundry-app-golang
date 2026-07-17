package report

import (
	"laundry-app-with-golang/internal/apphelper"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// reportOutletFilter mirrors complaintListFilter (internal/order, ticket
// #7): outlet_admin is forced to their own outlet; super_admin is unscoped
// unless they explicitly pass ?outlet_id=.
func reportOutletFilter(c *gin.Context) (outletID pgtype.UUID, scoped bool) {
	if apphelper.CurrentEmployeeRole(c) == "outlet_admin" {
		outletID, ok := apphelper.CurrentEmployeeOutletID(c)
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

func formatMoney(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
