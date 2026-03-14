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

type mockMarketplaceSvc struct {
	marketplace.Service
	shipOrderRtn  *marketplace.ShipExternalOrderResponse
	shipOrderErr  error
	shipOrderCall bool
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
			OrderSN:   "SHP001",
			ShopID:    "shop-1",
			WMSStatus: WMSStatusPacked, // Step 1: Pre-condition
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
			OrderSN:   "SHP001",
			ShopID:    "shop-1",
			WMSStatus: WMSStatusPicking, // Step 1: fails
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
