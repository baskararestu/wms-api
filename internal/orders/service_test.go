package orders

import (
	"testing"

	"github.com/baskararestu/wms-api/internal/integrations/marketplace"
	"github.com/google/uuid"
)

type mockOrderRepo struct {
	findOrderBySNRtn   *Order
	findOrderBySNErr   error
	updateShipmentErr  error
	updateShipmentCall bool
	updateCancelErr    error
	updateCancelCall   bool
}

func (m *mockOrderRepo) FindOrders(query GetOrderListQuery) ([]Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderRepo) FindOrderSummaryStats() (int64, int64, error) { return 0, 0, nil }
func (m *mockOrderRepo) FindOrderByID(id uuid.UUID) (*Order, error)   { return nil, nil }
func (m *mockOrderRepo) FindOrderBySN(orderSN string) (*Order, error) {
	return m.findOrderBySNRtn, m.findOrderBySNErr
}
func (m *mockOrderRepo) UpsertOrder(order *Order) error                       { return nil }
func (m *mockOrderRepo) UpdateWMSStatus(id uuid.UUID, newStatus string) error { return nil }
func (m *mockOrderRepo) UpdateWMSStatusBySN(orderSN, newStatus string) error  { return nil }
func (m *mockOrderRepo) UpdateOrderStatus(orderSN, status string) error       { return nil }
func (m *mockOrderRepo) UpdateShippingStatus(orderSN, status string) error    { return nil }
func (m *mockOrderRepo) UpdateMarketplaceStatus(orderSN, mpStatus, shipStatus, tracking string) error {
	return nil
}
func (m *mockOrderRepo) UpdateShipmentInfo(orderSN, trackingNum, shippingStatus, shippingChannel, wmsStatus string) error {
	m.updateShipmentCall = true
	return m.updateShipmentErr
}

func (m *mockOrderRepo) UpdateCancellationInfo(orderSN, mpStatus, shipStatus, wmsStatus string) error {
	m.updateCancelCall = true
	return m.updateCancelErr
}

type mockMarketplaceSvc struct {
	marketplace.Service
	cancelOrderRtn  *marketplace.CancelOrderResponse
	cancelOrderErr  error
	cancelOrderCall bool
	shipOrderRtn    *marketplace.ShipExternalOrderResponse
	shipOrderErr    error
	shipOrderCall   bool
}

func (m *mockMarketplaceSvc) CancelOrder(shopID, orderSN string) (*marketplace.CancelOrderResponse, error) {
	m.cancelOrderCall = true
	return m.cancelOrderRtn, m.cancelOrderErr
}

func (m *mockMarketplaceSvc) ShipOrder(shopID, orderSN, channelID string) (*marketplace.ShipExternalOrderResponse, error) {
	m.shipOrderCall = true
	return m.shipOrderRtn, m.shipOrderErr
}

func (m *mockMarketplaceSvc) NotifyOrderStatus(orderSN, status string) error {
	return nil
}

func (m *mockMarketplaceSvc) NotifyShippingStatus(orderSN, status string) error {
	return nil
}

func TestShipOrder_Success(t *testing.T) {
	repo := &mockOrderRepo{
		findOrderBySNRtn: &Order{
			OrderSN:           "SHP001",
			ShopID:            "shop-1",
			WMSStatus:         WMSStatusPacked, // Step 1: Pre-condition
			MarketplaceStatus: MPStatusPaid,    // actionable status
		},
	}

	mpSvc := &mockMarketplaceSvc{
		shipOrderRtn: &marketplace.ShipExternalOrderResponse{
			Message: "Order shipped",
		},
	}
	mpSvc.shipOrderRtn.Data.OrderSN = "SHP001"
	mpSvc.shipOrderRtn.Data.ShippingStatus = "shipped"
	mpSvc.shipOrderRtn.Data.TrackingNo = "TRK-12345"

	svc := NewService(repo, mpSvc)

	res, err := svc.ShipOrder("SHP001", "JNE")

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	// Step 2 & 3: Assert marketplace called and responded
	if !mpSvc.shipOrderCall {
		t.Errorf("expected marketplace api to be called")
	}

	// Step 4 & 5: Assert DB persisted info and wms_status
	if !repo.updateShipmentCall {
		t.Errorf("expected update shipment info db query to be called")
	}

	if res.TrackingNumber != "TRK-12345" {
		t.Errorf("expected tracking number TRK-12345, got %s", res.TrackingNumber)
	}
	if res.WMSStatus != WMSStatusShipped {
		t.Errorf("expected wms status %s, got %s", WMSStatusShipped, res.WMSStatus)
	}
}

func TestShipOrder_FailsValidationIfNotPacked(t *testing.T) {
	repo := &mockOrderRepo{
		findOrderBySNRtn: &Order{
			OrderSN:           "SHP001",
			ShopID:            "shop-1",
			WMSStatus:         WMSStatusPicking, // Step 1: fails
			MarketplaceStatus: MPStatusPaid,     // actionable, so error comes from wms_status check
		},
	}
	mpSvc := &mockMarketplaceSvc{}

	svc := NewService(repo, mpSvc)

	_, err := svc.ShipOrder("SHP001", "JNE")

	if err == nil {
		t.Fatalf("expected error when wms_status is not PACKED")
	}

	// Assert: Marketplace is never reached
	if mpSvc.shipOrderCall {
		t.Errorf("expected marketplace NOT to be called when invalid status")
	}
	// Assert: DB never saves anything
	if repo.updateShipmentCall {
		t.Errorf("expected db NOT to be updated when invalid status")
	}
}

func TestCancelOrder_Success(t *testing.T) {
	repo := &mockOrderRepo{
		findOrderBySNRtn: &Order{
			OrderSN:           "SHP001",
			ShopID:            "shop-1",
			WMSStatus:         WMSStatusReadyToPickup,
			MarketplaceStatus: MPStatusPaid,
		},
	}

	mpSvc := &mockMarketplaceSvc{
		cancelOrderRtn: &marketplace.CancelOrderResponse{Message: "Order cancelled"},
	}
	mpSvc.cancelOrderRtn.Data.OrderSN = "SHP001"
	mpSvc.cancelOrderRtn.Data.Status = MPStatusCancelled
	mpSvc.cancelOrderRtn.Data.ShippingStatus = MPStatusCancelled

	svc := NewService(repo, mpSvc)

	res, err := svc.CancelOrder("SHP001")
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	if !mpSvc.cancelOrderCall {
		t.Errorf("expected marketplace cancel api to be called")
	}

	if !repo.updateCancelCall {
		t.Errorf("expected cancellation info db query to be called")
	}

	if res.Data.Status != MPStatusCancelled {
		t.Errorf("expected cancelled status, got %s", res.Data.Status)
	}
	if res.Data.ShippingStatus != MPStatusCancelled {
		t.Errorf("expected cancelled shipping status, got %s", res.Data.ShippingStatus)
	}
}

func TestCancelOrder_FailsWhenAlreadyShipped(t *testing.T) {
	repo := &mockOrderRepo{
		findOrderBySNRtn: &Order{
			OrderSN:           "SHP001",
			ShopID:            "shop-1",
			WMSStatus:         WMSStatusShipped,
			MarketplaceStatus: MPStatusShipping,
		},
	}
	mpSvc := &mockMarketplaceSvc{}

	svc := NewService(repo, mpSvc)

	_, err := svc.CancelOrder("SHP001")
	if err == nil {
		t.Fatalf("expected error when order already shipped")
	}
	if mpSvc.cancelOrderCall {
		t.Errorf("expected marketplace cancel NOT to be called when order already shipped")
	}
	if repo.updateCancelCall {
		t.Errorf("expected db cancellation NOT to be updated when order already shipped")
	}
}
