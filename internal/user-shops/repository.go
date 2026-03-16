package usershops

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles user-shop relation queries.
type Repository interface {
	FindShopIDByUserID(userID string) (string, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindShopIDByUserID(userID string) (string, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}

	var userShop UserShop
	err = r.db.Where("user_id = ?", parsedUserID).First(&userShop).Error
	if err != nil {
		return "", err
	}

	return userShop.ShopID, nil
}
