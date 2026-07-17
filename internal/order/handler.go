package order

import (
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool          *pgxpool.Pool
	Queries       *db.Queries
	StorageClient *storage.Client
}

func NewHandler(pool *pgxpool.Pool, queries *db.Queries, storageClient *storage.Client) *Handler {
	return &Handler{Pool: pool, Queries: queries, StorageClient: storageClient}
}
