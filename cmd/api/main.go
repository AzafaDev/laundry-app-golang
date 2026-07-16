package main

import (
	"context"
	"laundry-app-with-golang/internal/clothingtype"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/customer"
	"laundry-app-with-golang/internal/database"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/email"
	"laundry-app-with-golang/internal/employee"
	"laundry-app-with-golang/internal/geocode"
	"laundry-app-with-golang/internal/laundryitem"
	oauthpkg "laundry-app-with-golang/internal/oauth"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/outlet"
	"laundry-app-with-golang/internal/server"
	"laundry-app-with-golang/internal/storage"
	"laundry-app-with-golang/internal/wilayah"
	"log"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	emailClient := email.NewClient(cfg.ResendAPIKey, cfg.AppBaseURL, cfg.FrontendURL)

	storageClient, err := storage.NewClient(cfg.CloudinaryURL)
	if err != nil {
		log.Fatalf("failed to init cloudinary client: %v", err)
	}

	googleClient := oauthpkg.NewGoogleClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.AppBaseURL)
	geocodeClient := geocode.NewClient(cfg.OpenCageAPIKey)

	customerHandler := customer.NewHandler(queries, pool, cfg, emailClient, storageClient, googleClient, geocodeClient)
	employeeHandler := employee.NewHandler(queries, pool, cfg, emailClient, geocodeClient)
	wilayahHandler := wilayah.NewHandler(queries)
	outletHandler := outlet.NewHandler(queries)
	orderHandler := order.NewHandler(pool, queries)
	laundryItemHandler := laundryitem.NewHandler(queries)
	clothingTypeHandler := clothingtype.NewHandler(queries)

	router := server.NewRouter(customerHandler, employeeHandler, wilayahHandler, outletHandler, orderHandler, laundryItemHandler, clothingTypeHandler, cfg, queries)
	port := ":" + cfg.Port

	log.Println("connected to database successfully")

	if err := router.Run(port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
