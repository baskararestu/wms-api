package orders

import (
	"encoding/json"
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
	SyncMarketplaceOrders(shopID string) error
	ProcessWebhook(payload WebhookPayload) error
}

type service struct {
	repo  Repository
	mpSvc marketplace.Service
}

func NewService(repo Repository, mpSvc marketplace.Service) Service {
	return &service{repo: repo, mpSvc: mpSvc}
}

func (s *service) GetOrders(query GetOrderListQuery) (*OrderListResponse, error) {
	// Try getting from cache first
	cacheKey := s.generateCacheKey(query)
	if redis.Client != nil {
		cached, err := redis.Client.Get(redis.Ctx, cacheKey).Result()
		if err == nil && cached != "" {
			var resp OrderListResponse
			if json.Unmarshal([]byte(cached), &resp) == nil {
				xlogger.Logger.Debug().Str("cache_key", cacheKey).Msg("Orders retrieved from Redis cache hit")
				return &resp, nil
			}
		}
	}

	// Cache Miss / Fallback
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

	if redis.Client != nil {
		jsonData, _ := json.Marshal(resp)
		redis.Client.Set(redis.Ctx, cacheKey, jsonData, 5*time.Minute)
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

	// A rigorous validation could check if order.WMSStatus == "Ready to Pickup" transitioning to "Picking" only etc.
	err = s.repo.UpdateWMSStatus(id, newStatus)
	if err != nil {
		return err
	}

	s.invalidateOrdersCache()
	return nil
}

func (s *service) SyncMarketplaceOrders(shopID string) error {
	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Starting sync from marketplace mock")

	r, err := s.mpSvc.GetOrderListByShopID(shopID)
	if err != nil {
		xlogger.Logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to call marketplace get order list")
		// The error will be mapped to UI response
		return err
	}

	for _, mpOrder := range r.Data {
		var totalAmt float64
		var items []OrderItem

		id := uuid.New()

		for _, mpItem := range mpOrder.Items {
			items = append(items, OrderItem{
				OrderID:  id,
				SKU:      mpItem.SKU,
				Name:     fmt.Sprintf("Product %s", mpItem.SKU), // API didn't provide name
				Quantity: mpItem.Quantity,
				Price:    mpItem.Price,
			})
			totalAmt += (mpItem.Price * float64(mpItem.Quantity))
		}

		// Parse created_at
		var createdAt time.Time
		if t, err := time.Parse(time.RFC3339, mpOrder.CreatedAt); err == nil {
			createdAt = t
		} else {
			createdAt = time.Now()
		}

		wmsStatus := WMSStatusReadyToPickup

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
			WMSStatus:         wmsStatus,
			TrackingNumber:    mpOrder.TrackingNumber,
			TotalAmount:       totalAmt,
		}

		// Insert or Update the order. We rely on the raw Upsert we built.
		// However we need to upsert items as well, so a basic find and replace is better, or a transaction
		existing, err := s.repo.FindOrderBySN(mpOrder.OrderSN)
		if err == nil && existing != nil {
			// update existing
			existing.MarketplaceStatus = mpOrder.Status
			existing.ShippingStatus = mpOrder.ShippingStatus
			existing.TrackingNumber = mpOrder.TrackingNumber
			existing.TotalAmount = totalAmt
			existing.Items = items
			existing.UpdatedAt = time.Now()

			_ = s.repo.UpsertOrder(existing)
		} else {
			// new record
			order.Items = items
			_ = s.repo.UpsertOrder(order)
		}
	}

	s.invalidateOrdersCache()
	xlogger.Logger.Info().Int("synced_count", len(r.Data)).Str("shop_id", shopID).Msg("Marketplace sync completed")
	return nil
}

func (s *service) ProcessWebhook(payload WebhookPayload) error {
	err := s.repo.UpdateMarketplaceStatus(
		payload.OrderSN,
		payload.Data.MarketplaceStatus,
		payload.Data.ShippingStatus,
		payload.Data.TrackingNumber,
	)

	if err != nil {
		xlogger.Logger.Error().Err(err).Str("order_sn", payload.OrderSN).Msg("Failed to update marketplace status from webhook")
		return err
	}

	s.invalidateOrdersCache()
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
