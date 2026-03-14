package marketplace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const DefaultMarketplace = "shopee"

// MarketplaceCredential stores OAuth tokens for a specific shop
type MarketplaceCredential struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Marketplace  string         `gorm:"type:varchar(50);not null;default:shopee" json:"marketplace"`
	ShopID       string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"shop_id"`
	AccessToken  string         `gorm:"type:text;not null" json:"-"`
	RefreshToken string         `gorm:"type:text;not null" json:"-"`
	ExpiresAt    time.Time      `json:"expires_at"`
}
