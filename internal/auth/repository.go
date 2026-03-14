package auth

import "gorm.io/gorm"

// Repository defines the interface for data operations in the Auth domain
type Repository interface {
	FindUserByUsername(username string) (*User, error)
	CreateUser(user *User) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Auth Repository instance
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindUserByUsername(username string) (*User, error) {
	var user User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) CreateUser(user *User) error {
	return r.db.Create(user).Error
}
