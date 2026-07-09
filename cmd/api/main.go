package main

import (
	"context"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/database"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/server"
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
	_ = queries

	router := server.NewRouter()
	port := ":" + cfg.Port

	log.Println("connected to database successfully")

	if err := router.Run(port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
