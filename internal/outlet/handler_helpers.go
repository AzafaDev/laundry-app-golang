package outlet

import (
	"laundry-app-with-golang/internal/apphelper"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit       = 50
	maxPageLimit           = 100
	defaultServiceRadiusKM = 10.0
)

func toOutletResponse(id pgtype.UUID, name, address string, latitude, longitude pgtype.Numeric, isActive bool, serviceRadiusKM pgtype.Numeric) OutletResponse {
	return OutletResponse{
		ID:              id.String(),
		Name:            name,
		Address:         address,
		Latitude:        apphelper.NumericToFloat64(latitude),
		Longitude:       apphelper.NumericToFloat64(longitude),
		IsActive:        isActive,
		ServiceRadiusKM: apphelper.NumericToFloat64(serviceRadiusKM),
	}
}
