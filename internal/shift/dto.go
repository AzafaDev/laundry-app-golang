package shift

type WorkShiftRequest struct {
	Name        string `json:"name" binding:"required,max=50"`
	StartTime   string `json:"start_time" binding:"required"` // "HH:MM"
	EndTime     string `json:"end_time" binding:"required"`   // "HH:MM"
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type WorkShiftResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	Message     string `json:"message,omitempty"`
}

type WorkShiftListResponse struct {
	Data       []WorkShiftResponse `json:"data"`
	TotalCount int64               `json:"total_count"`
}

type EmployeeShiftRequest struct {
	ShiftID   string `json:"shift_id" binding:"required"`
	OutletID  string `json:"outlet_id" binding:"required"`
	DayOfWeek *int16 `json:"day_of_week"`
	Date      string `json:"date"` // "YYYY-MM-DD", mutually exclusive with DayOfWeek
	IsActive  bool   `json:"is_active"`
}

type EmployeeShiftResponse struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	ShiftID    string `json:"shift_id"`
	OutletID   string `json:"outlet_id"`
	DayOfWeek  *int16 `json:"day_of_week,omitempty"`
	Date       string `json:"date,omitempty"`
	IsActive   bool   `json:"is_active"`
	Message    string `json:"message,omitempty"`
}
