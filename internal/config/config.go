package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	JWTAccessSecret         string
	JWTEmployeeAccessSecret string
	GoEnv                   string
	ResendAPIKey            string
	AppBaseURL              string
	CloudinaryURL           string
	GoogleClientID          string
	GoogleClientSecret      string
	FrontendURL             string
	OpenCageAPIKey          string
	CheckinRadiusMeters     int
	LateThresholdMinutes    int
	MidtransServerKey       string
	MidtransClientKey       string
	MidtransIsProduction    bool
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on real environment variables")
	}

	return Config{
		Port:                    getEnv("PORT", "8080"),
		DatabaseURL:             mustGetEnv("DATABASE_URL"),
		JWTAccessSecret:         mustGetEnv("JWT_ACCESS_SECRET"),
		JWTEmployeeAccessSecret: mustGetEnv("JWT_EMPLOYEE_ACCESS_SECRET"),
		GoEnv:                   getEnv("GO_ENV", "development"),
		ResendAPIKey:            mustGetEnv("RESEND_API_KEY"),
		AppBaseURL:              getEnv("APP_BASE_URL", "http://localhost:8080"),
		CloudinaryURL:           mustGetEnv("CLOUDINARY_URL"),
		GoogleClientID:          mustGetEnv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:      mustGetEnv("GOOGLE_CLIENT_SECRET"),
		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:3000"),
		OpenCageAPIKey:          mustGetEnv("OPENCAGE_API_KEY"),
		CheckinRadiusMeters:     getEnvInt("CHECKIN_RADIUS_METERS", 500),
		LateThresholdMinutes:    getEnvInt("LATE_THRESHOLD_MINUTES", 30),
		MidtransServerKey:       mustGetEnv("MIDTRANS_SERVER_KEY"),
		MidtransClientKey:       mustGetEnv("MIDTRANS_CLIENT_KEY"),
		MidtransIsProduction:    getEnvBool("MIDTRANS_IS_PRODUCTION", false),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}
