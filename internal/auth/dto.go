package auth

// LoginRequest represents the payload for the internal login endpoint
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=4"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents the token payload returned upon successful login
type LoginResponse struct {
	Token string `json:"token"`
}
