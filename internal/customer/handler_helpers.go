package customer

import (
	"errors"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate email).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// currentCustomerID reads the "customer_id" set by the auth middleware and
// converts it into a pgtype.UUID, along with the raw string form (needed by
// handlers like UploadAvatar that pass it straight through to other clients).
func (h *Handler) currentCustomerID(c *gin.Context) (pgtype.UUID, string, error) {
	var customerUUID pgtype.UUID

	customerIDVal, ok := c.Get("customer_id")
	if !ok {
		return customerUUID, "", errors.New("something went wrong")
	}

	customerIDStr, ok := customerIDVal.(string)
	if !ok {
		return customerUUID, "", errors.New("something went wrong")
	}

	if err := customerUUID.Scan(customerIDStr); err != nil {
		return customerUUID, "", err
	}

	return customerUUID, customerIDStr, nil
}

// issueTokens generates a fresh access+refresh token pair for a customer,
// persists the refresh token, and sets both as cookies on the response.
func (h *Handler) issueTokens(c *gin.Context, customerID pgtype.UUID, tokenVersion int32) (accessToken, refreshToken string, err error) {
	accessToken, err = auth.GenerateAccessToken(customerID.String(), tokenVersion, h.Config.JWTAccessSecret)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = auth.GenerateRandomToken()
	if err != nil {
		return "", "", err
	}

	_, err = h.Queries.CreateRefreshToken(c.Request.Context(), db.CreateRefreshTokenParams{
		CustomerID: customerID,
		TokenHash:  auth.HashToken(refreshToken),
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		return "", "", err
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("access_token", accessToken, 15*60, "/", "", h.cookieSecure(), true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", h.cookieSecure(), true)

	return accessToken, refreshToken, nil
}
