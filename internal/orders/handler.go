package orders

import "github.com/gofiber/fiber/v2"

// Handler handles HTTP requests for the Orders domain
type Handler struct {
	service Service
}

// NewHandler creates a new Orders Handler instance
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes binds the handler methods to the Fiber router
func (h *Handler) RegisterRoutes(router fiber.Router) {
	// Example: router.Post("/orders/sync", h.SyncWebhook)
	// Example: router.Patch("/orders/:id/pick", h.MarkAsPicking)
}
