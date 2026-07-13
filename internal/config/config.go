package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTAccessSecret    string
	JWTRefreshSecret   string
	GoEnv              string
	ResendAPIKey       string
	AppBaseURL         string
	CloudinaryURL      string
	GoogleClientID     string
	GoogleClientSecret string
	FrontendURL        string
	OpenCageAPIKey     string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on real environment variables")
	}

	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        mustGetEnv("DATABASE_URL"),
		JWTAccessSecret:    mustGetEnv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:   mustGetEnv("JWT_REFRESH_SECRET"),
		GoEnv:              getEnv("GO_ENV", "development"),
		ResendAPIKey:       mustGetEnv("RESEND_API_KEY"),
		AppBaseURL:         getEnv("APP_BASE_URL", "http://localhost:8080"),
		CloudinaryURL:      mustGetEnv("CLOUDINARY_URL"),
		GoogleClientID:     mustGetEnv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: mustGetEnv("GOOGLE_CLIENT_SECRET"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		OpenCageAPIKey:     mustGetEnv("OPENCAGE_API_KEY"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}
