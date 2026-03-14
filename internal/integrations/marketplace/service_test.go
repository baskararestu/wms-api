package marketplace

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"gorm.io/gorm"
)

var setupLoggerOnce sync.Once

func setupTestLogger() {
	setupLoggerOnce.Do(func() {
		xlogger.Setup(config.Config{IsDevelopment: false})
	})
}

type mockMarketplaceClient struct {
	authResp     *AuthCodeResponse
	tokenResp    *TokenResponse
	shopResp     *ShopDetailResponse
	authorizeErr error
	getTokenErr  error
	refreshErr   error
	shopErr      error
	err          error
	shipResp     *ShipExternalOrderResponse
}

func (m *mockMarketplaceClient) Authorize(_ string, _ string) (*AuthCodeResponse, *AuthorizeContext, error) {
	if m.authorizeErr != nil {
		return nil, nil, m.authorizeErr
	}
	return m.authResp, &AuthorizeContext{ShopID: m.authResp.Data.ShopID, Timestamp: time.Now().Unix(), Sign: "mock-sign"}, nil
}

func (m *mockMarketplaceClient) GetToken(_ string, _ *AuthorizeContext) (*TokenResponse, error) {
	if m.getTokenErr != nil {
		return nil, m.getTokenErr
	}
	return m.tokenResp, nil
}

func (m *mockMarketplaceClient) RefreshToken(_ string) (*TokenResponse, error) {
	if m.refreshErr != nil {
		return nil, m.refreshErr
	}
	return m.tokenResp, nil
}

func (m *mockMarketplaceClient) GetShopDetail(_ string) (*ShopDetailResponse, error) {
	if m.shopErr != nil {
		return nil, m.shopErr
	}
	return m.shopResp, nil
}

func (m *mockMarketplaceClient) GetOrderList(_ string) (*OrderListResponse, error) {
	return nil, nil // not tested here
}

func (m *mockMarketplaceClient) GetOrderDetail(_, _ string) (*OrderDetailResponse, error) {
	return nil, nil // not tested here
}

func (m *mockMarketplaceClient) ShipOrder(accessToken string, req ShipExternalOrderRequest) (*ShipExternalOrderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.shipResp, nil
}

func (m *mockMarketplaceClient) GetLogisticChannels(accessToken string) (*LogisticChannelsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil // not fully mocked for existing auth tests yet
}

type mockMarketplaceRepo struct {
	credByShopID map[string]*MarketplaceCredential
	upsertCount  int
}

func newMockMarketplaceRepo() *mockMarketplaceRepo {
	return &mockMarketplaceRepo{
		credByShopID: make(map[string]*MarketplaceCredential),
	}
}

func (m *mockMarketplaceRepo) FindMarketplaceCredentialByShopID(shopID string) (*MarketplaceCredential, error) {
	cred, ok := m.credByShopID[shopID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return cred, nil
}

func (m *mockMarketplaceRepo) UpsertMarketplaceCredential(cred *MarketplaceCredential, expiresIn int) error {
	m.upsertCount++
	cred.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	m.credByShopID[cred.ShopID] = cred
	return nil
}

func TestStartLinkShopConnectsAndStoresCredential(t *testing.T) {
	setupTestLogger()

	client := &mockMarketplaceClient{
		authResp:  &AuthCodeResponse{},
		tokenResp: &TokenResponse{},
		shopResp:  &ShopDetailResponse{},
	}
	client.authResp.Data.Code = "auth-code"
	client.authResp.Data.ShopID = "shopee-123"
	client.authResp.Data.State = "pm"
	client.tokenResp.Data.AccessToken = "access-1"
	client.tokenResp.Data.RefreshToken = "refresh-1"
	client.tokenResp.Data.ExpiresIn = 300
	client.shopResp.Data.ShopID = "shopee-123"
	client.shopResp.Data.ShopName = "Shopee - General Store"

	repo := newMockMarketplaceRepo()
	svc := NewService(client, repo, config.App.RedirectURL)

	res, err := svc.StartLinkShop("shopee-123")
	if err != nil {
		t.Fatalf("expected start link shop success, got error: %v", err)
	}

	if !res.Connected {
		t.Fatal("expected connected=true")
	}

	if repo.upsertCount != 1 {
		t.Fatalf("expected one credential upsert, got %d", repo.upsertCount)
	}

	stored := repo.credByShopID["shopee-123"]
	if stored == nil || stored.Marketplace != "Shopee - General Store" {
		t.Fatalf("expected marketplace to be populated from shop name, got %+v", stored)
	}
}

func TestCompleteLinkShopStoresCredential(t *testing.T) {
	setupTestLogger()

	client := &mockMarketplaceClient{
		tokenResp: &TokenResponse{},
		shopResp:  &ShopDetailResponse{},
	}
	client.tokenResp.Data.AccessToken = "access-1"
	client.tokenResp.Data.RefreshToken = "refresh-1"
	client.tokenResp.Data.ExpiresIn = 300
	client.shopResp.Data.ShopID = "shopee-123"
	client.shopResp.Data.ShopName = "Shopee - General Store"

	repo := newMockMarketplaceRepo()
	svc := NewService(client, repo, config.App.RedirectURL)

	err := svc.CompleteLinkShop("auth-code", "shopee-123", "pm")
	if err != nil {
		t.Fatalf("expected complete link success, got error: %v", err)
	}

	if repo.upsertCount != 1 {
		t.Fatalf("expected one credential upsert, got %d", repo.upsertCount)
	}
}

func TestGetShopDetailByShopIDRefreshesExpiredToken(t *testing.T) {
	setupTestLogger()

	client := &mockMarketplaceClient{
		tokenResp: &TokenResponse{},
		shopResp:  &ShopDetailResponse{},
	}
	client.tokenResp.Data.AccessToken = "access-new"
	client.tokenResp.Data.RefreshToken = "refresh-new"
	client.tokenResp.Data.ExpiresIn = 300
	client.shopResp.Data.ShopID = "shopee-123"
	client.shopResp.Data.ShopName = "Shopee Test"

	repo := newMockMarketplaceRepo()
	repo.credByShopID["shopee-123"] = &MarketplaceCredential{
		ShopID:       "shopee-123",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-1 * time.Minute),
	}

	svc := NewService(client, repo, config.App.RedirectURL)

	res, err := svc.GetShopDetailByShopID("shopee-123")
	if err != nil {
		t.Fatalf("expected get shop detail success, got error: %v", err)
	}

	if res.Data.ShopID != "shopee-123" {
		t.Fatalf("expected shop id shopee-123, got %s", res.Data.ShopID)
	}

	if repo.upsertCount != 1 {
		t.Fatalf("expected token refresh upsert count 1, got %d", repo.upsertCount)
	}
}

func TestGetShopDetailByShopIDFailsWhenShopNotConnected(t *testing.T) {
	setupTestLogger()

	client := &mockMarketplaceClient{}
	repo := newMockMarketplaceRepo()
	svc := NewService(client, repo, config.App.RedirectURL)

	_, err := svc.GetShopDetailByShopID("missing-shop")
	if err == nil {
		t.Fatal("expected error when shop not connected")
	}
}

func TestStartLinkShopReturnsUnavailableWhenMarketplaceDown(t *testing.T) {
	setupTestLogger()

	client := &mockMarketplaceClient{
		authorizeErr: errors.New("authorize failed: status=500 body=\"temporarily unavailable\""),
	}
	repo := newMockMarketplaceRepo()
	svc := NewService(client, repo, config.App.RedirectURL)

	_, err := svc.StartLinkShop("shopee-123")
	if !errors.Is(err, ErrMarketplaceUnavailable) {
		t.Fatalf("expected ErrMarketplaceUnavailable, got %v", err)
	}
}
