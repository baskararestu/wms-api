package auth

import "gorm.io/gorm"

// Repository defines the interface for data operations in the Auth domain
type Repository interface {
	// Methods will go here (e.g. FindUserByUsername, CreateUser, etc.)
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Auth Repository instance
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}
