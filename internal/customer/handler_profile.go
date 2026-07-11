package customer

import (
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) Me(c *gin.Context) {
	customerID, _ := c.Get("customer_id")
	c.JSON(http.StatusOK, gin.H{"customer_id": customerID})
}

func (h *Handler) Profile(c *gin.Context) {
	var customerUUID pgtype.UUID
	customerIDVal, ok := c.Get("customer_id")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	customerIDStr, ok := customerIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	if err := customerUUID.Scan(customerIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	existingCustomer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customerResponse := CustomerResponse{
		ID:       existingCustomer.ID.String(),
		FullName: existingCustomer.FullName,
		Email:    existingCustomer.Email,
		Message:  "get profile successfully",
	}

	c.JSON(http.StatusOK, customerResponse)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	var customerUUID pgtype.UUID

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerID, ok := c.Get("customer_id")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	customerIDStr, ok := customerID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	if err := customerUUID.Scan(customerIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	existingCustomer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := auth.ComparePassword(existingCustomer.PasswordHash.String, req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updateCustomerPasswordParams := db.UpdateCustomerPasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           existingCustomer.ID,
	}

	updatedCustomer, err := h.Queries.UpdateCustomerPassword(c.Request.Context(), updateCustomerPasswordParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.RevokeRefreshTokensByCustomerID(c.Request.Context(), updatedCustomer.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := auth.GenerateAccessToken(updatedCustomer.ID.String(), h.Config.JWTAccessSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	refreshToken, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	hashedRefreshToken := auth.HashToken(refreshToken)

	createRefreshTokenParams := db.CreateRefreshTokenParams{
		CustomerID: updatedCustomer.ID,
		TokenHash:  hashedRefreshToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
	}

	if _, err := h.Queries.CreateRefreshToken(c.Request.Context(), createRefreshTokenParams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var secure bool
	if h.Config.GoEnv == "production" {
		secure = true
	} else {
		secure = false
	}

	c.SetCookie("access_token", accessToken, 15*60, "/", "", secure, true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", secure, true)

	customerResponse := CustomerResponse{
		ID:       updatedCustomer.ID.String(),
		FullName: updatedCustomer.FullName,
		Email:    updatedCustomer.Email,
		Message:  "changed password successfully!",
	}

	c.JSON(http.StatusOK, customerResponse)
}
