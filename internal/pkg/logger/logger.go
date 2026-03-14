package logger

import (
	"log/slog"
	"os"

	"github.com/baskararestu/wms-api/internal/config"
)

// InitLogger initializes the global structured logger (slog)
func InitLogger() {
	env := config.GetEnv("APP_ENV", "development")

	var handler slog.Handler
	if env == "production" {
		// JSON format for production (easy to parse by Datadog, ELK, etc.)
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		// Human-readable format for local development
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	
	slog.Info("Structured logger initialized", "env", env)
}
