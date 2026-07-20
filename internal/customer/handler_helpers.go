package customer

import (
	"context"
	"errors"
	"laundry-app-with-golang/internal/apphelper"
	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/csrf"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) cookieSecure() bool {
	return h.Config.GoEnv == "production"
}

func (h *Handler) cookieSameSite() http.SameSite {
	if h.Config.GoEnv == "production" {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func (h *Handler) cookieDomain() string {
	return h.Config.CookieDomain
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

	csrfToken, err := csrf.GenerateToken()
	if err != nil {
		return "", "", err
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("access_token", accessToken, 15*60, "/", h.cookieDomain(), h.cookieSecure(), true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", h.cookieDomain(), h.cookieSecure(), true)
	c.SetCookie(csrf.CookieName, csrfToken, 7*24*60*60, "/", h.cookieDomain(), h.cookieSecure(), false)

	return accessToken, refreshToken, nil
}

// setPrimaryAddress atomically unsets any existing primary address for the
// customer and marks addressID as the new primary, inside a single
// transaction so the customer never ends up with zero or multiple primaries.
func (h *Handler) setPrimaryAddress(ctx context.Context, customerID, addressID pgtype.UUID) error {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	if err := qtx.UnsetPrimaryAddresses(ctx, customerID); err != nil {
		return err
	}
	if _, err := qtx.SetAddressPrimary(ctx, db.SetAddressPrimaryParams{
		ID:         addressID,
		CustomerID: customerID,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// toAddressResponse builds an AddressResponse from scalar fields rather than
// a single struct, because sqlc generates a distinct row type per query
// (CreateAddressRow, UpdateAddressRow, ListAddressesRow, ...) even though
// Create/List/Get/Update/SetPrimary all select the same JOIN'd shape. Passing
// scalars lets every call site share this one conversion regardless of which
// generated row type it's converting from.
func toAddressResponse(
	id pgtype.UUID,
	label, address string,
	provinceID, cityID, districtID int32,
	provinceName, cityName, districtName string,
	postalCode pgtype.Text,
	latitude, longitude pgtype.Numeric,
	isPrimary bool,
) AddressResponse {
	return AddressResponse{
		ID:         id.String(),
		Label:      label,
		Address:    address,
		ProvinceID: provinceID,
		CityID:     cityID,
		DistrictID: districtID,
		Province:   provinceName,
		City:       cityName,
		District:   districtName,
		PostalCode: postalCode.String,
		Latitude:   apphelper.NumericToFloat64(latitude),
		Longitude:  apphelper.NumericToFloat64(longitude),
		IsPrimary:  isPrimary,
	}
}
