package customer

import (
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/email"
)

type Handler struct {
	Queries     *db.Queries
	Config      config.Config
	emailClient *email.Client
}

func NewHandler(queries *db.Queries, cfg config.Config, email *email.Client) *Handler {
	return &Handler{
		Queries:     queries,
		Config:      cfg,
		emailClient: email,
	}
}
