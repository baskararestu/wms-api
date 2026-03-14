package marketplace

// LinkShopRequest represents the incoming payload from the frontend to initiate OAuth
type LinkShopRequest struct {
	ShopID string `json:"shop_id" validate:"required"`
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
