package auth

// Service defines the interface for business logic in the Auth domain
type Service interface {
	// Business methods go here (e.g. Login, GenerateToken, ProcessOAuthCode)
}

type service struct {
	repo Repository
}

// NewService creates a new Auth Service instance
func NewService(repo Repository) Service {
	return &service{repo: repo}
}
