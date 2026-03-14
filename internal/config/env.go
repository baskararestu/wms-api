package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// AppConfig holds all strongly-typed environment variables
type Config struct {
	AppEnv               string
	JWTSecret            string
	DatabaseHost         string
	DatabaseUser         string
	DatabasePassword     string
	DatabaseName         string
	DatabasePort         string
	MarketplaceBaseURL   string
	PartnerID            string
	PartnerKey           string
	IsDevelopment        bool
}

// Global configuration instance
var App Config

// LoadConfig loads environment variables and populates the global App struct.
func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	App = Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		JWTSecret:          getEnv("JWT_SECRET", "supersecretkey"),
		DatabaseHost:       getEnv("DB_HOST", "localhost"),
		DatabaseUser:       getEnv("DB_USER", "postgres"),
		DatabasePassword:   getEnv("DB_PASSWORD", "postgres123"),
		DatabaseName:       getEnv("DB_NAME", "wms_db"),
		DatabasePort:       getEnv("DB_PORT", "5432"),
		MarketplaceBaseURL: getEnvRequired("MARKETPLACE_BASE_URL"),
		PartnerID:          getEnvRequired("PARTNER_ID"),
		PartnerKey:         getEnvRequired("PARTNER_KEY"),
	}

	App.IsDevelopment = App.AppEnv != "production"

	return nil
}

// getEnv retrieves an environment variable or returns a fallback value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvRequired retrieves an environment variable and crashes if it is missing
func getEnvRequired(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		log.Fatalf("Fatal: required environment variable %s is not set", key)
	}
	return value
}
