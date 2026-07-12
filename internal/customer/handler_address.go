package customer

import (
	"errors"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	created, err := h.Queries.CreateAddress(c.Request.Context(), db.CreateAddressParams{
		CustomerID: customerID,
		Label:      req.Label,
		Address:    req.Address,
		Province:   req.Province,
		City:       req.City,
		District:   req.District,
		PostalCode: pgtype.Text{String: req.PostalCode, Valid: req.PostalCode != ""},
		Latitude:   latitude,
		Longitude:  longitude,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := created
	if req.IsPrimary {
		if err := h.setPrimaryAddress(c.Request.Context(), customerID, created.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
			ID:         created.ID,
			CustomerID: customerID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		result = refetched
	}

	resp := toAddressResponse(result)
	resp.Message = "address created successfully"
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAddresses(c *gin.Context) {
	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	addresses, err := h.Queries.ListAddresses(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]AddressResponse, 0, len(addresses))
	for _, a := range addresses {
		responses = append(responses, toAddressResponse(a))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) UpdateAddress(c *gin.Context) {
	var req AddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address id"})
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		Province:   req.Province,
		City:       req.City,
		District:   req.District,
		PostalCode: pgtype.Text{String: req.PostalCode, Valid: req.PostalCode != ""},
		Latitude:   latitude,
		Longitude:  longitude,
		ID:         addressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := updated
	if req.IsPrimary {
		if err := h.setPrimaryAddress(c.Request.Context(), customerID, addressID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
			ID:         addressID,
			CustomerID: customerID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		result = refetched
	}

	resp := toAddressResponse(result)
	resp.Message = "address updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SetPrimaryAddress(c *gin.Context) {
	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address id"})
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	}); errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.setPrimaryAddress(c.Request.Context(), customerID, addressID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	refetched, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toAddressResponse(refetched))
}

func (h *Handler) DeleteAddress(c *gin.Context) {
	var addressID pgtype.UUID
	if err := addressID.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address id"})
		return
	}

	customerID, _, err := h.currentCustomerID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Queries.GetAddressByID(c.Request.Context(), db.GetAddressByIDParams{
		ID:         addressID,
		CustomerID: customerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.Queries.DeleteAddress(c.Request.Context(), db.DeleteAddressParams{
		ID:         addressID,
		CustomerID: customerID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if existing.IsPrimary {
		mostRecent, err := h.Queries.GetMostRecentAddress(c.Request.Context(), customerID)
		if err == nil {
			if err := h.setPrimaryAddress(c.Request.Context(), customerID, mostRecent.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "address deleted successfully"})
}
