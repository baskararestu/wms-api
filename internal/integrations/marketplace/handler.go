package marketplace

import (
	"errors"
	"fmt"

	"github.com/baskararestu/wms-api/internal/pkg/middleware"
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/oauth/callback", h.OAuthCallback)

	protected := router.Group("/", middleware.Protected())
	protected.Post("/shops/connect/start", validation.New[LinkShopRequest](), h.StartLinkShop)
	protected.Get("/shops/:shopID", h.GetShopDetail)
	protected.Get("/shops/:shopID/logistic/channels", h.GetLogisticChannels)
	protected.Get("/webhooks/metrics", h.GetWebhookMetrics)
}

func (h *Handler) StartLinkShop(c *fiber.Ctx) error {
	req := c.Locals("payload").(*LinkShopRequest)

	res, err := h.service.StartLinkShop(req.ShopID)
	if err != nil {
		xlogger.Logger.Warn().Str("shop_id", req.ShopID).Err(err).Msg("Failed to start link shop")
		if errors.Is(err, ErrMarketplaceUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.ErrorResponse{
				Code:    fiber.StatusServiceUnavailable,
				Message: err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Shop connected successfully",
		Data:    res,
	})
}

func (h *Handler) OAuthCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	shopID := c.Query("shop_id")
	state := c.Query("state")

	if code == "" || shopID == "" || state == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "missing required callback query params",
			Errors:  []string{"required: code, shop_id, state"},
		})
	}

	if err := h.service.CompleteLinkShop(code, shopID, state); err != nil {
		xlogger.Logger.Warn().Str("shop_id", shopID).Err(err).Msg("OAuth callback failed")
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Shop linked successfully",
	})
}

func (h *Handler) GetShopDetail(c *fiber.Ctx) error {
	shopID := c.Params("shopID")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "shopID is required",
		})
	}

	shopDetail, err := h.service.GetShopDetailByShopID(shopID)
	if err != nil {
		xlogger.Logger.Warn().Str("shop_id", shopID).Err(err).Msg("Failed to fetch shop detail")
		if errors.Is(err, ErrShopNotConnected) {
			return c.Status(fiber.StatusNotFound).JSON(response.ErrorResponse{
				Code:    fiber.StatusNotFound,
				Message: err.Error(),
			})
		}
		if errors.Is(err, ErrMarketplaceUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.ErrorResponse{
				Code:    fiber.StatusServiceUnavailable,
				Message: err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: fmt.Sprintf("Shop detail for %s", shopID),
		Data:    shopDetail,
	})
}

func (h *Handler) GetLogisticChannels(c *fiber.Ctx) error {
	shopID := c.Params("shopID")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "shopID is required",
		})
	}

	channels, err := h.service.GetLogisticChannelsByShopID(shopID)
	if err != nil {
		xlogger.Logger.Warn().Str("shop_id", shopID).Err(err).Msg("Failed to fetch logistic channels")
		if errors.Is(err, ErrShopNotConnected) {
			return c.Status(fiber.StatusNotFound).JSON(response.ErrorResponse{
				Code:    fiber.StatusNotFound,
				Message: err.Error(),
			})
		}
		if errors.Is(err, ErrMarketplaceUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.ErrorResponse{
				Code:    fiber.StatusServiceUnavailable,
				Message: err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	// We can manually forward the literal data structure or return the nested one
	return c.Status(fiber.StatusOK).JSON(channels)
}

func (h *Handler) GetWebhookMetrics(c *fiber.Ctx) error {
	metrics := h.service.GetWebhookDeliveryMetrics()

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Webhook delivery metrics",
		Data:    metrics,
	})
}
