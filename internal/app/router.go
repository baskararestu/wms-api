package app

import (
	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/integrations/marketplace"
	"github.com/baskararestu/wms-api/internal/orders"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Run starts the application and handles DI wiring
func Run(app *fiber.App, db *gorm.DB) {
	xlogger.Logger.Info().Msg("Initializing Domain Modules...")

	// 1. Initialize Repositories
	authRepo := auth.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	marketplaceRepo := marketplace.NewRepository(db)

	// 2. Initialize Services (DI happens here)
	authService := auth.NewService(authRepo)

	// --- Integrations / Marketplace Domain ---
	marketplaceClient := marketplace.NewClient()
	marketplaceService := marketplace.NewService(marketplaceClient, marketplaceRepo, config.App.RedirectURL)
	marketplace.StartWebhookRetryScheduler(marketplaceService)

	// Orders domain uses marketplace service for Sync functionality
	ordersService := orders.NewService(ordersRepo, marketplaceService)

	// 3. Initialize Handlers
	authHandler := auth.NewHandler(authService)
	ordersHandler := orders.NewHandler(ordersService)
	marketplaceHandler := marketplace.NewHandler(marketplaceService)

	// 4. Register Routes
	api := app.Group("/api")
	authHandler.RegisterRoutes(api.Group("/auth"))
	ordersHandler.RegisterRoutes(api.Group("/orders"))
	marketplaceHandler.RegisterRoutes(api.Group("/integrations/marketplace"))

	xlogger.Logger.Info().Msg("✅ Domain Modules Loaded and Routes Registered")
}
