package employee

import (
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employee, err := h.Queries.GetEmployeeByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if employee.PasswordHash.Valid {
		if err := auth.ComparePassword(employee.PasswordHash.String, req.Password); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if !employee.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if _, _, err := h.issueEmployeeTokens(c, employee.ID, employee.Role, employee.TokenVersion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toEmployeeResponse(employee))
}

func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("staff_refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	hashedRefreshToken := auth.HashToken(refreshToken)
	existingRefreshToken, err := h.Queries.GetEmployeeRefreshTokenByHash(c.Request.Context(), hashedRefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.RevokeEmployeeRefreshToken(c.Request.Context(), existingRefreshToken.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	employee, err := h.Queries.GetEmployeeByID(c.Request.Context(), existingRefreshToken.EmployeeID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}

	if _, _, err := h.issueEmployeeTokens(c, employee.ID, employee.Role, employee.TokenVersion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("staff_refresh_token")

	if refreshToken != "" {
		hashedRefreshToken := auth.HashToken(refreshToken)
		existing, err := h.Queries.GetEmployeeRefreshTokenByHash(c.Request.Context(), hashedRefreshToken)
		if err == nil {
			h.Queries.RevokeEmployeeRefreshToken(c.Request.Context(), existing.ID)
		}
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("staff_access_token", "", -1, "/", "", h.cookieSecure(), true)
	c.SetCookie("staff_refresh_token", "", -1, "/", "", h.cookieSecure(), true)

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Queries.GetEmployeeByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
		return
	}

	token, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
		return
	}

	hashedToken := auth.HashToken(token)

	params := db.CreateEmployeePasswordResetTokenParams{
		EmployeeID: existing.ID,
		TokenHash:  hashedToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}
	_, err = h.Queries.CreateEmployeePasswordResetToken(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
		return
	}

	if err = h.emailClient.SendEmployeePasswordResetEmail(existing.Email, token); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedToken := auth.HashToken(req.Token)

	passwordResetToken, err := h.Queries.GetEmployeePasswordResetTokenByHash(c.Request.Context(), hashedToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedEmployee, err := h.Queries.UpdateEmployeePassword(c.Request.Context(), db.UpdateEmployeePasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           passwordResetToken.EmployeeID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = h.Queries.MarkEmployeePasswordResetTokenUsed(c.Request.Context(), passwordResetToken.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	bumpedEmployee, err := h.Queries.IncrementEmployeeTokenVersion(c.Request.Context(), updatedEmployee.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.RevokeEmployeeRefreshTokensByEmployeeID(c.Request.Context(), updatedEmployee.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, _, err := h.issueEmployeeTokens(c, bumpedEmployee.ID, bumpedEmployee.Role, bumpedEmployee.TokenVersion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := toEmployeeResponse(bumpedEmployee)
	resp.Message = "reset password successfully!"
	c.JSON(http.StatusOK, resp)
}
