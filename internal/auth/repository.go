package auth

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines the interface for data operations in the Auth domain
type Repository interface {
	FindUserByEmail(email string) (*User, error)
	FindUserByID(userID uuid.UUID) (*User, error)
	CreateUser(user *User) error
	CreateRefreshToken(token *RefreshToken) error
	FindRefreshTokenByHash(tokenHash string) (*RefreshToken, error)
	RevokeRefreshTokenByHash(tokenHash string) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Auth Repository instance
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindUserByEmail(email string) (*User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) FindUserByID(userID uuid.UUID) (*User, error) {
	var user User
	err := r.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) CreateUser(user *User) error {
	return r.db.Create(user).Error
}

func (r *repository) CreateRefreshToken(token *RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *repository) FindRefreshTokenByHash(tokenHash string) (*RefreshToken, error) {
	var token RefreshToken
	err := r.db.Where("token_hash = ?", tokenHash).First(&token).Error
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (r *repository) RevokeRefreshTokenByHash(tokenHash string) error {
	now := time.Now()
	return r.db.Model(&RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", now).Error
}
