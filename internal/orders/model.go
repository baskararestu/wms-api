package orders

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides common fields for all GORM models
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Order status constants
const (
	WMSStatusReadyToPickup = "READY_TO_PICK"
	WMSStatusPicking       = "PICKING"
	WMSStatusPacked        = "PACKED"
	WMSStatusShipped       = "SHIPPED"
	WMSStatusCancelled     = "CANCELLED"
)

// Marketplace status constants (from external API)
const (
	MPStatusProcessing = "processing"
	MPStatusPaid       = "paid"
	MPStatusShipping   = "shipping"
	MPStatusDelivered  = "delivered"
	MPStatusCancelled  = "cancelled"
)

// resolveInitialWMSStatus maps marketplace_status to the correct initial wms_status
// when a new order is ingested from the marketplace.
func resolveInitialWMSStatus(marketplaceStatus string) string {
	switch marketplaceStatus {
	case MPStatusProcessing, MPStatusPaid:
		return WMSStatusReadyToPickup
	case MPStatusShipping, MPStatusDelivered:
		return WMSStatusShipped
	case MPStatusCancelled:
		return WMSStatusCancelled
	default:
		// Unknown status → treat as not actionable, stay READY_TO_PICK
		return WMSStatusReadyToPickup
	}
}

// isActionableForWMS returns true if the marketplace_status allows pick/pack/ship
func isActionableForWMS(marketplaceStatus string) bool {
	return marketplaceStatus == MPStatusProcessing || marketplaceStatus == MPStatusPaid
}

// Order represents a customer order from a marketplace synced into the WMS
type Order struct {
	BaseModel
	OrderSN           string      `gorm:"type:varchar(100);uniqueIndex;not null" json:"order_sn"`
	ShopID            string      `gorm:"type:varchar(100);index;not null" json:"shop_id"`
	MarketplaceStatus string      `gorm:"type:varchar(50);not null" json:"marketplace_status"`
	ShippingStatus    string      `gorm:"type:varchar(50);not null" json:"shipping_status"`
	ShippingChannel   string      `gorm:"type:varchar(50)" json:"shipping_channel"`
	WMSStatus         string      `gorm:"type:varchar(50);index;not null;default:'READY_TO_PICK'" json:"wms_status"`
	TrackingNumber    string      `gorm:"type:varchar(100)" json:"tracking_number"`
	TotalAmount       float64     `gorm:"type:numeric(12,2);not null;default:0" json:"total_amount"`
	Items             []OrderItem `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items"`
}

// OrderItem represents individual products in an order
type OrderItem struct {
	BaseModel
	OrderID  uuid.UUID `gorm:"type:uuid;index;not null" json:"order_id"`
	SKU      string    `gorm:"type:varchar(100);index;not null" json:"sku"`
	Name     string    `gorm:"type:varchar(255);not null" json:"name"`
	Quantity int       `gorm:"type:int;not null;default:1" json:"quantity"`
	Price    float64   `gorm:"type:numeric(12,2);not null;default:0" json:"price"`
}
