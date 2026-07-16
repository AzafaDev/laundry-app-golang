package report

type SalesReportPeriod struct {
	Period          string  `json:"period"`
	Income          float64 `json:"income"`
	OrderCount      int64   `json:"order_count"`
	AveragePerOrder float64 `json:"average_per_order"`
}

type SalesReportSummary struct {
	TotalIncome      float64 `json:"total_income"`
	TotalOrders      int64   `json:"total_orders"`
	AveragePerOrder  float64 `json:"average_per_order"`
	AveragePerPeriod float64 `json:"average_per_period"`
	PeriodCount      int     `json:"period_count"`
}

type SalesReportResponse struct {
	Summary SalesReportSummary  `json:"summary"`
	Chart   []SalesReportPeriod `json:"chart"`
}

type EmployeePerformanceResponse struct {
	EmployeeID string `json:"employee_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	OutletID   string `json:"outlet_id,omitempty"`
	WorkerJobs int64  `json:"worker_jobs"`
	DriverJobs int64  `json:"driver_jobs"`
	TotalJobs  int64  `json:"total_jobs"`
}

type EmployeePerformanceListResponse struct {
	Data []EmployeePerformanceResponse `json:"data"`
}
