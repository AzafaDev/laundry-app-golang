package clothingtype

import (
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

func toClothingTypeResponse(id pgtype.UUID, name string, isActive bool) ClothingTypeResponse {
	return ClothingTypeResponse{
		ID:       id.String(),
		Name:     name,
		IsActive: isActive,
	}
}
