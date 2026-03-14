package main

import (
	appInit "github.com/baskararestu/wms-api/internal/app"
	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/database"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Initialize Config and DB
	config.LoadConfig()
	xlogger.Setup(config.App)
	database.ConnectDB()

	app := fiber.New(fiber.Config{
		AppName: "WMS API",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "ok",
			"message": "WMS API is alive",
		})
	})

	// Wire and Run Domain modules
	appInit.Run(app, database.DB)

	if err := app.Listen(":3000"); err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Failed to start server")
	}
}
