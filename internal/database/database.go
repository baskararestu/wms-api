package database

import (
	"fmt"
	"time"

	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/integrations/marketplace"
	"github.com/baskararestu/wms-api/internal/orders"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	usershops "github.com/baskararestu/wms-api/internal/user-shops"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectDB establishes the connection to the PostgreSQL database
func ConnectDB() {
	host := config.App.DatabaseHost
	user := config.App.DatabaseUser
	password := config.App.DatabasePassword
	dbname := config.App.DatabaseName
	port := config.App.DatabasePort

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
		xlogger.Logger.Warn().Msgf("Failed to connect to database. Retrying in 2 seconds... (%d/5)", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Could not connect to database after 5 retries")
	}

	xlogger.Logger.Info().Msg("✅ Successfully connected to PostgreSQL Database!")

	// Auto Migrate the schemas
	err = DB.AutoMigrate(
		&auth.User{},
		&auth.RefreshToken{},
		&usershops.UserShop{},
		&marketplace.MarketplaceCredential{},
		&orders.Order{},
		&orders.OrderItem{},
	)
	if err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Failed to run migrations")
	}
	xlogger.Logger.Info().Msg("✅ Database Migration completed successfully!")

	// Seed Defaults
	SeedAll(DB)
}
