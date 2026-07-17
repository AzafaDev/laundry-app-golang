package laundryitem

import (
	"laundry-app-with-golang/internal/apphelper"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
	defaultUnit      = "pcs"
	maxBasePrice     = 99_999_999.99
)

var validUnits = map[string]bool{
	"pcs": true,
	"kg":  true,
}

func toLaundryItemResponse(id pgtype.UUID, name string, description pgtype.Text, unit string, basePrice pgtype.Numeric, isActive bool) LaundryItemResponse {
	return LaundryItemResponse{
		ID:          id.String(),
		Name:        name,
		Description: description.String,
		Unit:        unit,
		BasePrice:   apphelper.NumericToFloat64(basePrice),
		IsActive:    isActive,
	}
}

func toPublicLaundryItemResponse(id pgtype.UUID, name string, description pgtype.Text, unit string, basePrice pgtype.Numeric) PublicLaundryItemResponse {
	return PublicLaundryItemResponse{
		ID:          id.String(),
		Name:        name,
		Description: description.String,
		Unit:        unit,
		BasePrice:   apphelper.NumericToFloat64(basePrice),
	}
}
