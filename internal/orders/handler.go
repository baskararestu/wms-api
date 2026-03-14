package orders

import (
	"github.com/baskararestu/wms-api/internal/pkg/middleware"
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	// Public or external routes
	router.Post("/webhook", validation.New[WebhookPayload](), h.HandleMarketplaceWebhook)

	// Protected routes that require a valid user token
	protected := router.Group("/", middleware.Protected())
	protected.Get("/", h.GetOrders)
	protected.Get("/:order_sn", h.GetOrderDetail)
	protected.Post("/:order_sn/pick", h.PickOrder)
	protected.Post("/:order_sn/pack", h.PackOrder)
	protected.Patch("/:id/wms-status", validation.New[UpdateWMSStatusRequest](), h.UpdateWMSStatus)
	protected.Post("/sync", validation.New[SyncOrdersRequest](), h.SyncOrders)
}

func (h *Handler) GetOrders(c *fiber.Ctx) error {
	var query GetOrderListQuery
	if err := c.QueryParser(&query); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid query parameters",
		})
	}

	res, err := h.service.GetOrders(query)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Failed to list orders")
		return c.Status(fiber.StatusInternalServerError).JSON(response.ErrorResponse{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to list orders",
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Orders retrieved successfully",
		Data:    res,
	})
}

func (h *Handler) GetOrderDetail(c *fiber.Ctx) error {
	orderSN := c.Params("order_sn")
	if orderSN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Order SN is required",
		})
	}

	res, err := h.service.GetOrderDetail(orderSN)
	if err != nil {
		xlogger.Logger.Warn().Str("order_sn", orderSN).Err(err).Msg("Order not found")
		return c.Status(fiber.StatusNotFound).JSON(response.ErrorResponse{
			Code:    fiber.StatusNotFound,
			Message: "Order not found",
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Order retrieved successfully",
		Data:    res,
	})
}

func (h *Handler) UpdateWMSStatus(c *fiber.Ctx) error {
	idVal := c.Params("id")
	orderID, err := uuid.Parse(idVal)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid order ID format",
		})
	}

	req := c.Locals("payload").(*UpdateWMSStatusRequest)
	err = h.service.UpdateWMSStatus(orderID, req.Status)
	if err != nil {
		xlogger.Logger.Warn().Str("id", idVal).Str("status", req.Status).Err(err).Msg("Failed to update status")
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "WMS status updated successfully",
	})
}

func (h *Handler) PickOrder(c *fiber.Ctx) error {
	orderSN := c.Params("order_sn")
	if orderSN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Order SN is required",
		})
	}

	err := h.service.PickOrder(orderSN)
	if err != nil {
		xlogger.Logger.Warn().Str("order_sn", orderSN).Err(err).Msg("Failed to pick order")
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Order picked successfully",
	})
}

func (h *Handler) PackOrder(c *fiber.Ctx) error {
	orderSN := c.Params("order_sn")
	if orderSN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Order SN is required",
		})
	}

	err := h.service.PackOrder(orderSN)
	if err != nil {
		xlogger.Logger.Warn().Str("order_sn", orderSN).Err(err).Msg("Failed to pack order")
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Order packed successfully",
	})
}

func (h *Handler) SyncOrders(c *fiber.Ctx) error {
	req := c.Locals("payload").(*SyncOrdersRequest)

	err := h.service.SyncMarketplaceOrders(req.ShopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ErrorResponse{
			Code:    fiber.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Sync process started",
	})
}

func (h *Handler) HandleMarketplaceWebhook(c *fiber.Ctx) error {
	req := c.Locals("payload").(*WebhookPayload)

	err := h.service.ProcessWebhook(*req)
	if err != nil {
		xlogger.Logger.Error().Err(err).Msg("Failed to process webhook")
		return c.Status(fiber.StatusInternalServerError).JSON(response.ErrorResponse{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to process webhook",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "success"})
}
