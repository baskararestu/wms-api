package auth

import (
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the Auth domain
type Handler struct {
	service Service
}

// NewHandler creates a new Auth Handler instance
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes binds the handler methods to the Fiber router
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/login", validation.New[LoginRequest](), h.Login)
	router.Post("/refresh", validation.New[RefreshTokenRequest](), h.RefreshToken)
	router.Post("/logout", validation.New[LogoutRequest](), h.Logout)
}

// Login handles the user authentication and token generation
func (h *Handler) Login(c *fiber.Ctx) error {
	// The validation middleware safely injected our strongly-typed struct
	req := c.Locals("payload").(*LoginRequest)

	res, err := h.service.Login(*req)
	if err != nil {
		xlogger.Logger.Warn().Str("email", req.Email).Err(err).Msg("Login failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	xlogger.Logger.Info().Str("email", req.Email).Msg("User logged in successfully")
	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Login successful",
		Data:    res,
	})
}

// RefreshToken handles access token renewal using a refresh token
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	req := c.Locals("payload").(*RefreshTokenRequest)

	res, err := h.service.RefreshToken(*req)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Refresh token failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Token refreshed successfully",
		Data:    res,
	})
}

// Logout invalidates a refresh token session
func (h *Handler) Logout(c *fiber.Ctx) error {
	req := c.Locals("payload").(*LogoutRequest)

	err := h.service.Logout(*req)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Logout failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Logout successful",
	})
}
