package middleware

import (
	"strings"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Protected requires a valid JWT access token to proceed
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			xlogger.Logger.Warn().Msg("Missing Authorization header")
			return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
				Code:    fiber.StatusUnauthorized,
				Message: "Authorization is required",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			xlogger.Logger.Warn().Msg("Invalid Authorization header format")
			return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
				Code:    fiber.StatusUnauthorized,
				Message: "Invalid authorization format",
			})
		}

		tokenStr := parts[1]
		secret := []byte(config.App.JWTSecret)

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			// Validate the alg is what we expect
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return secret, nil
		})

		if err != nil || !token.Valid {
			xlogger.Logger.Warn().Err(err).Msg("Invalid or expired token")
			return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
				Code:    fiber.StatusUnauthorized,
				Message: "Invalid or expired token",
			})
		}

		// Optionally extract claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// For example, verify type if we appended "type": "access"
			if tokenType, exists := claims["type"]; !exists || tokenType != "access" {
				return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
					Code:    fiber.StatusUnauthorized,
					Message: "Invalid token type",
				})
			}
			c.Locals("user_id", claims["user_id"])
			c.Locals("email", claims["email"])
		}

		return c.Next()
	}
}
