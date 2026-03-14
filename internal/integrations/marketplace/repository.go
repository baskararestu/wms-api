package marketplace

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	FindMarketplaceCredentialByShopID(shopID string) (*MarketplaceCredential, error)
	UpsertMarketplaceCredential(cred *MarketplaceCredential, expiresIn int) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindMarketplaceCredentialByShopID(shopID string) (*MarketplaceCredential, error) {
	var cred MarketplaceCredential
	err := r.db.Where("shop_id = ?", shopID).First(&cred).Error
	if err != nil {
		return nil, err
	}

	return &cred, nil
}

func (r *repository) UpsertMarketplaceCredential(cred *MarketplaceCredential, expiresIn int) error {
	cred.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	if cred.Marketplace == "" {
		cred.Marketplace = DefaultMarketplace
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing MarketplaceCredential
		err := tx.Where("shop_id = ?", cred.ShopID).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(cred).Error
			}
			return err
		}

		existing.AccessToken = cred.AccessToken
		existing.RefreshToken = cred.RefreshToken
		existing.ExpiresAt = cred.ExpiresAt
		if existing.Marketplace == "" {
			existing.Marketplace = cred.Marketplace
		}

		return tx.Save(&existing).Error
	})
}
