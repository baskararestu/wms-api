package orders

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/baskararestu/wms-api/internal/integrations/marketplace"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/baskararestu/wms-api/internal/redis"
	"github.com/google/uuid"
)

type Service interface {
	GetOrders(query GetOrderListQuery) (*OrderListResponse, error)
	GetOrderDetail(orderSN string) (*OrderDetailResponse, error)
	UpdateWMSStatus(id uuid.UUID, newStatus string) error
	PickOrder(orderSN string) error
	PackOrder(orderSN string) error
	ShipOrder(orderSN, channelID string) (*ShipOrderResponse, error)
	SyncMarketplaceOrders(shopID string) error
}

type service struct {
	repo  Repository
	mpSvc marketplace.Service
}

var ErrInvalidSince = errors.New("invalid since format, use RFC3339")

func NewService(repo Repository, mpSvc marketplace.Service) Service {
	return &service{repo: repo, mpSvc: mpSvc}
}

func (s *service) GetOrders(query GetOrderListQuery) (*OrderListResponse, error) {
	if query.Since != "" {
		if _, err := time.Parse(time.RFC3339, query.Since); err != nil {
			return nil, ErrInvalidSince
		}
	}

	orders, total, err := s.repo.FindOrders(query)
	if err != nil {
		return nil, err
	}

	totalMonth, cancelMonth, err := s.repo.FindOrderSummaryStats()
	if err != nil {
		return nil, err
	}

	limit := query.Limit
	if limit < 1 {
		limit = 10
	}
	page := query.Page
	if page < 1 {
		page = 1
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	var list []OrderListItem
	for _, o := range orders {
		list = append(list, OrderListItem{
			ID:                o.ID,
			OrderSN:           o.OrderSN,
			MarketplaceStatus: o.MarketplaceStatus,
			ShippingStatus:    o.ShippingStatus,
			WMSStatus:         o.WMSStatus,
			TrackingNumber:    o.TrackingNumber,
			UpdatedAt:         o.UpdatedAt,
			CreatedAt:         o.CreatedAt,
		})
	}
	if list == nil {
		list = []OrderListItem{}
	}

	resp := &OrderListResponse{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		Orders:     list,
		SummaryStats: OrderSummaryStats{
			TotalOrdersCount:     totalMonth,
			CancelledOrdersCount: cancelMonth,
		},
	}

	return resp, nil
}

func (s *service) GetOrderDetail(orderSN string) (*OrderDetailResponse, error) {
	order, err := s.repo.FindOrderBySN(orderSN)
	if err != nil {
		return nil, errors.New("order not found")
	}

	var items []OrderDetailItem
	for _, i := range order.Items {
		items = append(items, OrderDetailItem{
			SKU:      i.SKU,
			Quantity: i.Quantity,
			Price:    i.Price,
		})
	}
	if items == nil {
		items = []OrderDetailItem{}
	}

	return &OrderDetailResponse{
		OrderListItem: OrderListItem{
			ID:                order.ID,
			OrderSN:           order.OrderSN,
			MarketplaceStatus: order.MarketplaceStatus,
			ShippingStatus:    order.ShippingStatus,
			WMSStatus:         order.WMSStatus,
			TrackingNumber:    order.TrackingNumber,
			UpdatedAt:         order.UpdatedAt,
			CreatedAt:         order.CreatedAt,
		},
		TotalAmount: order.TotalAmount,
		Items:       items,
	}, nil
}

func (s *service) UpdateWMSStatus(id uuid.UUID, newStatus string) error {
	order, err := s.repo.FindOrderByID(id)
	if err != nil {
		return errors.New("order not found")
	}

	// Very simple lifecycle validation matching the modal buttons
	if order.WMSStatus == newStatus {
		return nil // skip identical updates
	}

	// A rigorous validation could check if order.WMSStatus == "READY_TO_PICK" transitioning to "PICKING" only etc.
	err = s.repo.UpdateWMSStatus(id, newStatus)
	if err != nil {
		return err
	}

	s.invalidateOrdersCache()
	return nil
}

func (s *service) PickOrder(orderSN string) error {
	order, err := s.repo.FindOrderBySN(orderSN)
	if err != nil {
		return errors.New("order not found")
	}

	if !isActionableForWMS(order.MarketplaceStatus) {
		return fmt.Errorf("order cannot be picked, marketplace status is: %s", order.MarketplaceStatus)
	}

	if order.WMSStatus != WMSStatusReadyToPickup {
		return fmt.Errorf("order cannot be picked, current wms status: %s", order.WMSStatus)
	}

	err = s.repo.UpdateWMSStatusBySN(orderSN, WMSStatusPicking)
	if err != nil {
		return err
	}

	s.invalidateOrdersCache()
	return nil
}

func (s *service) PackOrder(orderSN string) error {
	order, err := s.repo.FindOrderBySN(orderSN)
	if err != nil {
		return errors.New("order not found")
	}

	if !isActionableForWMS(order.MarketplaceStatus) {
		return fmt.Errorf("order cannot be packed, marketplace status is: %s", order.MarketplaceStatus)
	}

	if order.WMSStatus != WMSStatusPicking {
		return fmt.Errorf("order cannot be packed, current wms status: %s", order.WMSStatus)
	}

	err = s.repo.UpdateWMSStatusBySN(orderSN, WMSStatusPacked)
	if err != nil {
		return err
	}

	s.invalidateOrdersCache()
	return nil
}

func (s *service) ShipOrder(orderSN, channelID string) (*ShipOrderResponse, error) {
	order, err := s.repo.FindOrderBySN(orderSN)
	if err != nil {
		return nil, errors.New("order not found")
	}

	if !isActionableForWMS(order.MarketplaceStatus) {
		return nil, fmt.Errorf("order cannot be shipped, marketplace status is: %s", order.MarketplaceStatus)
	}

	if order.WMSStatus != WMSStatusPacked {
		return nil, fmt.Errorf("order cannot be shipped, current wms status: %s", order.WMSStatus)
	}

	// Call the external marketplace API to ship (generate tracking NO)
	resp, err := s.mpSvc.ShipOrder(order.ShopID, orderSN, channelID)
	if err != nil {
		xlogger.Logger.Error().Err(err).Str("order_sn", orderSN).Str("channel_id", channelID).Msg("Failed to call marketplace ship order API")
		return nil, fmt.Errorf("failed to sync with marketplace: %v", err)
	}

	trackingNo := resp.Data.TrackingNo
	shippingStatus := resp.Data.ShippingStatus
	wmsStatus := WMSStatusShipped

	err = s.repo.UpdateShipmentInfo(orderSN, trackingNo, shippingStatus, channelID, wmsStatus)
	if err != nil {
		xlogger.Logger.Error().Err(err).Str("order_sn", orderSN).Msg("Failed to update shipment info in DB")
		return nil, errors.New("failed to save shipment info locally")
	}

	if err := s.mpSvc.NotifyShippingStatus(orderSN, shippingStatus); err != nil {
		xlogger.Logger.Warn().Err(err).Str("order_sn", orderSN).Str("shipping_status", shippingStatus).Msg("Failed to notify marketplace shipping-status webhook")
	}

	if err := s.mpSvc.NotifyOrderStatus(orderSN, shippingStatus); err != nil {
		xlogger.Logger.Warn().Err(err).Str("order_sn", orderSN).Str("status", shippingStatus).Msg("Failed to notify marketplace order-status webhook")
	}

	s.invalidateOrdersCache()

	return &ShipOrderResponse{
		OrderSN:         orderSN,
		WMSStatus:       wmsStatus,
		ShippingStatus:  shippingStatus,
		TrackingNumber:  trackingNo,
		ShippingChannel: channelID,
	}, nil
}

func (s *service) SyncMarketplaceOrders(shopID string) error {
	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Starting sync from marketplace mock")

	r, err := s.mpSvc.GetOrderListByShopID(shopID)
	if err != nil {
		xlogger.Logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to call marketplace get order list")
		return err
	}

	successCount := 0
	failCount := 0

	for _, mpOrder := range r.Data {
		id := uuid.New()

		var items []OrderItem
		for _, mpItem := range mpOrder.Items {
			items = append(items, OrderItem{
				OrderID:  id,
				SKU:      mpItem.SKU,
				Name:     fmt.Sprintf("Product %s", mpItem.SKU),
				Quantity: mpItem.Quantity,
				Price:    mpItem.Price,
			})
		}

		var createdAt time.Time
		if t, err := time.Parse(time.RFC3339, mpOrder.CreatedAt); err == nil {
			createdAt = t
		} else {
			createdAt = time.Now()
		}

		existing, findErr := s.repo.FindOrderBySN(mpOrder.OrderSN)

		if findErr == nil && existing != nil {
			// --- UPDATE existing order ---
			// Always sync marketplace fields
			existing.MarketplaceStatus = mpOrder.Status
			existing.ShippingStatus = mpOrder.ShippingStatus
			existing.TrackingNumber = mpOrder.TrackingNumber
			existing.TotalAmount = mpOrder.TotalAmount
			existing.UpdatedAt = time.Now()

			// Re-resolve wms_status only if order is not yet in an active WMS workflow.
			// Once a warehouse worker has started picking, we do NOT override their progress.
			if existing.WMSStatus == WMSStatusReadyToPickup || existing.WMSStatus == WMSStatusCancelled {
				existing.WMSStatus = resolveInitialWMSStatus(mpOrder.Status)
			}
			// If the marketplace cancels an order mid-workflow (e.g. during PICKING/PACKED),
			// force-cancel it so workers don't waste time fulfilling it.
			if mpOrder.Status == MPStatusCancelled && existing.WMSStatus != WMSStatusShipped {
				existing.WMSStatus = WMSStatusCancelled
			}

			existing.Items = items

			if err := s.repo.UpsertOrder(existing); err != nil {
				xlogger.Logger.Error().Err(err).Str("order_sn", mpOrder.OrderSN).Msg("Failed to upsert existing order during sync")
				failCount++
				continue
			}
		} else {
			// --- INSERT new order ---
			order := &Order{
				BaseModel: BaseModel{
					ID:        id,
					CreatedAt: createdAt,
					UpdatedAt: time.Now(),
				},
				OrderSN:           mpOrder.OrderSN,
				ShopID:            mpOrder.ShopID,
				MarketplaceStatus: mpOrder.Status,
				ShippingStatus:    mpOrder.ShippingStatus,
				WMSStatus:         resolveInitialWMSStatus(mpOrder.Status),
				TrackingNumber:    mpOrder.TrackingNumber,
				TotalAmount:       mpOrder.TotalAmount,
				Items:             items,
			}

			if err := s.repo.UpsertOrder(order); err != nil {
				xlogger.Logger.Error().Err(err).Str("order_sn", mpOrder.OrderSN).Msg("Failed to insert new order during sync")
				failCount++
				continue
			}
		}

		successCount++
	}

	s.invalidateOrdersCache()
	xlogger.Logger.Info().
		Str("shop_id", shopID).
		Int("total", len(r.Data)).
		Int("success", successCount).
		Int("failed", failCount).
		Msg("Marketplace sync completed")
	return nil
}

// Helpers
func (s *service) generateCacheKey(q GetOrderListQuery) string {
	return fmt.Sprintf("orders:list:wms=%s:mp=%s:ss=%s:shop=%s:search=%s:page=%d:limit=%d:sort=%s:%s",
		q.WMSStatus, q.MarketplaceStatus, q.ShippingStatus, q.ShopID, q.Search, q.Page, q.Limit, q.SortBy, q.SortDir)
}

func (s *service) invalidateOrdersCache() {
	if redis.Client != nil {
		iter := redis.Client.Scan(redis.Ctx, 0, "orders:list:*", 0).Iterator()
		for iter.Next(redis.Ctx) {
			redis.Client.Del(redis.Ctx, iter.Val())
		}
	}
}
