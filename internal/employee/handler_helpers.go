package employee

import (
	"errors"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// parsePagination reads limit/offset query params, defaulting to 50/0 and
// clamping limit to 100, floor-ing invalid or negative values to their
// defaults.
func parsePagination(c *gin.Context) (limit, offset int32) {
	limit = defaultPageLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. duplicate email).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

func (h *Handler) cookieSecure() bool {
	return h.Config.GoEnv == "production"
}

func (h *Handler) cookieSameSite() http.SameSite {
	if h.Config.GoEnv == "production" {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

// currentEmployeeID reads the "employee_id" set by EmployeeAuthMiddleware and
// converts it into a pgtype.UUID.
func (h *Handler) currentEmployeeID(c *gin.Context) (pgtype.UUID, error) {
	var employeeUUID pgtype.UUID

	employeeIDVal, ok := c.Get("employee_id")
	if !ok {
		return employeeUUID, errors.New("something went wrong")
	}

	employeeIDStr, ok := employeeIDVal.(string)
	if !ok {
		return employeeUUID, errors.New("something went wrong")
	}

	if err := employeeUUID.Scan(employeeIDStr); err != nil {
		return employeeUUID, err
	}

	return employeeUUID, nil
}

// issueEmployeeTokens generates a fresh access+refresh token pair for an
// employee, persists the refresh token, and sets both as cookies on the
// response.
func (h *Handler) issueEmployeeTokens(c *gin.Context, employeeID pgtype.UUID, role string, tokenVersion int32) (accessToken, refreshToken string, err error) {
	accessToken, err = auth.GenerateEmployeeAccessToken(employeeID.String(), role, tokenVersion, h.Config.JWTEmployeeAccessSecret)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = auth.GenerateRandomToken()
	if err != nil {
		return "", "", err
	}

	_, err = h.Queries.CreateEmployeeRefreshToken(c.Request.Context(), db.CreateEmployeeRefreshTokenParams{
		EmployeeID: employeeID,
		TokenHash:  auth.HashToken(refreshToken),
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		return "", "", err
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("staff_access_token", accessToken, 15*60, "/", "", h.cookieSecure(), true)
	c.SetCookie("staff_refresh_token", refreshToken, 7*24*60*60, "/", "", h.cookieSecure(), true)

	return accessToken, refreshToken, nil
}
