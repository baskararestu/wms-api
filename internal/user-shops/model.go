package usershops

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserShop represents the join table for users and marketplace shops
// Supports many-to-many mapping: one user can have multiple shops, one shop can belong to multiple users (if needed)
type UserShop struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_shop" json:"user_id"`
	ShopID    string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_user_shop" json:"shop_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
