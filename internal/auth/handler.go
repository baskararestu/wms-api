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
	// Post /api/v1/auth/login
	router.Post("/login", validation.New[LoginRequest](), h.Login)
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
