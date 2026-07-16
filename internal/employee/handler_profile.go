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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	existingEmployee, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toEmployeeResponse(existingEmployee))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employeeUUID, err := h.currentEmployeeID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	existingEmployee, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedEmployee, err := h.Queries.UpdateEmployeePassword(c.Request.Context(), db.UpdateEmployeePasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           existingEmployee.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.RevokeEmployeeRefreshTokensByEmployeeID(c.Request.Context(), updatedEmployee.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	bumpedEmployee, err := h.Queries.IncrementEmployeeTokenVersion(c.Request.Context(), updatedEmployee.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, _, err := h.issueEmployeeTokens(c, bumpedEmployee.ID, bumpedEmployee.Role, bumpedEmployee.TokenVersion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toEmployeeResponse(bumpedEmployee)
	resp.Message = "changed password successfully!"

	c.JSON(http.StatusOK, resp)
}
