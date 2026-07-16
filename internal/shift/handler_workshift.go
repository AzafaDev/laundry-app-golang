package shift

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) ListWorkShifts(c *gin.Context) {
	limit, offset := parsePagination(c)

	shifts, err := h.Queries.ListWorkShifts(c.Request.Context(), db.ListWorkShiftsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalCount, err := h.Queries.CountWorkShifts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := make([]WorkShiftResponse, 0, len(shifts))
	for _, ws := range shifts {
		data = append(data, toWorkShiftResponse(ws))
	}

	c.JSON(http.StatusOK, WorkShiftListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetWorkShiftByID(c *gin.Context) {
	var shiftID pgtype.UUID
	if err := shiftID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_work_shift_id")
		return
	}

	ws, err := h.Queries.GetWorkShiftByID(c.Request.Context(), shiftID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "work_shift_not_found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toWorkShiftResponse(ws))
}

func (h *Handler) CreateWorkShift(c *gin.Context) {
	var req WorkShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime, err := parseTimeOfDay(req.StartTime)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_start_time")
		return
	}
	endTime, err := parseTimeOfDay(req.EndTime)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_end_time")
		return
	}

	created, err := h.Queries.CreateWorkShift(c.Request.Context(), db.CreateWorkShiftParams{
		Name:        req.Name,
		StartTime:   startTime,
		EndTime:     endTime,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		IsActive:    req.IsActive,
	})
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toWorkShiftResponse(created)
	resp.Message = "work shift created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateWorkShift(c *gin.Context) {
	var shiftID pgtype.UUID
	if err := shiftID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_work_shift_id")
		return
	}

	var req WorkShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime, err := parseTimeOfDay(req.StartTime)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_start_time")
		return
	}
	endTime, err := parseTimeOfDay(req.EndTime)
	if err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_end_time")
		return
	}

	updated, err := h.Queries.UpdateWorkShift(c.Request.Context(), db.UpdateWorkShiftParams{
		Name:        req.Name,
		StartTime:   startTime,
		EndTime:     endTime,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		IsActive:    req.IsActive,
		ID:          shiftID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "work_shift_not_found")
		return
	}
	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "name_already_exists")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toWorkShiftResponse(updated)
	resp.Message = "work shift updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SoftDeleteWorkShift(c *gin.Context) {
	var shiftID pgtype.UUID
	if err := shiftID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_work_shift_id")
		return
	}

	if _, err := h.Queries.GetWorkShiftByID(c.Request.Context(), shiftID); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "work_shift_not_found")
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	activeAssignments, err := h.Queries.CountActiveEmployeeShiftsByShiftID(c.Request.Context(), shiftID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if activeAssignments > 0 {
		apperr.RespondError(c, http.StatusBadRequest, "work_shift_still_assigned")
		return
	}

	if err := h.Queries.SoftDeleteWorkShift(c.Request.Context(), shiftID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "work shift deleted successfully"})
}

func (h *Handler) HardDeleteWorkShift(c *gin.Context) {
	var shiftID pgtype.UUID
	if err := shiftID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_work_shift_id")
		return
	}

	existing, err := h.Queries.GetWorkShiftByIDAny(c.Request.Context(), shiftID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "work_shift_not_found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !existing.DeletedAt.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "work_shift_must_be_deleted_first")
		return
	}

	if err := h.Queries.HardDeleteWorkShift(c.Request.Context(), shiftID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "work shift permanently deleted"})
}
