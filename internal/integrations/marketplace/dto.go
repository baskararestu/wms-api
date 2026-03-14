package marketplace

// LinkShopRequest represents the incoming payload from the frontend to initiate OAuth
type LinkShopRequest struct {
	ShopID string `json:"shop_id" validate:"required"`
}

// OAuthCallbackRequest represents query payload for OAuth callback completion
type OAuthCallbackRequest struct {
	Code   string `query:"code" validate:"required"`
	ShopID string `query:"shop_id" validate:"required"`
	State  string `query:"state" validate:"required"`
}

// LinkShopStartResponse contains the result of one-step connect flow
type LinkShopStartResponse struct {
	ShopID    string `json:"shop_id"`
	Connected bool   `json:"connected"`
}

// AuthCodeResponse represents the mock API response for /oauth/authorize
type AuthCodeResponse struct {
	Message string `json:"message"`
	Data    struct {
		Code   string `json:"code"`
		ShopID string `json:"shop_id"`
		State  string `json:"state"`
	} `json:"data"`
}

// TokenRequest represents the payload to exchange the code or refresh token
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// TokenResponse represents the payload received from /oauth/token
type TokenResponse struct {
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"` // seconds (e.g., 300)
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
	} `json:"data"`
}

// ShopDetailResponse represents marketplace shop detail response
type ShopDetailResponse struct {
	Message string `json:"message"`
	Data    struct {
		ShopID      string `json:"shop_id"`
		ShopName    string `json:"shop_name"`
		Marketplace string `json:"marketplace"`
		Country     string `json:"country"`
		Currency    string `json:"currency"`
		AccessLevel string `json:"access_level"`
	} `json:"data"`
}

// OrderItemDTO represents an item in an order from the mock API
type OrderItemDTO struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"qty"`
	Price    float64 `json:"price"`
}

// OrderDTO represents an order from the mock API
type OrderDTO struct {
	OrderSN        string         `json:"order_sn"`
	ShopID         string         `json:"shop_id"`
	Status         string         `json:"status"` // Marketplace status
	ShippingStatus string         `json:"shipping_status"`
	TrackingNumber string         `json:"tracking_number,omitempty"`
	Items          []OrderItemDTO `json:"items"`
	TotalAmount    float64        `json:"total_amount"`
	CreatedAt      string         `json:"created_at"` // ISO8601 string
}

// OrderListResponse represents the response for /order/list
type OrderListResponse struct {
	Message string     `json:"message"`
	Data    []OrderDTO `json:"data"`
}

// OrderDetailResponse represents the response for /order/detail
type OrderDetailResponse struct {
	Message string   `json:"message"`
	Data    OrderDTO `json:"data"`
}

// ShipExternalOrderRequest represents payload for external logistic/ship API
type ShipExternalOrderRequest struct {
	OrderSN   string `json:"order_sn"`
	ChannelID string `json:"channel_id"`
}

// ShipExternalOrderResponse represents the response from external logistic/ship
type ShipExternalOrderResponse struct {
	Message string `json:"message"`
	Data    struct {
		OrderSN        string `json:"order_sn"`
		ShippingStatus string `json:"shipping_status"`
		TrackingNo     string `json:"tracking_no"`
	} `json:"data"`
}

// LogisticChannelDTO represents a single shipping channel from the marketplace
type LogisticChannelDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// LogisticChannelsResponse represents the marketplace /logistic/channels response
type LogisticChannelsResponse struct {
	Message string               `json:"message"`
	Data    []LogisticChannelDTO `json:"data"`
}
