package auth

import (
	"log/slog"

	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
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
	// Post /api/v1/auth/register (Helpful for seeding tests, typically not public but useful here)
	router.Post("/register", validation.New[LoginRequest](), h.Register)
}

// Login handles the user authentication and token generation
func (h *Handler) Login(c *fiber.Ctx) error {
	// The validation middleware safely injected our strongly-typed struct
	req := c.Locals("payload").(*LoginRequest)

	res, err := h.service.Login(*req)
	if err != nil {
		slog.Warn("Login failed", "username", req.Username, "error", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	slog.Info("User logged in successfully", "username", req.Username)
	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Login successful",
		Data:    res,
	})
}

// Register handles creation of a new internal user
func (h *Handler) Register(c *fiber.Ctx) error {
	req := c.Locals("payload").(*LoginRequest)

	err := h.service.Register(*req)
	if err != nil {
		slog.Warn("Registration failed", "username", req.Username, "error", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	slog.Info("New user registered", "username", req.Username)
	return c.Status(fiber.StatusCreated).JSON(response.SuccessResponse{
		Code:    fiber.StatusCreated,
		Message: "User registered successfully",
	})
}
