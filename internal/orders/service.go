package orders

// Service defines the interface for business logic in the Orders domain
type Service interface {
	// Business methods go here (e.g. SyncFromMarketplace, PickOrder, ShipOrder)
}

type service struct {
	repo Repository
}

// NewService creates a new Orders Service instance
func NewService(repo Repository) Service {
	return &service{repo: repo}
}
