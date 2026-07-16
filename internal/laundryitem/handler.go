package laundryitem

import (
	db "laundry-app-with-golang/internal/db/generated"
)

type Handler struct {
	Queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{Queries: queries}
}
