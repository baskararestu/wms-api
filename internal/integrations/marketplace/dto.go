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
