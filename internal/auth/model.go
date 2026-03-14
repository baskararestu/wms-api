package auth

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides UUID and standard timestamps
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// User represents an internal API user
type User struct {
	BaseModel
	Email        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"type:varchar(255);not null" json:"-"`
}

// MarketplaceCredential stores OAuth tokens for a specific shop
type MarketplaceCredential struct {
	BaseModel
	ShopID       string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"shop_id"`
	AccessToken  string    `gorm:"type:text;not null" json:"-"`
	RefreshToken string    `gorm:"type:text;not null" json:"-"`
	ExpiresAt    time.Time `json:"expires_at"` // Calculated by adding 'expires_in' (300) to current time
}
