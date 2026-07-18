package employee

import (
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) Profile(c *gin.Context) {
	employeeUUID, err := h.currentEmployeeID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existingEmployee, err := h.Queries.GetEmployeeByIDWithOutlet(c.Request.Context(), employeeUUID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toEmployeeResponseWithOutlet(existingEmployee.ID, existingEmployee.FullName, existingEmployee.Email, existingEmployee.Phone, existingEmployee.Role, existingEmployee.OutletID, existingEmployee.IsActive, existingEmployee.DeletedAt, existingEmployee.OutletName, existingEmployee.OutletDeletedAt))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employeeUUID, err := h.currentEmployeeID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existingEmployee, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeUUID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := auth.ComparePassword(existingEmployee.PasswordHash.String, req.CurrentPassword); err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_password")
		return
	}

	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if len(req.NewPassword) < 8 {
		apperr.RespondError(c, http.StatusBadRequest, "password_too_short")
		return
	}

	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())
	qtx := h.Queries.WithTx(tx)

	updatedEmployee, err := qtx.UpdateEmployeePassword(c.Request.Context(), db.UpdateEmployeePasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           existingEmployee.ID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := qtx.RevokeEmployeeRefreshTokensByEmployeeID(c.Request.Context(), updatedEmployee.ID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	bumpedEmployee, err := qtx.IncrementEmployeeTokenVersion(c.Request.Context(), updatedEmployee.ID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if _, _, err := h.issueEmployeeTokens(c, bumpedEmployee.ID, bumpedEmployee.Role, bumpedEmployee.TokenVersion); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toEmployeeResponse(bumpedEmployee)
	resp.Message = "changed password successfully!"

	c.JSON(http.StatusOK, resp)
}
