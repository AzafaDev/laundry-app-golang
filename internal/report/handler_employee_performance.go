package report

import (
	"context"
	"fmt"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/apphelper"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// fetchEmployeePerformance is the single source of both
// GetEmployeePerformanceReport (JSON) and ExportEmployeePerformanceReport
// (CSV). Worker and driver job counts come from two separate GROUP BY
// queries (order_status_histories, driver_tasks — different source tables,
// not a SQL UNION), then merged per-employee in Go.
func (h *Handler) fetchEmployeePerformance(ctx context.Context, outletID pgtype.UUID, scoped bool, dateFrom, dateTo pgtype.Timestamptz) ([]EmployeePerformanceResponse, error) {
	outletFilter := pgtype.UUID{Valid: false}
	if scoped {
		outletFilter = outletID
	}

	workerRows, err := h.Queries.WorkerPerformanceReport(ctx, db.WorkerPerformanceReportParams{
		OutletID: outletFilter,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return nil, err
	}

	driverRows, err := h.Queries.DriverPerformanceReport(ctx, db.DriverPerformanceReportParams{
		OutletID: outletFilter,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return nil, err
	}

	type counts struct {
		worker, driver int64
	}
	byEmployee := make(map[string]counts)
	var ids []pgtype.UUID

	for _, r := range workerRows {
		key := r.EmployeeID.String()
		if _, ok := byEmployee[key]; !ok {
			ids = append(ids, r.EmployeeID)
		}
		c := byEmployee[key]
		c.worker += r.TotalJobs
		byEmployee[key] = c
	}
	for _, r := range driverRows {
		key := r.EmployeeID.String()
		if _, ok := byEmployee[key]; !ok {
			ids = append(ids, r.EmployeeID)
		}
		c := byEmployee[key]
		c.driver += r.TotalJobs
		byEmployee[key] = c
	}

	if len(ids) == 0 {
		return []EmployeePerformanceResponse{}, nil
	}

	employees, err := h.Queries.GetEmployeesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	result := make([]EmployeePerformanceResponse, 0, len(employees))
	for _, e := range employees {
		c := byEmployee[e.ID.String()]
		item := EmployeePerformanceResponse{
			EmployeeID: e.ID.String(),
			Name:       e.FullName,
			Role:       e.Role,
			WorkerJobs: c.worker,
			DriverJobs: c.driver,
			TotalJobs:  c.worker + c.driver,
		}
		if e.OutletID.Valid {
			item.OutletID = e.OutletID.String()
		}
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalJobs > result[j].TotalJobs
	})

	return result, nil
}

func (h *Handler) GetEmployeePerformanceReport(c *gin.Context) {
	outletID, scoped := reportOutletFilter(c)
	dateFrom, dateTo := dateRangeFilter(c)

	data, err := h.fetchEmployeePerformance(c.Request.Context(), outletID, scoped, dateFrom, dateTo)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, EmployeePerformanceListResponse{Data: data})
}

func (h *Handler) ExportEmployeePerformanceReport(c *gin.Context) {
	outletID, scoped := reportOutletFilter(c)
	dateFrom, dateTo := dateRangeFilter(c)

	data, err := h.fetchEmployeePerformance(c.Request.Context(), outletID, scoped, dateFrom, dateTo)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	header := []string{"employee_id", "name", "role", "outlet_id", "worker_jobs", "driver_jobs", "total_jobs"}
	rows := make([][]string, 0, len(data))
	for _, item := range data {
		rows = append(rows, []string{
			item.EmployeeID,
			item.Name,
			item.Role,
			item.OutletID,
			fmt.Sprintf("%d", item.WorkerJobs),
			fmt.Sprintf("%d", item.DriverJobs),
			fmt.Sprintf("%d", item.TotalJobs),
		})
	}

	filename := fmt.Sprintf("employee_performance_%s.csv", time.Now().Format("2006-01-02"))
	apphelper.WriteCSV(c, "text/csv; charset=utf-8", filename, header, rows, true)
}
