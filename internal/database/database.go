package database

import (
	"fmt"
	"log"
	"time"

	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/orders"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectDB establishes the connection to the PostgreSQL database
func ConnectDB() {
	host := config.GetEnv("DB_HOST", "localhost")
	user := config.GetEnv("DB_USER", "postgres")
	password := config.GetEnv("DB_PASSWORD", "postgres123")
	dbname := config.GetEnv("DB_NAME", "wms_db")
	port := config.GetEnv("DB_PORT", "5432")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		host, user, password, dbname, port,
	)

	var err error
	
	// Add retry logic for docker compose startup delays
	for i := 0; i < 5; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database. Retrying in 2 seconds... (%d/5)\n", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Fatal: Could not connect to database after 5 retries: %v\n", err)
	}

	log.Println("✅ Successfully connected to PostgreSQL Database!")

	// Auto Migrate the schemas
	err = DB.AutoMigrate(
		&auth.User{},
		&auth.MarketplaceCredential{},
		&orders.Order{},
		&orders.OrderItem{},
	)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v\n", err)
	}
	
	log.Println("✅ Database Migration completed successfully!")
}
