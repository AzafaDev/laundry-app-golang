package employee

import (
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/email"
	"laundry-app-with-golang/internal/geocode"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Queries       *db.Queries
	Pool          *pgxpool.Pool
	Config        config.Config
	emailClient   *email.Client
	geocodeClient *geocode.Client
}

func NewHandler(queries *db.Queries, pool *pgxpool.Pool, cfg config.Config, emailClient *email.Client, geocodeClient *geocode.Client) *Handler {
	return &Handler{
		Queries:       queries,
		Pool:          pool,
		Config:        cfg,
		emailClient:   emailClient,
		geocodeClient: geocodeClient,
	}
}
