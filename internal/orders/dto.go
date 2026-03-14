package orders

// SyncWebhookRequest represents the incoming payload from the Marketplace Webhook
type SyncWebhookRequest struct {
	OrderSN          string  `json:"order_sn" validate:"required"`
	ShopID           string  `json:"shop_id" validate:"required"`
	MarketplaceStatus string  `json:"status" validate:"required"`
	ShippingStatus   string  `json:"shipping_status"`
	TrackingNumber   string  `json:"tracking_number"`
	TotalAmount      float64 `json:"total_amount"`
}

// OrderResponse represents the WMS internal response for an Order 
// This masks the internal database struct
type OrderResponse struct {
	OrderSN        string `json:"order_sn"`
	WmsStatus      string `json:"wms_status"`
	ShippingStatus string `json:"shipping_status"`
	TrackingNumber string `json:"tracking_number"`
}
