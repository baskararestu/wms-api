package orders

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

// Order represents an imported marketplace order
type Order struct {
	BaseModel
	OrderSN               string      `gorm:"type:varchar(100);uniqueIndex;not null" json:"order_sn"`
	ShopID                string      `gorm:"type:varchar(100);index;not null" json:"shop_id"`
	MarketplaceStatus     string      `gorm:"type:varchar(50);not null" json:"marketplace_status"`
	ShippingStatus        string      `gorm:"type:varchar(50)" json:"shipping_status"` // specific to platform logistics
	WmsStatus             string      `gorm:"type:varchar(50);not null;default:'READY_TO_PICK'" json:"wms_status"`
	ShippingProvider      string      `gorm:"type:varchar(100)" json:"shipping_provider"`
	AWBNumber             string      `gorm:"type:varchar(100);index" json:"awb_number"` // Airway Bill
	BuyerInfo             string      `gorm:"type:jsonb" json:"buyer_info"`              // Store structured JSON
	TotalAmount           float64     `gorm:"type:numeric(10,2);not null;default:0" json:"total_amount"`
	RawMarketplacePayload string      `gorm:"type:jsonb" json:"raw_marketplace_payload"` // Store the raw webhook request for auditing
	OrderItems            []OrderItem `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"items"`
}

// OrderItem represents a physical item within an order
type OrderItem struct {
	BaseModel
	OrderID           uuid.UUID `gorm:"type:uuid;index;not null" json:"order_id"`
	MarketplaceItemID string    `gorm:"type:varchar(100);not null" json:"marketplace_item_id"`
	SKU               string    `gorm:"type:varchar(100);index;not null" json:"sku"`
	Name              string    `gorm:"type:varchar(255);not null" json:"name"`
	Quantity          int       `gorm:"not null;default:1" json:"quantity"`
	Price             float64   `gorm:"type:numeric(10,2);not null;default:0" json:"price"`
}
