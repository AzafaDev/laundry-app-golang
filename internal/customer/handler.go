package customer

import (
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/email"
	oauthpkg "laundry-app-with-golang/internal/oauth"
	"laundry-app-with-golang/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Queries       *db.Queries
	Pool          *pgxpool.Pool
	Config        config.Config
	emailClient   *email.Client
	storageClient *storage.Client
	googleClient  *oauthpkg.Client
}

func NewHandler(queries *db.Queries, pool *pgxpool.Pool, cfg config.Config, email *email.Client, storageClient *storage.Client, googleClient *oauthpkg.Client) *Handler {
	return &Handler{
		Queries:       queries,
		Pool:          pool,
		Config:        cfg,
		emailClient:   email,
		storageClient: storageClient,
		googleClient:  googleClient,
	}
}
