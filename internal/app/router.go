package app

import (
	"log"

	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/integrations/marketplace"
	"github.com/baskararestu/wms-api/internal/orders"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Run starts the application and handles DI wiring
func Run(app *fiber.App, db *gorm.DB) {
	log.Println("Initializing Domain Modules...")

	// 1. Initialize Repositories
	authRepo := auth.NewRepository(db)
	ordersRepo := orders.NewRepository(db)

	// 2. Initialize Services (DI happens here)
	authService := auth.NewService(authRepo)
	
	// Example: Since Orders needs Auth functionality, we inject the authService
	// assuming auth.Service satisfies the orders.AuthProvider interface.
	ordersService := orders.NewService(ordersRepo) // Using ordersRepo inside orders service

	// --- Integrations / Marketplace Domain ---
	marketplaceClient := marketplace.NewClient()
	marketplaceService := marketplace.NewService(marketplaceClient, authRepo)
	marketplaceHandler := marketplace.NewHandler(marketplaceService)

	// 3. Initialize Handlers
	authHandler := auth.NewHandler(authService)
	ordersHandler := orders.NewHandler(ordersService)

	// 4. Register Routes
	api := app.Group("/api/v1")
	authHandler.RegisterRoutes(api.Group("/auth"))
	ordersHandler.RegisterRoutes(api.Group("/orders"))
	marketplaceHandler.RegisterRoutes(api.Group("/integrations/marketplace"))

	log.Println("✅ Domain Modules Loaded and Routes Registered")
}
