package customer

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CreateAddress(c *gin.Context) {
	var req AddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	latitude, err := float64ToNumeric(req.Latitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	longitude, err := float64ToNumeric(req.Longitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If the customer has no addresses yet, this one becomes primary
	// automatically regardless of req.IsPrimary — inserted directly as
	// primary since there's nothing else to conflict with the partial
	// unique index. Otherwise insert as non-primary and let the
	// req.IsPrimary branch below do the transactional swap.
	isFirstAddress := false
	if _, err := h.Queries.GetMostRecentAddress(c.Request.Context(), customerID); errors.Is(err, pgx.ErrNoRows) {
		isFirstAddress = true
	}

	created, err := h.Queries.CreateAddress(c.Request.Context(), db.CreateAddressParams{
		CustomerID: customerID,
		Label:      req.Label,
		Address:    req.Address,
		ProvinceID: req.ProvinceID,
		CityID:     req.CityID,
		DistrictID: req.DistrictID,
		PostalCode: pgtype.Text{String: req.PostalCode, Valid: req.PostalCode != ""},
		Latitude:   latitude,
		Longitude:  longitude,
		IsPrimary:  isFirstAddress,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toAddressResponse(
		created.ID, created.Label, created.Address,
		created.ProvinceID, created.CityID, created.DistrictID,
		created.ProvinceName, created.CityName, created.DistrictName,
		created.PostalCode, created.Latitude, created.Longitude, created.IsPrimary,
	)

	if !isFirstAddress && req.IsPrimary {
		if err := h.setPrimaryAddress(c.Request.Context(), customerID, created.ID); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}

		refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
			ID:         created.ID,
			CustomerID: customerID,
		})
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		resp = toAddressResponse(
			refetched.ID, refetched.Label, refetched.Address,
			refetched.ProvinceID, refetched.CityID, refetched.DistrictID,
			refetched.ProvinceName, refetched.CityName, refetched.DistrictName,
			refetched.PostalCode, refetched.Latitude, refetched.Longitude, refetched.IsPrimary,
		)
	}

	resp.Message = "address created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAddresses(c *gin.Context) {
	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	addresses, err := h.Queries.ListAddresses(c.Request.Context(), customerID)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	responses := make([]AddressResponse, 0, len(addresses))
	for _, a := range addresses {
		responses = append(responses, toAddressResponse(
			a.ID, a.Label, a.Address,
			a.ProvinceID, a.CityID, a.DistrictID,
			a.ProvinceName, a.CityName, a.DistrictName,
			a.PostalCode, a.Latitude, a.Longitude, a.IsPrimary,
		))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) GetAddressByID(c *gin.Context) {
	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_address_id")
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	address, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "address_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toAddressResponse(
		address.ID, address.Label, address.Address,
		address.ProvinceID, address.CityID, address.DistrictID,
		address.ProvinceName, address.CityName, address.DistrictName,
		address.PostalCode, address.Latitude, address.Longitude, address.IsPrimary,
	))
}

func (h *Handler) UpdateAddress(c *gin.Context) {
	var req AddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_address_id")
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	latitude, err := float64ToNumeric(req.Latitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	longitude, err := float64ToNumeric(req.Longitude)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.Queries.UpdateAddress(c.Request.Context(), db.UpdateAddressParams{
		Label:      req.Label,
		Address:    req.Address,
		ProvinceID: req.ProvinceID,
		CityID:     req.CityID,
		DistrictID: req.DistrictID,
		PostalCode: pgtype.Text{String: req.PostalCode, Valid: req.PostalCode != ""},
		Latitude:   latitude,
		Longitude:  longitude,
		ID:         addressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "address_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toAddressResponse(
		updated.ID, updated.Label, updated.Address,
		updated.ProvinceID, updated.CityID, updated.DistrictID,
		updated.ProvinceName, updated.CityName, updated.DistrictName,
		updated.PostalCode, updated.Latitude, updated.Longitude, updated.IsPrimary,
	)

	if req.IsPrimary {
		if err := h.setPrimaryAddress(c.Request.Context(), customerID, addressID); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}

		refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
			ID:         addressID,
			CustomerID: customerID,
		})
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		resp = toAddressResponse(
			refetched.ID, refetched.Label, refetched.Address,
			refetched.ProvinceID, refetched.CityID, refetched.DistrictID,
			refetched.ProvinceName, refetched.CityName, refetched.DistrictName,
			refetched.PostalCode, refetched.Latitude, refetched.Longitude, refetched.IsPrimary,
		)
	}

	resp.Message = "address updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SetPrimaryAddress(c *gin.Context) {
	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_address_id")
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if _, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	}); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "address_not_found")
		return
	} else if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := h.setPrimaryAddress(c.Request.Context(), customerID, addressID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toAddressResponse(
		refetched.ID, refetched.Label, refetched.Address,
		refetched.ProvinceID, refetched.CityID, refetched.DistrictID,
		refetched.ProvinceName, refetched.CityName, refetched.DistrictName,
		refetched.PostalCode, refetched.Latitude, refetched.Longitude, refetched.IsPrimary,
	))
}

func (h *Handler) DeleteAddress(c *gin.Context) {
	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_address_id")
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	existing, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "address_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := h.Queries.DeleteAddress(c.Request.Context(), db.DeleteAddressParams{
		ID:         addressID,
		CustomerID: customerID,
	}); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if existing.IsPrimary {
		mostRecent, err := h.Queries.GetMostRecentAddress(c.Request.Context(), customerID)
		if err == nil {
			if err := h.setPrimaryAddress(c.Request.Context(), customerID, mostRecent.ID); err != nil {
				apperr.RespondInternalError(c, err)
				return
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "address deleted successfully"})
}
