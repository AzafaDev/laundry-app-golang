package attendance

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/shift"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CheckIn(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var req CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().In(shift.JakartaLocation)

	_, ws, ok := shift.ResolveEmployeeShiftForDate(c.Request.Context(), h.Queries, employeeID, now)
	if !ok {
		apperr.RespondError(c, http.StatusBadRequest, "no_shift_today")
		return
	}

	shiftStart, shiftEnd := shift.ResolveShiftWindow(ws, now)
	if now.Before(shiftStart.Add(-checkinToleranceBeforeShift)) {
		apperr.RespondError(c, http.StatusBadRequest, "too_early")
		return
	}
	if now.After(shiftEnd) {
		apperr.RespondError(c, http.StatusBadRequest, "shift_ended")
		return
	}

	outlet, err := resolveEmployeeOutlet(c.Request.Context(), h.Queries, employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusBadRequest, "no_outlet_assigned")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	distanceKM := haversineKM(numericToFloat64(outlet.Latitude), numericToFloat64(outlet.Longitude), req.Latitude, req.Longitude)
	if distanceKM*1000 > float64(h.Config.CheckinRadiusMeters) {
		apperr.RespondError(c, http.StatusBadRequest, "outside_geofence")
		return
	}

	checkInLat, err := float64ToNumeric(req.Latitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	checkInLon, err := float64ToNumeric(req.Longitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lateMinutes := int32(now.Sub(shiftStart).Minutes())
	isLate := lateMinutes > int32(h.Config.LateThresholdMinutes)
	status := "on_time"
	if isLate {
		status = "late"
	}
	if lateMinutes < 0 {
		lateMinutes = 0
	}

	created, err := h.Queries.CreateAttendance(c.Request.Context(), db.CreateAttendanceParams{
		EmployeeID:       employeeID,
		OutletID:         outlet.ID,
		Date:             pgtype.Date{Time: shift.CivilDateStart(now), Valid: true},
		CheckInTime:      pgtype.Timestamptz{Time: now, Valid: true},
		CheckInLatitude:  checkInLat,
		CheckInLongitude: checkInLon,
		IsLate:           pgtype.Bool{Bool: isLate, Valid: true},
		LateMinutes:      pgtype.Int4{Int32: lateMinutes, Valid: true},
		Status:           pgtype.Text{String: status, Valid: true},
	})
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "already_checked_in")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toAttendanceResponse(created)
	resp.Message = "check-in successful"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) CheckOut(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	var req CheckOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().In(shift.JakartaLocation)

	_, ws, ok := shift.ResolveEmployeeShiftForDate(c.Request.Context(), h.Queries, employeeID, now)
	if !ok {
		apperr.RespondError(c, http.StatusBadRequest, "no_shift_today")
		return
	}

	_, shiftEnd := shift.ResolveShiftWindow(ws, now)
	if now.Before(shiftEnd) {
		apperr.RespondError(c, http.StatusBadRequest, "too_early_to_check_out")
		return
	}

	todayDate := pgtype.Date{Time: shift.CivilDateStart(now), Valid: true}

	existing, err := h.Queries.GetAttendanceByEmployeeAndDate(c.Request.Context(), db.GetAttendanceByEmployeeAndDateParams{
		EmployeeID: employeeID,
		Date:       todayDate,
	})
	if errors.Is(err, pgx.ErrNoRows) || !existing.CheckInTime.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "not_checked_in")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing.CheckOutTime.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "already_checked_out")
		return
	}

	// Check-out deliberately does not enforce geofencing — asymmetry
	// preserved from the source system. Lat/long are stored if sent, but
	// never validated against the outlet radius.
	var checkOutLat, checkOutLon pgtype.Numeric
	if req.Latitude != nil {
		checkOutLat, err = float64ToNumeric(*req.Latitude)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Longitude != nil {
		checkOutLon, err = float64ToNumeric(*req.Longitude)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	updated, err := h.Queries.CheckOutAttendance(c.Request.Context(), db.CheckOutAttendanceParams{
		CheckOutTime:      pgtype.Timestamptz{Time: now, Valid: true},
		CheckOutLatitude:  checkOutLat,
		CheckOutLongitude: checkOutLon,
		EmployeeID:        employeeID,
		Date:              todayDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusBadRequest, "already_checked_out")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toAttendanceResponse(updated)
	resp.Message = "check-out successful"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) MyAttendanceLogs(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	limit, offset := parsePagination(c)

	logs, err := h.Queries.ListAttendancesByEmployee(c.Request.Context(), db.ListAttendancesByEmployeeParams{
		EmployeeID: employeeID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalCount, err := h.Queries.CountAttendancesByEmployee(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]AttendanceResponse, 0, len(logs))
	for _, a := range logs {
		data = append(data, toAttendanceResponse(a))
	}

	c.JSON(http.StatusOK, AttendanceListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) TodayAttendance(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	now := time.Now().In(shift.JakartaLocation)
	today, err := h.Queries.GetAttendanceByEmployeeAndDate(c.Request.Context(), db.GetAttendanceByEmployeeAndDateParams{
		EmployeeID: employeeID,
		Date:       pgtype.Date{Time: shift.CivilDateStart(now), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toAttendanceResponse(today)})
}

func (h *Handler) CurrentShift(c *gin.Context) {
	employeeID, err := currentEmployeeID(c)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	now := time.Now().In(shift.JakartaLocation)
	_, ws, ok := shift.ResolveEmployeeShiftForDate(c.Request.Context(), h.Queries, employeeID, now)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	start, end := shift.ResolveShiftWindow(ws, now)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"shift_id":   ws.ID.String(),
		"name":       ws.Name,
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
	}})
}
