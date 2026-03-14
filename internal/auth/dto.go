package auth

// LoginRequest represents the payload for the internal login endpoint
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents the token payload returned upon successful login
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenRequest represents payload for refresh endpoint
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest represents payload for logout endpoint
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
