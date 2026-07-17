package customer

import (
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	customerUUID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existingCustomer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerUUID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	customerResponse := CustomerResponse{
		ID:        existingCustomer.ID.String(),
		FullName:  existingCustomer.FullName,
		Email:     existingCustomer.Email,
		Phone:     existingCustomer.Phone.String,
		AvatarURL: existingCustomer.AvatarUrl.String,
		Message:   "get profile successfully",
	}

	c.JSON(http.StatusOK, customerResponse)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerUUID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existingCustomer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerUUID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := auth.ComparePassword(existingCustomer.PasswordHash.String, req.CurrentPassword); err != nil {
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

	updateCustomerPasswordParams := db.UpdateCustomerPasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           existingCustomer.ID,
	}

	updatedCustomer, err := h.Queries.UpdateCustomerPassword(c.Request.Context(), updateCustomerPasswordParams)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := h.Queries.RevokeRefreshTokensByCustomerID(c.Request.Context(), updatedCustomer.ID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	bumpedCustomer, err := h.Queries.IncrementCustomerTokenVersion(c.Request.Context(), updatedCustomer.ID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if _, _, err := h.issueTokens(c, bumpedCustomer.ID, bumpedCustomer.TokenVersion); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

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

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	if len(req.FullName) == 0 {
		apperr.RespondError(c, http.StatusBadRequest, "full_name_required")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if len(req.Phone) == 0 {
		apperr.RespondError(c, http.StatusBadRequest, "phone_required")
		return
	}

	customerUUID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	updateProfileParams := db.UpdateCustomerProfileParams{
		FullName: req.FullName,
		Phone:    pgtype.Text{String: req.Phone, Valid: true},
		ID:       customerUUID,
	}

	updatedCustomer, err := h.Queries.UpdateCustomerProfile(c.Request.Context(), updateProfileParams)
	if err != nil {
		apperr.RespondInternalError(c, err)
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

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerUUID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existingCustomer, err := h.Queries.GetCustomerByID(c.Request.Context(), customerUUID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := auth.ComparePassword(existingCustomer.PasswordHash.String, req.CurrentPassword); err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_password")
		return
	}

	if _, err := h.Queries.GetCustomerByEmail(c.Request.Context(), req.NewEmail); err == nil {
		apperr.RespondError(c, http.StatusConflict, "email_already_registered")
		return
	}

	token, err := auth.GenerateRandomToken()
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	createEmailChangeTokenParams := db.CreateEmailChangeTokenParams{
		CustomerID: existingCustomer.ID,
		NewEmail:   req.NewEmail,
		TokenHash:  auth.HashToken(token),
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}

	if _, err = h.Queries.CreateEmailChangeToken(c.Request.Context(), createEmailChangeTokenParams); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err = h.emailClient.SendEmailChangeVerification(req.NewEmail, token); err != nil {
		log.Printf("error in sending email change verification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent to your new email!"})
}

func (h *Handler) VerifyEmailChange(c *gin.Context) {
	var req VerifyEmailChangeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedToken := auth.HashToken(req.Token)

	emailChangeToken, err := h.Queries.GetEmailChangeTokenByHash(c.Request.Context(), hashedToken)
	if err != nil {
		apperr.RespondError(c, http.StatusUnauthorized, "invalid_or_expired_token")
		return
	}

	updatedCustomer, err := h.Queries.UpdateCustomerEmail(c.Request.Context(), db.UpdateCustomerEmailParams{
		Email: emailChangeToken.NewEmail,
		ID:    emailChangeToken.CustomerID,
	})

	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "email_already_registered")
		return
	}

	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err = h.Queries.MarkEmailChangeTokenUsed(c.Request.Context(), emailChangeToken.ID); err != nil {
		apperr.RespondInternalError(c, err)
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
	customerUUID, customerIDStr, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if fileHeader.Size > maxAvatarSize {
		apperr.RespondError(c, http.StatusBadRequest, "avatar_too_large")
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedAvatarContentTypes[contentType] {
		apperr.RespondError(c, http.StatusBadRequest, "avatar_invalid_type")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer file.Close()

	avatarURL, err := h.storageClient.UploadAvatar(c.Request.Context(), file, customerIDStr)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	updatedCustomer, err := h.Queries.UpdateCustomerAvatar(c.Request.Context(), db.UpdateCustomerAvatarParams{
		AvatarUrl: pgtype.Text{String: avatarURL, Valid: true},
		ID:        customerUUID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
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
