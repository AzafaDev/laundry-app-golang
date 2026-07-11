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

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	var pgErr *pgconn.PgError

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	req.FullName = strings.TrimSpace(req.FullName)
	if len(req.FullName) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "full name is required"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	customer, err := h.Queries.CreateCustomer(c.Request.Context(), db.CreateCustomerParams{
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
	})

	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		c.JSON(http.StatusConflict, gin.H{"error": "email has been registered"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	emailVerificationParams := db.CreateEmailVerificationTokenParams{
		CustomerID: customer.ID,
		TokenHash:  auth.HashToken(token),
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}

	_, err = h.Queries.CreateEmailVerificationToken(c.Request.Context(), emailVerificationParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = h.emailClient.SendVerificationEmail(customer.Email, token)
	if err != nil {
		log.Printf("error in sending verification email: %v", err)
	}

	resp := CustomerResponse{
		ID:       customer.ID.String(),
		FullName: customer.FullName,
		Email:    customer.Email,
		Message:  "Email verification has been sent to your email!",
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.Queries.GetCustomerByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if customer.PasswordHash.Valid {
		if err := auth.ComparePassword(customer.PasswordHash.String, req.Password); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := auth.GenerateAccessToken(customer.ID.String(), customer.TokenVersion, h.Config.JWTAccessSecret)
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
	_, err = h.Queries.CreateRefreshToken(c.Request.Context(), db.CreateRefreshTokenParams{
		CustomerID: customer.ID,
		TokenHash:  hashedRefreshToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error in inserting refresh token to db"})
		return
	}

	var secure bool
	if h.Config.GoEnv == "production" {
		secure = true
	} else {
		secure = false
	}

	c.SetCookie("access_token", token, 15*60, "/", "", secure, true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", secure, true)

	resp := CustomerResponse{
		ID:       customer.ID.String(),
		FullName: customer.FullName,
		Email:    customer.Email,
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	hashedRefreshToken := auth.HashToken(refreshToken)
	existingRefreshToken, err := h.Queries.GetRefreshTokenByHash(c.Request.Context(), hashedRefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	err = h.Queries.RevokeRefreshToken(c.Request.Context(), existingRefreshToken.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newRefreshToken, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.Queries.GetCustomerByID(c.Request.Context(), existingRefreshToken.CustomerID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}

	accessToken, err := auth.GenerateAccessToken(customer.ID.String(), customer.TokenVersion, h.Config.JWTAccessSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	hashedNewRefreshToken := auth.HashToken(newRefreshToken)
	_, err = h.Queries.CreateRefreshToken(c.Request.Context(), db.CreateRefreshTokenParams{
		CustomerID: existingRefreshToken.CustomerID,
		TokenHash:  hashedNewRefreshToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error in inserting refresh token to db"})
		return
	}

	var secure bool
	if h.Config.GoEnv == "production" {
		secure = true
	} else {
		secure = false
	}

	c.SetCookie("access_token", accessToken, 15*60, "/", "", secure, true)
	c.SetCookie("refresh_token", newRefreshToken, 7*24*60*60, "/", "", secure, true)

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("refresh_token")

	if refreshToken != "" {
		hashedRefreshToken := auth.HashToken(refreshToken)
		existing, _ := h.Queries.GetRefreshTokenByHash(c.Request.Context(), hashedRefreshToken)
		h.Queries.RevokeRefreshToken(c.Request.Context(), existing.ID)
	}

	var secure bool
	if h.Config.GoEnv == "production" {
		secure = true
	} else {
		secure = false
	}

	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedToken := auth.HashToken(req.Token)

	emailVerificationToken, err := h.Queries.GetEmailVerificationByTokenHash(c.Request.Context(), hashedToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid or expired token"})
		return
	}

	if err = h.Queries.MarkEmailVerificationTokenUsed(c.Request.Context(), emailVerificationToken.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = h.Queries.VerifyCustomerEmail(c.Request.Context(), emailVerificationToken.CustomerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully!"})
}

func (h *Handler) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.Queries.GetCustomerByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent!"})
		return
	}

	if customer.IsVerified {
		c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent!"})
		return
	}

	token, err := auth.GenerateRandomToken()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent!"})
		return
	}

	hashedToken := auth.HashToken(token)

	createEmailVerificationTokenParams := db.CreateEmailVerificationTokenParams{
		CustomerID: customer.ID,
		TokenHash:  hashedToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}

	_, err = h.Queries.CreateEmailVerificationToken(c.Request.Context(), createEmailVerificationTokenParams)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent!"})
		return
	}

	err = h.emailClient.SendVerificationEmail(customer.Email, token)
	if err != nil {
		log.Printf("error in sending verification email: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification email has been sent!"})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Queries.GetCustomerByEmail(c.Request.Context(), req.Email)
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

	params := db.CreatePasswordResetTokenParams{
		CustomerID: existing.ID,
		TokenHash:  hashedToken,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}
	_, err = h.Queries.CreatePasswordResetToken(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "password reset email has been sent!"})
		return
	}

	if err = h.emailClient.SendPasswordResetEmail(existing.Email, token); err != nil {
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

	passwordResetToken, err := h.Queries.GetPasswordResetTokenByHash(c.Request.Context(), hashedToken)
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

	updatedCustomerPasswordParams := db.UpdateCustomerPasswordParams{
		PasswordHash: pgtype.Text{String: hashedPassword, Valid: true},
		ID:           passwordResetToken.CustomerID,
	}
	updatedCustomer, err := h.Queries.UpdateCustomerPassword(c.Request.Context(), updatedCustomerPasswordParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = h.Queries.MarkPasswordResetTokenUsed(c.Request.Context(), passwordResetToken.ID); err != nil {
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

	if err = h.Queries.RevokeRefreshTokensByCustomerID(c.Request.Context(), updatedCustomer.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = h.Queries.CreateRefreshToken(c.Request.Context(), createRefreshTokenParams)
	if err != nil {
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
		Message:  "reset password successfully!",
	}
	c.JSON(http.StatusOK, customerResponse)
}
