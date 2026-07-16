package shift

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) ListEmployeeShifts(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	shifts, err := h.Queries.ListEmployeeShiftsByEmployee(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]EmployeeShiftResponse, 0, len(shifts))
	for _, es := range shifts {
		data = append(data, toEmployeeShiftResponse(es))
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) CreateEmployeeShift(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	var req EmployeeShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hasDayOfWeek := req.DayOfWeek != nil
	hasDate := req.Date != ""
	if hasDayOfWeek == hasDate {
		apperr.RespondError(c, http.StatusBadRequest, "must_set_day_of_week_xor_date")
		return
	}

	var shiftID, outletID pgtype.UUID
	if err := shiftID.Scan(req.ShiftID); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_shift_id")
		return
	}
	if err := outletID.Scan(req.OutletID); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_outlet_id")
		return
	}

	dayOfWeek := pgtype.Int2{Valid: false}
	if hasDayOfWeek {
		if *req.DayOfWeek < 0 || *req.DayOfWeek > 6 {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_day_of_week")
			return
		}
		dayOfWeek = pgtype.Int2{Int16: *req.DayOfWeek, Valid: true}
	}

	date := pgtype.Date{Valid: false}
	if hasDate {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_date")
			return
		}
		date = pgtype.Date{Time: parsed, Valid: true}
	}

	created, err := h.Queries.CreateEmployeeShift(c.Request.Context(), db.CreateEmployeeShiftParams{
		EmployeeID: employeeID,
		ShiftID:    shiftID,
		OutletID:   outletID,
		DayOfWeek:  dayOfWeek,
		Date:       date,
		IsActive:   req.IsActive,
	})
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "employee_shift_already_assigned")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toEmployeeShiftResponse(created)
	resp.Message = "shift assigned successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) DeleteEmployeeShift(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	var shiftRecordID pgtype.UUID
	if err := shiftRecordID.Scan(c.Param("shiftRecordId")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_shift_id")
		return
	}

	if _, err := h.Queries.GetEmployeeShiftByID(c.Request.Context(), db.GetEmployeeShiftByIDParams{
		ID:         shiftRecordID,
		EmployeeID: employeeID,
	}); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_shift_not_found")
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.DeleteEmployeeShift(c.Request.Context(), db.DeleteEmployeeShiftParams{
		ID:         shiftRecordID,
		EmployeeID: employeeID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "shift assignment removed successfully"})
}
