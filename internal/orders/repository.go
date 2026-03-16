package orders

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	FindOrders(query GetOrderListQuery) ([]Order, int64, error)
	FindOrderSummaryStats() (int64, int64, error)
	FindOrderByID(id uuid.UUID) (*Order, error)
	FindOrderBySN(orderSN string) (*Order, error)
	UpsertOrder(order *Order) error
	UpdateWMSStatus(id uuid.UUID, newStatus string) error
	UpdateWMSStatusBySN(orderSN, newStatus string) error
	UpdateOrderStatus(orderSN, status string) error
	UpdateShippingStatus(orderSN, status string) error
	UpdateMarketplaceStatus(orderSN, mpStatus, shipStatus, tracking string) error
	UpdateShipmentInfo(orderSN, trackingNum, shippingStatus, shippingChannel, wmsStatus string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindOrders(query GetOrderListQuery) ([]Order, int64, error) {
	var orders []Order
	var total int64

	dbQuery := r.db.Model(&Order{})

	if query.WMSStatus != "" {
		dbQuery = dbQuery.Where("wms_status = ?", query.WMSStatus)
	}
	if query.MarketplaceStatus != "" {
		dbQuery = dbQuery.Where("marketplace_status = ?", query.MarketplaceStatus)
	}
	if query.ShippingStatus != "" {
		dbQuery = dbQuery.Where("shipping_status = ?", query.ShippingStatus)
	}
	if query.ShopID != "" {
		dbQuery = dbQuery.Where("shop_id = ?", query.ShopID)
	}
	if query.Since != "" {
		sinceTime, err := time.Parse(time.RFC3339, query.Since)
		if err != nil {
			return nil, 0, err
		}
		dbQuery = dbQuery.Where("updated_at > ?", sinceTime)
	}
	if query.Search != "" {
		searchTerm := "%" + query.Search + "%"
		dbQuery = dbQuery.Where("order_sn ILIKE ? OR tracking_number ILIKE ?", searchTerm, searchTerm)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	orderStr := "updated_at desc"
	if query.SortBy != "" {
		sortDir := "asc"
		if strings.EqualFold(query.SortDir, "desc") {
			sortDir = "desc"
		}
		// simple prevention of sql injection for sort column
		allowedColumns := map[string]bool{
			"created_at":         true,
			"updated_at":         true,
			"order_sn":           true,
			"wms_status":         true,
			"shipping_status":    true,
			"marketplace_status": true,
		}
		if allowedColumns[query.SortBy] {
			orderStr = query.SortBy + " " + sortDir
			if query.SortBy != "updated_at" {
				orderStr += ", updated_at desc"
			}
		}
	}

	err := dbQuery.Order(orderStr).Limit(limit).Offset(offset).Find(&orders).Error
	return orders, total, err
}

func (r *repository) FindOrderSummaryStats() (int64, int64, error) {
	var totalOrders int64
	var cancelledOrders int64

	err := r.db.Model(&Order{}).Count(&totalOrders).Error
	if err != nil {
		return 0, 0, err
	}

	err = r.db.Model(&Order{}).
		Where("marketplace_status = ? OR wms_status = ? OR shipping_status ILIKE ?", MPStatusCancelled, WMSStatusCancelled, "%cancel%").
		Count(&cancelledOrders).Error
	if err != nil {
		return 0, 0, err
	}

	return totalOrders, cancelledOrders, nil
}

func (r *repository) FindOrderByID(id uuid.UUID) (*Order, error) {
	var order Order
	err := r.db.Preload("Items").Where("id = ?", id).First(&order).Error
	return &order, err
}

func (r *repository) FindOrderBySN(orderSN string) (*Order, error) {
	var order Order
	err := r.db.Preload("Items").Where("order_sn = ?", orderSN).First(&order).Error
	return &order, err
}

func (r *repository) UpsertOrder(order *Order) error {
	// GORM raw upsert since we want to handle the specific fields
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_sn"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"marketplace_status", "shipping_status", "tracking_number",
			"updated_at", "total_amount", "wms_status",
		}),
	}).Create(order).Error
}

func (r *repository) UpdateWMSStatus(id uuid.UUID, newStatus string) error {
	return r.db.Model(&Order{}).Where("id = ?", id).Update("wms_status", newStatus).Error
}

func (r *repository) UpdateWMSStatusBySN(orderSN, newStatus string) error {
	return r.db.Model(&Order{}).Where("order_sn = ?", orderSN).Update("wms_status", newStatus).Error
}

func (r *repository) UpdateOrderStatus(orderSN, status string) error {
	return r.db.Model(&Order{}).Where("order_sn = ?", orderSN).Update("marketplace_status", status).Error
}

func (r *repository) UpdateShippingStatus(orderSN, status string) error {
	return r.db.Model(&Order{}).Where("order_sn = ?", orderSN).Update("shipping_status", status).Error
}

func (r *repository) UpdateMarketplaceStatus(orderSN, mpStatus, shipStatus, tracking string) error {
	updates := map[string]interface{}{
		"marketplace_status": mpStatus,
		"shipping_status":    shipStatus,
	}
	if tracking != "" {
		updates["tracking_number"] = tracking
	}
	return r.db.Model(&Order{}).Where("order_sn = ?", orderSN).Updates(updates).Error
}

func (r *repository) UpdateShipmentInfo(orderSN, trackingNum, shippingStatus, shippingChannel, wmsStatus string) error {
	updates := map[string]interface{}{
		"tracking_number":  trackingNum,
		"shipping_status":  shippingStatus,
		"shipping_channel": shippingChannel,
		"wms_status":       wmsStatus,
	}
	return r.db.Model(&Order{}).Where("order_sn = ?", orderSN).Updates(updates).Error
}
