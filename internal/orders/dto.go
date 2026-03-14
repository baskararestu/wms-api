package orders

import (
	"time"

	"github.com/google/uuid"
)

// PaginationQuery reusable params for querying
type PaginationQuery struct {
	Page    int    `query:"page" validate:"min=1"`
	Limit   int    `query:"limit" validate:"min=1,max=100"`
	SortBy  string `query:"sort_by"`
	SortDir string `query:"sort_dir" validate:"omitempty,oneof=asc desc"`
	Search  string `query:"search"`
}

// GetOrderListQuery extends default pagination with specific order filters
type GetOrderListQuery struct {
	PaginationQuery
	WMSStatus         string `query:"wms_status"`
	MarketplaceStatus string `query:"marketplace_status"`
	ShippingStatus    string `query:"shipping_status"`
	ShopID            string `query:"shop_id"`
}

// OrderListResponse represents the response payload for GET /orders
type OrderListResponse struct {
	Total        int64             `json:"total"`
	Page         int               `json:"page"`
	Limit        int               `json:"limit"`
	TotalPages   int               `json:"total_pages"`
	Orders       []OrderListItem   `json:"orders"`
	SummaryStats OrderSummaryStats `json:"summary_stats"`
}

type OrderSummaryStats struct {
	TotalOrdersCount     int64 `json:"total_orders_count"`
	CancelledOrdersCount int64 `json:"cancelled_orders_count"`
}

type OrderListItem struct {
	ID                uuid.UUID `json:"id"`
	OrderSN           string    `json:"order_sn"`
	MarketplaceStatus string    `json:"marketplace_status"`
	ShippingStatus    string    `json:"shipping_status"`
	WMSStatus         string    `json:"wms_status"`
	TrackingNumber    string    `json:"tracking_number"`
	UpdatedAt         time.Time `json:"updated_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// OrderDetailItem represents an individual item in an order
type OrderDetailItem struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"qty"`
	Price    float64 `json:"price"`
}

// OrderDetailResponse represents the response payload for GET /orders/:id
type OrderDetailResponse struct {
	OrderListItem
	TotalAmount float64           `json:"total_amount"`
	Items       []OrderDetailItem `json:"items"`
}

// UpdateWMSStatusRequest represents the payload from picking/packing/shipping buttons
type UpdateWMSStatusRequest struct {
	Status string `json:"wms_status" validate:"required,oneof=PICKING PACKED SHIPPED"`
}

// SyncOrdersRequest triggers the background or foreground pulling of orders
type SyncOrdersRequest struct {
	ShopID string `json:"shop_id" validate:"required"`
}

// WebhookPayload represents the expected payload when marketplace status changes
type WebhookPayload struct {
	ShopID    string `json:"shop_id" validate:"required"`
	OrderSN   string `json:"order_sn" validate:"required"`
	Code      string `json:"code" validate:"required"` // e.g., ORDER_STATUS_UPDATE
	Timestamp int64  `json:"timestamp" validate:"required"`
	Data      struct {
		MarketplaceStatus string `json:"marketplace_status"`
		ShippingStatus    string `json:"shipping_status"`
		TrackingNumber    string `json:"tracking_number"`
	} `json:"data"`
}
