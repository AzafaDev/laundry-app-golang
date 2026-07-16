package notification

import (
	db "laundry-app-with-golang/internal/db/generated"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
}

func NewHandler(pool *pgxpool.Pool, queries *db.Queries) *Handler {
	return &Handler{Pool: pool, Queries: queries}
}
