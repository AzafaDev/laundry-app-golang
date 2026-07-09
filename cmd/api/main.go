package main

import (
	"context"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/database"
	db "laundry-app-with-golang/internal/db/generated"
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

	queris := db.New(pool)
	_ = queris

	log.Println("connected to database successfully")
}
