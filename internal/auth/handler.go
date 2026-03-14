package auth

import "github.com/gofiber/fiber/v2"

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
	// Example: router.Post("/login", h.Login)
}
