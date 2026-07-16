package payment

import (
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Config  config.Config
}

func NewHandler(pool *pgxpool.Pool, queries *db.Queries, cfg config.Config) *Handler {
	return &Handler{Pool: pool, Queries: queries, Config: cfg}
}
