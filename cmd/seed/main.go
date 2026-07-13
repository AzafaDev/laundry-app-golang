package main

import (
	"context"
	"log"
	"os"

	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/database"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	fullName := getEnv("SEED_ADMIN_NAME", "Super Admin")
	email := getEnv("SEED_ADMIN_EMAIL", "admin@laundry.test")
	password := getEnv("SEED_ADMIN_PASSWORD", "changeme123")

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	tag, err := pool.Exec(ctx, `
		INSERT INTO employees (full_name, email, password_hash, role, is_active)
		VALUES ($1, $2, $3, 'super_admin', TRUE)
		ON CONFLICT (email) DO NOTHING
	`, fullName, email, hash)
	if err != nil {
		log.Fatalf("failed to seed super_admin: %v", err)
	}

	if tag.RowsAffected() == 0 {
		log.Printf("employee with email %s already exists, skipped", email)
		return
	}

	log.Printf("seeded super_admin: %s", email)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
