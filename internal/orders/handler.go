package orders

import (
	"errors"

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
	// Protected routes that require a valid user token
	protected := router.Group("/", middleware.Protected())
	protected.Get("/", h.GetOrders)
	protected.Get("/:order_sn", h.GetOrderDetail)
	protected.Post("/:order_sn/pick", h.PickOrder)
	protected.Post("/:order_sn/pack", h.PackOrder)
	protected.Post("/:order_sn/ship", validation.New[ShipOrderRequest](), h.ShipOrder)
	protected.Patch("/:id/wms-status", validation.New[UpdateWMSStatusRequest](), h.UpdateWMSStatus)
	protected.Post("/sync", validation.New[SyncOrdersRequest](), h.SyncOrders)
}

// GetOrders godoc
// @Summary List orders
// @Description Get paginated orders with optional WMS, marketplace, shipping, shop, and since filters.
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" minimum(1)
// @Param limit query int false "Items per page" minimum(1) maximum(100)
// @Param sort_by query string false "Field to sort by"
// @Param sort_dir query string false "Sort direction" Enums(asc,desc)
// @Param search query string false "Search term"
// @Param wms_status query string false "WMS status"
// @Param marketplace_status query string false "Marketplace status"
// @Param shipping_status query string false "Shipping status"
// @Param shop_id query string false "Marketplace shop ID"
// @Param since query string false "Updated since timestamp"
// @Success 200 {object} response.SuccessResponse{data=OrderListResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/orders/ [get]
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
		if errors.Is(err, ErrInvalidSince) {
			return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			})
		}

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

// GetOrderDetail godoc
// @Summary Get order detail
// @Description Retrieve a single order by marketplace order serial number.
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Param order_sn path string true "Marketplace order serial number"
// @Success 200 {object} response.SuccessResponse{data=OrderDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/orders/{order_sn} [get]
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

// UpdateWMSStatus godoc
// @Summary Update WMS status
// @Description Manually update the WMS status for an order by internal UUID.
// @Tags Orders
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Internal order UUID"
// @Param request body UpdateWMSStatusRequest true "WMS status payload"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/orders/{id}/wms-status [patch]
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

// PickOrder godoc
// @Summary Pick order
// @Description Move an order from ready-to-pick into picking.
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Param order_sn path string true "Marketplace order serial number"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/orders/{order_sn}/pick [post]
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

// PackOrder godoc
// @Summary Pack order
// @Description Move an order from picking into packed.
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Param order_sn path string true "Marketplace order serial number"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/orders/{order_sn}/pack [post]
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

// ShipOrder godoc
// @Summary Ship order
// @Description Finalize shipping, request tracking data from marketplace, and trigger outbound webhooks.
// @Tags Orders
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param order_sn path string true "Marketplace order serial number"
// @Param request body ShipOrderRequest true "Shipping payload"
// @Success 200 {object} response.SuccessResponse{data=ShipOrderResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/orders/{order_sn}/ship [post]
func (h *Handler) ShipOrder(c *fiber.Ctx) error {
	orderSN := c.Params("order_sn")
	if orderSN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: "Order SN is required",
		})
	}

	req := c.Locals("payload").(*ShipOrderRequest)

	res, err := h.service.ShipOrder(orderSN, req.ChannelID)
	if err != nil {
		xlogger.Logger.Warn().Str("order_sn", orderSN).Err(err).Msg("Failed to ship order")
		return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Order shipped successfully",
		Data:    res,
	})
}

// SyncOrders godoc
// @Summary Sync marketplace orders
// @Description Pull orders from the connected marketplace shop and upsert them into WMS.
// @Tags Orders
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body SyncOrdersRequest true "Sync payload"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/orders/sync [post]
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
