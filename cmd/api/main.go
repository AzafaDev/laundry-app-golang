package main

import (
	"laundry-app-with-golang/internal/app"
	"laundry-app-with-golang/internal/config"
	"log"
)

func main() {
	cfg := config.Load()

	router, pool, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to start app: %v", err)
	}
	defer pool.Close()

	log.Println("connected to database successfully")

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
