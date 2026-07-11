package customer

import (
	"errors"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxAvatarSize = 2 << 20 // 2MB

var allowedAvatarContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

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

	bumpedCustomer, err := h.Queries.IncrementCustomerTokenVersion(c.Request.Context(), updatedCustomer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := auth.GenerateAccessToken(bumpedCustomer.ID.String(), bumpedCustomer.TokenVersion, h.Config.JWTAccessSecret)
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

	c.SetSameSite(h.cookieSameSite())
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

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	var customerUUID pgtype.UUID

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	if len(req.FullName) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "full name is required"})
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if len(req.Phone) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phone is required"})
		return
	}

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

	updateProfileParams := db.UpdateCustomerProfileParams{
		FullName: req.FullName,
		Phone:    pgtype.Text{String: req.Phone, Valid: true},
		ID:       customerUUID,
	}

	updatedCustomer, err := h.Queries.UpdateCustomerProfile(c.Request.Context(), updateProfileParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customerResponse := CustomerResponse{
		ID:       updatedCustomer.ID.String(),
		FullName: updatedCustomer.FullName,
		Email:    updatedCustomer.Email,
		Phone:    updatedCustomer.Phone.String,
		Message:  "updated profile successfully!",
	}

	c.JSON(http.StatusOK, customerResponse)
}

func (h *Handler) RequestEmailChange(c *gin.Context) {
	var req RequestEmailChangeRequest
	var customerUUID pgtype.UUID

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	if err := auth.ComparePassword(existingCustomer.PasswordHash.String, req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}

	if _, err := h.Queries.GetCustomerByEmail(c.Request.Context(), req.NewEmail); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email has been registered"})
		return
	}

	token, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	createEmailChangeTokenParams := db.CreateEmailChangeTokenParams{
		CustomerID: existingCustomer.ID,
		NewEmail:   req.NewEmail,
		TokenHash:  auth.HashToken(token),
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}

	if _, err = h.Queries.CreateEmailChangeToken(c.Request.Context(), createEmailChangeTokenParams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = h.emailClient.SendEmailChangeVerification(req.NewEmail, token); err != nil {
		log.Printf("error in sending email change verification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent to your new email!"})
}

func (h *Handler) VerifyEmailChange(c *gin.Context) {
	var req VerifyEmailChangeRequest
	var pgErr *pgconn.PgError

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedToken := auth.HashToken(req.Token)

	emailChangeToken, err := h.Queries.GetEmailChangeTokenByHash(c.Request.Context(), hashedToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	updatedCustomer, err := h.Queries.UpdateCustomerEmail(c.Request.Context(), db.UpdateCustomerEmailParams{
		Email: emailChangeToken.NewEmail,
		ID:    emailChangeToken.CustomerID,
	})

	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		c.JSON(http.StatusConflict, gin.H{"error": "email has been registered"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = h.Queries.MarkEmailChangeTokenUsed(c.Request.Context(), emailChangeToken.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customerResponse := CustomerResponse{
		ID:       updatedCustomer.ID.String(),
		FullName: updatedCustomer.FullName,
		Email:    updatedCustomer.Email,
		Message:  "email changed successfully!",
	}

	c.JSON(http.StatusOK, customerResponse)
}

func (h *Handler) UploadAvatar(c *gin.Context) {
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

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if fileHeader.Size > maxAvatarSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar file is too large, max 2MB"})
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedAvatarContentTypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar must be jpeg, png, or webp"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	avatarURL, err := h.storageClient.UploadAvatar(c.Request.Context(), file, customerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedCustomer, err := h.Queries.UpdateCustomerAvatar(c.Request.Context(), db.UpdateCustomerAvatarParams{
		AvatarUrl: pgtype.Text{String: avatarURL, Valid: true},
		ID:        customerUUID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customerResponse := CustomerResponse{
		ID:        updatedCustomer.ID.String(),
		FullName:  updatedCustomer.FullName,
		Email:     updatedCustomer.Email,
		AvatarURL: updatedCustomer.AvatarUrl.String,
		Message:   "avatar uploaded successfully!",
	}

	c.JSON(http.StatusOK, customerResponse)
}
