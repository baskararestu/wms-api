package marketplace

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/baskararestu/wms-api/internal/redis"
	goredis "github.com/redis/go-redis/v9"
)

// Service defines the orchestration logic for marketplace integrations
type Service interface {
	StartLinkShop(shopID string) (*LinkShopStartResponse, error)
	CompleteLinkShop(code, shopID, state string) error
	GetShopDetailByShopID(shopID string) (*ShopDetailResponse, error)
	GetOrderListByShopID(shopID string) (*OrderListResponse, error)
	GetOrderDetailByShopID(shopID, orderSN string) (*OrderDetailResponse, error)
	ShipOrder(shopID, orderSN, channelID string) (*ShipExternalOrderResponse, error)
	GetLogisticChannelsByShopID(shopID string) (*LogisticChannelsResponse, error)
	NotifyOrderStatus(orderSN, status string) error
	NotifyShippingStatus(orderSN, status string) error
}

type service struct {
	client      Client
	repo        Repository
	redirectURL string
}

var (
	ErrMarketplaceUnavailable = errors.New("marketplace service temporarily unavailable, please retry")
	ErrShopNotConnected       = errors.New("shop is not connected")
)

// NewService creates a new Marketplace Integration Service
func NewService(client Client, repo Repository, redirectURL string) Service {
	return &service{
		client:      client,
		repo:        repo,
		redirectURL: redirectURL,
	}
}

func (s *service) StartLinkShop(shopID string) (*LinkShopStartResponse, error) {
	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Starting one-step connect flow for shop")

	authResp, authorizeCtx, err := s.client.Authorize(shopID, s.redirectURL)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to authorize shop")
		if isMarketplaceUnavailableError(err) {
			return nil, ErrMarketplaceUnavailable
		}
		return nil, errors.New("failed to authorize shop with marketplace")
	}

	xlogger.Logger.Info().Str("shop_id", shopID).Interface("auth_response", authResp).Msg("Received authorize response from marketplace")

	if authResp == nil || authResp.Data.Code == "" || authResp.Data.ShopID == "" || authResp.Data.State == "" {
		return nil, errors.New("invalid authorize response from marketplace")
	}

	if err := s.cacheOAuthState(authResp.Data.State, authResp.Data.ShopID); err != nil {
		xlogger.Logger.Warn().Str("shop_id", shopID).Err(err).Msg("Failed to cache oauth state")
	}

	if err := s.completeLinkShopWithAuthorizeContext(authResp.Data.Code, authResp.Data.ShopID, authResp.Data.State, authorizeCtx); err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to complete one-step shop connect")
		return nil, err
	}

	return &LinkShopStartResponse{
		ShopID:    authResp.Data.ShopID,
		Connected: true,
	}, nil
}

func (s *service) CompleteLinkShop(code, shopID, state string) error {
	return s.completeLinkShopWithAuthorizeContext(code, shopID, state, nil)
}

func (s *service) completeLinkShopWithAuthorizeContext(code, shopID, state string, authorizeCtx *AuthorizeContext) error {
	if err := s.validateOAuthState(state, shopID); err != nil {
		return err
	}

	tokenResp, err := s.client.GetToken(code, authorizeCtx)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to exchange token")
		if isMarketplaceUnavailableError(err) {
			return ErrMarketplaceUnavailable
		}
		return errors.New("failed to exchange authorization code for access token")
	}

	shopDetail, err := s.client.GetShopDetail(tokenResp.Data.AccessToken)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to fetch shop detail during connect")
		if isMarketplaceUnavailableError(err) {
			return ErrMarketplaceUnavailable
		}
		return errors.New("failed to fetch shop detail from marketplace")
	}

	if shopDetail == nil || strings.TrimSpace(shopDetail.Data.ShopName) == "" {
		return errors.New("invalid shop detail from marketplace")
	}

	// Save token credential into marketplace credential storage
	cred := &MarketplaceCredential{
		Marketplace:  strings.TrimSpace(shopDetail.Data.ShopName),
		ShopID:       shopID,
		AccessToken:  tokenResp.Data.AccessToken,
		RefreshToken: tokenResp.Data.RefreshToken,
	}

	err = s.repo.UpsertMarketplaceCredential(cred, tokenResp.Data.ExpiresIn)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to save credentials to DB")
		return errors.New("failed to save marketplace credentials")
	}

	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Successfully linked shop and stored credentials")
	s.deleteOAuthState(state)
	return nil
}

func (s *service) GetShopDetailByShopID(shopID string) (*ShopDetailResponse, error) {
	cred, err := s.repo.FindMarketplaceCredentialByShopID(shopID)
	if err != nil {
		return nil, ErrShopNotConnected
	}

	accessToken, err := s.getValidAccessToken(cred)
	if err != nil {
		return nil, err
	}

	shopDetail, err := s.client.GetShopDetail(accessToken)
	if err != nil {
		return nil, errors.New("failed to fetch shop detail")
	}

	return shopDetail, nil
}

func (s *service) GetOrderListByShopID(shopID string) (*OrderListResponse, error) {
	cred, err := s.repo.FindMarketplaceCredentialByShopID(shopID)
	if err != nil {
		return nil, ErrShopNotConnected
	}

	accessToken, err := s.getValidAccessToken(cred)
	if err != nil {
		return nil, err
	}

	return s.client.GetOrderList(accessToken)
}

func (s *service) GetOrderDetailByShopID(shopID, orderSN string) (*OrderDetailResponse, error) {
	cred, err := s.repo.FindMarketplaceCredentialByShopID(shopID)
	if err != nil {
		return nil, ErrShopNotConnected
	}

	accessToken, err := s.getValidAccessToken(cred)
	if err != nil {
		return nil, err
	}

	return s.client.GetOrderDetail(accessToken, orderSN)
}

func (s *service) ShipOrder(shopID, orderSN, channelID string) (*ShipExternalOrderResponse, error) {
	cred, err := s.repo.FindMarketplaceCredentialByShopID(shopID)
	if err != nil {
		return nil, ErrShopNotConnected
	}

	accessToken, err := s.getValidAccessToken(cred)
	if err != nil {
		return nil, err
	}

	req := ShipExternalOrderRequest{
		OrderSN:   orderSN,
		ChannelID: channelID,
	}

	return s.client.ShipOrder(accessToken, req)
}

func (s *service) GetLogisticChannelsByShopID(shopID string) (*LogisticChannelsResponse, error) {
	// Let's check cache first
	cacheKey := "marketplace:logistic:channels:" + shopID
	if redis.Client != nil {
		cached, err := redis.Client.Get(redis.Ctx, cacheKey).Result()
		if err == nil && cached != "" {
			var resp LogisticChannelsResponse
			if json.Unmarshal([]byte(cached), &resp) == nil {
				return &resp, nil
			}
		}
	}

	cred, err := s.repo.FindMarketplaceCredentialByShopID(shopID)
	if err != nil {
		return nil, ErrShopNotConnected
	}

	accessToken, err := s.getValidAccessToken(cred)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.GetLogisticChannels(accessToken)
	if err != nil {
		return nil, err
	}

	// Cache the response
	if redis.Client != nil {
		if respBytes, err := json.Marshal(resp); err == nil {
			// Cache for 30 seconds as requested
			_ = redis.Client.Set(redis.Ctx, cacheKey, respBytes, 30*time.Second).Err()
		}
	}

	return resp, nil
}

func (s *service) NotifyOrderStatus(orderSN, status string) error {
	req := WebhookStatusNotifyRequest{OrderSN: orderSN, Status: status}
	_, err := s.client.NotifyOrderStatus(req)
	return err
}

func (s *service) NotifyShippingStatus(orderSN, status string) error {
	req := WebhookStatusNotifyRequest{OrderSN: orderSN, Status: status}
	_, err := s.client.NotifyShippingStatus(req)
	return err
}

func (s *service) getValidAccessToken(cred *MarketplaceCredential) (string, error) {
	if time.Now().Before(cred.ExpiresAt.Add(-30 * time.Second)) {
		return cred.AccessToken, nil
	}

	tokenResp, err := s.client.RefreshToken(cred.RefreshToken)
	if err != nil {
		if isMarketplaceUnavailableError(err) {
			return "", ErrMarketplaceUnavailable
		}
		return "", errors.New("failed to refresh marketplace token")
	}

	cred.AccessToken = tokenResp.Data.AccessToken
	cred.RefreshToken = tokenResp.Data.RefreshToken
	if err := s.repo.UpsertMarketplaceCredential(cred, tokenResp.Data.ExpiresIn); err != nil {
		return "", errors.New("failed to persist refreshed marketplace token")
	}

	return tokenResp.Data.AccessToken, nil
}

func (s *service) oauthStateKey(state string) string {
	return "marketplace:oauth:state:" + state
}

func (s *service) cacheOAuthState(state, shopID string) error {
	if redis.Client == nil {
		return nil
	}

	return redis.Client.Set(redis.Ctx, s.oauthStateKey(state), shopID, 10*time.Minute).Err()
}

func (s *service) validateOAuthState(state, shopID string) error {
	if redis.Client == nil {
		return nil
	}

	storedShopID, err := redis.Client.Get(redis.Ctx, s.oauthStateKey(state)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return errors.New("invalid oauth state")
		}
		return errors.New("failed to validate oauth state")
	}

	if storedShopID != shopID {
		return errors.New("oauth state does not match shop")
	}

	return nil
}

func (s *service) deleteOAuthState(state string) {
	if redis.Client == nil {
		return
	}

	_ = redis.Client.Del(redis.Ctx, s.oauthStateKey(state)).Err()
}

func isMarketplaceUnavailableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "status=500") ||
		strings.Contains(errMsg, "status=502") ||
		strings.Contains(errMsg, "status=503") ||
		strings.Contains(errMsg, "status=504") ||
		strings.Contains(errMsg, "temporarily unavailable") ||
		strings.Contains(errMsg, "timeout")
}
