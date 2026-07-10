package customer

import (
	"errors"
	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	Queries *db.Queries
	Config  config.Config
}

func NewHandler(queries *db.Queries, cfg config.Config) *Handler {
	return &Handler{
		Queries: queries,
		Config:  cfg,
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	var pgErr *pgconn.PgError

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	resp := CustomerResponse{
		ID:       customer.ID.String(),
		FullName: customer.FullName,
		Email:    customer.Email,
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

	token, err := auth.GenerateAccessToken(customer.ID.String(), h.Config.JWTAccessSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken()
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

func (h *Handler) Me(c *gin.Context) {
	customerID, _ := c.Get("customer_id")
	c.JSON(http.StatusOK, gin.H{"customer_id": customerID})
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

	newRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	accessToken, err := auth.GenerateAccessToken(existingRefreshToken.CustomerID.String(), h.Config.JWTAccessSecret)
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
