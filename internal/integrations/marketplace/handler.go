package marketplace

import (
	"log/slog"

	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service Service
}

// NewHandler creates a new Marketplace Handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes binds the handler methods to the Fiber router
func (h *Handler) RegisterRoutes(router fiber.Router) {
	// The frontend will hit this endpoint to initiate the OAuth flow behind the scenes
	router.Post("/link-shop", validation.New[LinkShopRequest](), h.LinkShop)
}

// LinkShop handles the request to link a new shop via OAuth
func (h *Handler) LinkShop(c *fiber.Ctx) error {
	req := c.Locals("payload").(*LinkShopRequest)

	err := h.service.LinkShop(req.ShopID)
	if err != nil {
		slog.Warn("Link Shop failed", "shop_id", req.ShopID, "error", err.Error())
		return c.Status(fiber.StatusBadGateway).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadGateway,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Shop Successfully Linked to WMS",
	})
}
