package marketplace

import (
	"errors"
	"fmt"

	"github.com/baskararestu/wms-api/internal/pkg/middleware"
	"github.com/baskararestu/wms-api/internal/pkg/response"
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
	protected.Get("/shops/connect/start", h.StartLinkShop)
	protected.Get("/shops/:shopID", h.GetShopDetail)
	protected.Get("/shops/:shopID/logistic/channels", h.GetLogisticChannels)
	protected.Get("/webhooks/metrics", h.GetWebhookMetrics)
}

// StartLinkShop godoc
// @Summary Connect marketplace shop
// @Description Start the one-step marketplace shop linking flow and persist credentials.
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=LinkShopStartResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Router /api/integrations/marketplace/shops/connect/start [get]
func (h *Handler) StartLinkShop(c *fiber.Ctx) error {
	userID := fmt.Sprint(c.Locals("user_id"))
	if userID == "" || userID == "<nil>" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: "User ID not found in token",
		})
	}

	res, err := h.service.StartLinkShop(userID)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Failed to start link shop")
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

// OAuthCallback godoc
// @Summary Complete OAuth callback
// @Description Complete marketplace OAuth callback using code, shop ID, and state query parameters.
// @Tags Marketplace
// @Produce json
// @Param code query string true "Authorization code"
// @Param shop_id query string true "Marketplace shop ID"
// @Param state query string true "OAuth state"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/integrations/marketplace/oauth/callback [get]
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

// GetShopDetail godoc
// @Summary Get connected shop detail
// @Description Retrieve marketplace shop metadata for a connected shop.
// @Tags Marketplace
// @Security BearerAuth
// @Produce json
// @Param shopID path string true "Marketplace shop ID"
// @Success 200 {object} response.SuccessResponse{data=ShopDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Router /api/integrations/marketplace/shops/{shopID} [get]
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

// GetLogisticChannels godoc
// @Summary Get logistic channels
// @Description Fetch available logistic channels for a connected marketplace shop.
// @Tags Marketplace
// @Security BearerAuth
// @Produce json
// @Param shopID path string true "Marketplace shop ID"
// @Success 200 {object} LogisticChannelsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Router /api/integrations/marketplace/shops/{shopID}/logistic/channels [get]
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

// GetWebhookMetrics godoc
// @Summary Get webhook delivery metrics
// @Description Retrieve current outbound webhook delivery counters and retry stats.
// @Tags Marketplace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=WebhookDeliveryMetrics}
// @Failure 401 {object} response.ErrorResponse
// @Router /api/integrations/marketplace/webhooks/metrics [get]
func (h *Handler) GetWebhookMetrics(c *fiber.Ctx) error {
	metrics := h.service.GetWebhookDeliveryMetrics()

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Webhook delivery metrics",
		Data:    metrics,
	})
}
