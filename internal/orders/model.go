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
	WMSStatusReadyToPickup = "Ready to Pickup"
	WMSStatusPicking       = "Picking"
	WMSStatusPacked        = "Packed"
	WMSStatusShipped       = "Shipped"
)

// Order represents a customer order from a marketplace synced into the WMS
type Order struct {
	BaseModel
	OrderSN               string      `gorm:"type:varchar(100);uniqueIndex;not null" json:"order_sn"`
	ShopID                string      `gorm:"type:varchar(100);index;not null" json:"shop_id"`
	Marketplace           string      `gorm:"type:varchar(50);not null" json:"marketplace"` // e.g. "SHOPEE"
	MarketplaceStatus     string      `gorm:"type:varchar(50);not null" json:"marketplace_status"`
	ShippingStatus        string      `gorm:"type:varchar(50);not null" json:"shipping_status"`
	WMSStatus             string      `gorm:"type:varchar(50);index;not null;default:'Ready to Pickup'" json:"wms_status"`
	TrackingNumber        string      `gorm:"type:varchar(100)" json:"tracking_number"`
	TotalAmount           float64     `gorm:"type:numeric(12,2);not null;default:0" json:"total_amount"`
	Items                 []OrderItem `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items"`
}

// OrderItem represents individual products in an order
type OrderItem struct {
	BaseModel
	OrderID             uuid.UUID `gorm:"type:uuid;index;not null" json:"order_id"`
	MarketplaceItemID   string    `gorm:"type:varchar(100)" json:"marketplace_item_id"`
	SKU                 string    `gorm:"type:varchar(100);index;not null" json:"sku"`
	Name                string    `gorm:"type:varchar(255);not null" json:"name"`
	Quantity            int       `gorm:"type:int;not null;default:1" json:"quantity"`
	Price               float64   `gorm:"type:numeric(12,2);not null;default:0" json:"price"`
}
