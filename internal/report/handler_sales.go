package report

import (
	"context"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

var validGroupBy = map[string]bool{
	"day":   true,
	"month": true,
	"year":  true,
}

func groupByPeriodFormat(groupBy string) string {
	switch groupBy {
	case "day":
		return "2006-01-02"
	case "year":
		return "2006"
	default:
		return "2006-01"
	}
}

// fetchSalesReport is the single source of both GetSalesReport (JSON) and
// ExportSalesReport (CSV) — one query set, two output formatters, not a
// duplicated fetch like the TS source's export controller.
func (h *Handler) fetchSalesReport(ctx context.Context, groupBy string, outletID pgtype.UUID, scoped bool, dateFrom, dateTo pgtype.Timestamptz) (SalesReportResponse, error) {
	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	rows, err := h.Queries.SalesReportByPeriod(ctx, db.SalesReportByPeriodParams{
		GroupBy:  groupBy,
		OutletID: outletFilter,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return SalesReportResponse{}, err
	}

	summary, err := h.Queries.SalesReportSummary(ctx, db.SalesReportSummaryParams{
		OutletID: outletFilter,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return SalesReportResponse{}, err
	}

	format := groupByPeriodFormat(groupBy)
	chart := make([]SalesReportPeriod, 0, len(rows))
	for _, r := range rows {
		income := numericToFloat64(r.Income)
		var avg float64
		if r.OrderCount > 0 {
			avg = income / float64(r.OrderCount)
		}
		chart = append(chart, SalesReportPeriod{
			Period:          r.Period.Time.Format(format),
			Income:          income,
			OrderCount:      r.OrderCount,
			AveragePerOrder: avg,
		})
	}

	totalIncome := numericToFloat64(summary.TotalIncome)
	resp := SalesReportResponse{
		Summary: SalesReportSummary{
			TotalIncome: totalIncome,
			TotalOrders: summary.TotalOrders,
			PeriodCount: len(chart),
		},
		Chart: chart,
	}
	if summary.TotalOrders > 0 {
		resp.Summary.AveragePerOrder = totalIncome / float64(summary.TotalOrders)
	}
	if len(chart) > 0 {
		resp.Summary.AveragePerPeriod = totalIncome / float64(len(chart))
	}

	return resp, nil
}

func (h *Handler) GetSalesReport(c *gin.Context) {
	groupBy := c.DefaultQuery("group_by", "month")
	if !validGroupBy[groupBy] {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_group_by")
		return
	}

	outletID, scoped := reportOutletFilter(c)
	dateFrom, dateTo := dateRangeFilter(c)

	resp, err := h.fetchSalesReport(c.Request.Context(), groupBy, outletID, scoped, dateFrom, dateTo)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ExportSalesReport(c *gin.Context) {
	groupBy := c.DefaultQuery("group_by", "month")
	if !validGroupBy[groupBy] {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_group_by")
		return
	}

	outletID, scoped := reportOutletFilter(c)
	dateFrom, dateTo := dateRangeFilter(c)

	resp, err := h.fetchSalesReport(c.Request.Context(), groupBy, outletID, scoped, dateFrom, dateTo)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	header := []string{"period", "income", "order_count", "average_per_order"}
	rows := make([][]string, 0, len(resp.Chart)+1)
	for _, p := range resp.Chart {
		rows = append(rows, []string{
			p.Period,
			formatMoney(p.Income),
			fmt.Sprintf("%d", p.OrderCount),
			formatMoney(p.AveragePerOrder),
		})
	}
	rows = append(rows, []string{
		"TOTAL",
		formatMoney(resp.Summary.TotalIncome),
		fmt.Sprintf("%d", resp.Summary.TotalOrders),
		formatMoney(resp.Summary.AveragePerOrder),
	})

	filename := fmt.Sprintf("sales_report_%s.csv", time.Now().Format("2006-01-02"))
	writeCSV(c, filename, header, rows, true)
}
