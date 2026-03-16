package config

import (
	"errors"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// AppConfig holds all strongly-typed environment variables
type Config struct {
	AppEnv             string
	Port               string
	JWTSecret          string
	DatabaseHost       string
	DatabaseUser       string
	DatabasePassword   string
	DatabaseName       string
	DatabasePort       string
	MarketplaceBaseURL string
	PartnerID          string
	PartnerKey         string
	RedisHost          string
	RedisPort          string
	RedisPassword      string
	IsDevelopment      bool
	RedirectURL        string
	FrontendURL        string
}

// Global configuration instance
var App Config

// LoadConfig loads environment variables and populates the global App struct.
func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Info: .env file not found, using environment variables")
		} else {
			log.Printf("Warning: could not load .env file: %v", err)
		}
	}

	App = Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		Port:               getEnv("PORT", "3000"),
		JWTSecret:          getEnv("JWT_SECRET", "supersecretkey"),
		DatabaseHost:       getEnvRequired("DB_HOST"),
		DatabaseUser:       getEnvRequired("DB_USER"),
		DatabasePassword:   getEnv("DB_PASSWORD", ""),
		DatabaseName:       getEnvRequired("DB_NAME"),
		DatabasePort:       getEnvRequired("DB_PORT"),
		MarketplaceBaseURL: getEnvRequired("MARKETPLACE_BASE_URL"),
		PartnerID:          getEnvRequired("PARTNER_ID"),
		PartnerKey:         getEnvRequired("PARTNER_KEY"),
		RedisHost:          getEnv("REDIS_HOST", "localhost"),
		RedisPort:          getEnv("REDIS_PORT", "6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedirectURL:        getEnv("REDIRECT_URL", "https://example.com/callback"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
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
