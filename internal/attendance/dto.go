package attendance

type CheckInRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

type CheckOutRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type AttendanceResponse struct {
	ID           string `json:"id"`
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name,omitempty"`
	OutletID     string `json:"outlet_id"`
	OutletName   string `json:"outlet_name,omitempty"`
	Date         string `json:"date"`
	CheckInTime  string `json:"check_in_time,omitempty"`
	CheckOutTime string `json:"check_out_time,omitempty"`
	IsLate       bool   `json:"is_late"`
	LateMinutes  int32  `json:"late_minutes"`
	Status       string `json:"status"`
	Notes        string `json:"notes,omitempty"`
	Message      string `json:"message,omitempty"`
}

type AttendanceListResponse struct {
	Data       []AttendanceResponse `json:"data"`
	TotalCount int64                `json:"total_count"`
}

type SweepRequest struct {
	Date string `json:"date" binding:"required"` // "YYYY-MM-DD"
}

type SweepResponse struct {
	Date           string `json:"date"`
	MarkedAbsent   int    `json:"marked_absent"`
	AutoCheckedOut int    `json:"auto_checked_out"`
}
