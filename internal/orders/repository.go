package orders

import "gorm.io/gorm"

// Repository defines the interface for data operations in the Orders domain
type Repository interface {
	// Methods will go here (e.g. FindOrderBySN, CreateOrder, UpdateWmsStatus, etc.)
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Orders Repository instance
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}
