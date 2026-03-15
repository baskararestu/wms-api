package main

import (
	"strings"
	"time"

	appInit "github.com/baskararestu/wms-api/internal/app"
	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/database"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/baskararestu/wms-api/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Initialize Config, DB, and Redis
	config.LoadConfig()
	xlogger.Setup(config.App)
	database.ConnectDB()
	redis.ConnectRedis()

	app := fiber.New(fiber.Config{
		AppName: "WMS API",
	})

	allowedOrigins := []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://[::1]:5173",
		config.App.FrontendURL,
	}

	if config.App.FrontendURL != "" {
		frontendURL := strings.TrimRight(strings.TrimSpace(config.App.FrontendURL), "/")
		if frontendURL != "" {
			allowedOrigins = append(allowedOrigins, frontendURL)
		}
	}

	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	uniqOrigins := make([]string, 0, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := strings.TrimRight(strings.TrimSpace(origin), "/")
		if normalized == "" {
			continue
		}
		if _, exists := allowedOriginSet[normalized]; exists {
			continue
		}
		allowedOriginSet[normalized] = struct{}{}
		uniqOrigins = append(uniqOrigins, normalized)
	}

	// Middleware
	app.Use(recover.New())
	app.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			xlogger.Logger.Warn().
				Str("ip", c.IP()).
				Str("path", c.Path()).
				Msg("Rate limit exceeded")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    fiber.StatusTooManyRequests,
				"message": "Too many requests, please slow down",
			})
		},
	}))
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(uniqOrigins, ","),
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "ok",
			"message": "WMS API is alive",
			"uptime": time.Now().Format(time.RFC3339),
		})
	})

	// Wire and Run Domain modules
	appInit.Run(app, database.DB)

	if err := app.Listen(":3000"); err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Failed to start server")
	}
}
