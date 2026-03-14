package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	GetWebhookDeliveryMetrics() WebhookDeliveryMetrics
}

type service struct {
	client      Client
	repo        Repository
	redirectURL string

	dispatchSuccess    atomic.Uint64
	dispatchFailure    atomic.Uint64
	retryQueued        atomic.Uint64
	retrySuccess       atomic.Uint64
	retryFailure       atomic.Uint64
	retryDropped       atomic.Uint64
	idempotencySkipped atomic.Uint64
	inflightSkipped    atomic.Uint64

	metricsMu       sync.RWMutex
	lastRetryRunAt  time.Time
	lastErrorSample string
}

var (
	ErrMarketplaceUnavailable = errors.New("marketplace service temporarily unavailable, please retry")
	ErrShopNotConnected       = errors.New("shop is not connected")
)

const (
	webhookSentKeyTTL       = 24 * time.Hour
	webhookRetryPollEvery   = 5 * time.Second
	webhookRetryMaxAttempts = 5
	webhookRetryZSetKey     = "marketplace:webhook:retry:zset"
	webhookInflightLockTTL  = 30 * time.Second
)

type webhookRetryJob struct {
	Kind     string `json:"kind"`
	OrderSN  string `json:"order_sn"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
}

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
		if isMarketplaceUnauthorizedError(err) {
			refreshedToken, refreshErr := s.forceRefreshAccessToken(cred)
			if refreshErr != nil {
				return nil, refreshErr
			}

			shopDetail, err = s.client.GetShopDetail(refreshedToken)
			if err == nil {
				return shopDetail, nil
			}
		}
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

	resp, err := s.client.GetOrderList(accessToken)
	if err != nil {
		if isMarketplaceUnauthorizedError(err) {
			refreshedToken, refreshErr := s.forceRefreshAccessToken(cred)
			if refreshErr != nil {
				return nil, refreshErr
			}
			return s.client.GetOrderList(refreshedToken)
		}
		return nil, err
	}

	return resp, nil
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

	resp, err := s.client.GetOrderDetail(accessToken, orderSN)
	if err != nil {
		if isMarketplaceUnauthorizedError(err) {
			refreshedToken, refreshErr := s.forceRefreshAccessToken(cred)
			if refreshErr != nil {
				return nil, refreshErr
			}
			return s.client.GetOrderDetail(refreshedToken, orderSN)
		}
		return nil, err
	}

	return resp, nil
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

	resp, err := s.client.ShipOrder(accessToken, req)
	if err != nil {
		if isMarketplaceUnauthorizedError(err) {
			refreshedToken, refreshErr := s.forceRefreshAccessToken(cred)
			if refreshErr != nil {
				return nil, refreshErr
			}
			return s.client.ShipOrder(refreshedToken, req)
		}
		return nil, err
	}

	return resp, nil
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
		if isMarketplaceUnauthorizedError(err) {
			refreshedToken, refreshErr := s.forceRefreshAccessToken(cred)
			if refreshErr != nil {
				return nil, refreshErr
			}
			resp, err = s.client.GetLogisticChannels(refreshedToken)
		}
		if err != nil {
			return nil, err
		}
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
	return s.sendWebhookWithIdempotency("order-status", orderSN, status)
}

func (s *service) NotifyShippingStatus(orderSN, status string) error {
	return s.sendWebhookWithIdempotency("shipping-status", orderSN, status)
}

func (s *service) sendWebhookWithIdempotency(kind, orderSN, status string) error {
	alreadySent, err := s.wasWebhookAlreadySent(kind, orderSN, status)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Str("kind", kind).Str("order_sn", orderSN).Msg("Failed to check webhook sent key")
	}

	if alreadySent {
		s.idempotencySkipped.Add(1)
		return nil
	}

	lockAcquired, err := s.acquireWebhookInflightLock(kind, orderSN, status)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Str("kind", kind).Str("order_sn", orderSN).Msg("Failed to acquire webhook inflight lock")
	}
	if !lockAcquired {
		s.inflightSkipped.Add(1)
		return nil
	}
	defer s.releaseWebhookInflightLock(kind, orderSN, status)

	if err := s.dispatchWebhook(kind, orderSN, status); err != nil {
		s.dispatchFailure.Add(1)
		s.retryQueued.Add(1)
		s.setLastErrorSample(err)
		xlogger.Logger.Warn().Err(err).Str("kind", kind).Str("order_sn", orderSN).Msg("Failed to dispatch webhook, queued for retry")
		s.enqueueWebhookRetry(webhookRetryJob{
			Kind:     kind,
			OrderSN:  orderSN,
			Status:   status,
			Attempts: 1,
		}, 5*time.Second)
		return err
	}

	s.dispatchSuccess.Add(1)

	if err := s.markWebhookSent(kind, orderSN, status); err != nil {
		xlogger.Logger.Warn().Err(err).Str("kind", kind).Str("order_sn", orderSN).Msg("Failed to store webhook sent key")
	}

	return nil
}

func (s *service) dispatchWebhook(kind, orderSN, status string) error {
	req := WebhookStatusNotifyRequest{OrderSN: orderSN, Status: status}

	switch kind {
	case "order-status":
		_, err := s.client.NotifyOrderStatus(req)
		return err
	case "shipping-status":
		_, err := s.client.NotifyShippingStatus(req)
		return err
	default:
		return fmt.Errorf("unsupported webhook kind: %s", kind)
	}
}

func (s *service) webhookSentKey(kind, orderSN, status string) string {
	return fmt.Sprintf(
		"marketplace:webhook:sent:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(kind)),
		strings.ToLower(strings.TrimSpace(orderSN)),
		strings.ToLower(strings.TrimSpace(status)),
	)
}

func (s *service) webhookInflightKey(kind, orderSN, status string) string {
	return fmt.Sprintf(
		"marketplace:webhook:inflight:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(kind)),
		strings.ToLower(strings.TrimSpace(orderSN)),
		strings.ToLower(strings.TrimSpace(status)),
	)
}

func (s *service) wasWebhookAlreadySent(kind, orderSN, status string) (bool, error) {
	if redis.Client == nil {
		return false, nil
	}

	exists, err := redis.Client.Exists(redis.Ctx, s.webhookSentKey(kind, orderSN, status)).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

func (s *service) markWebhookSent(kind, orderSN, status string) error {
	if redis.Client == nil {
		return nil
	}

	return redis.Client.Set(redis.Ctx, s.webhookSentKey(kind, orderSN, status), "1", webhookSentKeyTTL).Err()
}

func (s *service) acquireWebhookInflightLock(kind, orderSN, status string) (bool, error) {
	if redis.Client == nil {
		return true, nil
	}

	return redis.Client.SetNX(redis.Ctx, s.webhookInflightKey(kind, orderSN, status), "1", webhookInflightLockTTL).Result()
}

func (s *service) releaseWebhookInflightLock(kind, orderSN, status string) {
	if redis.Client == nil {
		return
	}

	_ = redis.Client.Del(redis.Ctx, s.webhookInflightKey(kind, orderSN, status)).Err()
}

func (s *service) retryQueuedKey(job webhookRetryJob) string {
	return fmt.Sprintf(
		"marketplace:webhook:retry:queued:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(job.Kind)),
		strings.ToLower(strings.TrimSpace(job.OrderSN)),
		strings.ToLower(strings.TrimSpace(job.Status)),
	)
}

func (s *service) enqueueWebhookRetry(job webhookRetryJob, delay time.Duration) {
	if redis.Client == nil {
		return
	}

	key := s.retryQueuedKey(job)
	created, err := redis.Client.SetNX(redis.Ctx, key, "1", 30*time.Minute).Result()
	if err != nil {
		xlogger.Logger.Warn().Err(err).Str("order_sn", job.OrderSN).Msg("Failed to set retry queued key")
		return
	}
	if !created {
		return
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		return
	}

	nextAt := float64(time.Now().Add(delay).Unix())
	if err := redis.Client.ZAdd(redis.Ctx, webhookRetryZSetKey, goredis.Z{Score: nextAt, Member: string(jobBytes)}).Err(); err != nil {
		_ = redis.Client.Del(redis.Ctx, key).Err()
		xlogger.Logger.Warn().Err(err).Str("order_sn", job.OrderSN).Msg("Failed to enqueue webhook retry")
	}
}

func StartWebhookRetryScheduler(svc Service) {
	s, ok := svc.(*service)
	if !ok {
		return
	}

	go func() {
		ticker := time.NewTicker(webhookRetryPollEvery)
		defer ticker.Stop()

		xlogger.Logger.Info().Msg("Marketplace webhook retry scheduler started")
		for range ticker.C {
			s.processWebhookRetries()
		}
	}()
}

func (s *service) processWebhookRetries() {
	if redis.Client == nil {
		return
	}

	s.setLastRetryRunAt(time.Now())

	now := fmt.Sprintf("%d", time.Now().Unix())
	jobs, err := redis.Client.ZRangeByScore(redis.Ctx, webhookRetryZSetKey, &goredis.ZRangeBy{
		Min:   "-inf",
		Max:   now,
		Count: 20,
	}).Result()
	if err != nil {
		s.setLastErrorSample(err)
		xlogger.Logger.Warn().Err(err).Msg("Failed to fetch webhook retry jobs")
		return
	}

	for _, raw := range jobs {
		var job webhookRetryJob
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			_ = redis.Client.ZRem(redis.Ctx, webhookRetryZSetKey, raw).Err()
			continue
		}

		alreadySent, _ := s.wasWebhookAlreadySent(job.Kind, job.OrderSN, job.Status)
		if alreadySent {
			s.idempotencySkipped.Add(1)
			s.cleanupRetryJob(job, raw)
			continue
		}

		lockAcquired, err := s.acquireWebhookInflightLock(job.Kind, job.OrderSN, job.Status)
		if err != nil {
			s.setLastErrorSample(err)
			xlogger.Logger.Warn().Err(err).Str("kind", job.Kind).Str("order_sn", job.OrderSN).Msg("Failed to acquire retry inflight lock")
			continue
		}
		if !lockAcquired {
			s.inflightSkipped.Add(1)
			continue
		}

		if err := s.dispatchWebhook(job.Kind, job.OrderSN, job.Status); err != nil {
			s.retryFailure.Add(1)
			s.setLastErrorSample(err)
			s.releaseWebhookInflightLock(job.Kind, job.OrderSN, job.Status)
			if job.Attempts >= webhookRetryMaxAttempts {
				s.retryDropped.Add(1)
				xlogger.Logger.Warn().Err(err).Str("kind", job.Kind).Str("order_sn", job.OrderSN).Msg("Dropping webhook retry job after max attempts")
				s.cleanupRetryJob(job, raw)
				continue
			}

			job.Attempts++
			s.retryQueued.Add(1)
			s.cleanupRetryJob(job, raw)
			s.enqueueWebhookRetry(job, time.Duration(job.Attempts)*5*time.Second)
			continue
		}

		s.retrySuccess.Add(1)
		_ = s.markWebhookSent(job.Kind, job.OrderSN, job.Status)
		s.releaseWebhookInflightLock(job.Kind, job.OrderSN, job.Status)
		s.cleanupRetryJob(job, raw)
	}
}

func (s *service) GetWebhookDeliveryMetrics() WebhookDeliveryMetrics {
	lastRetryRunAt, lastErrorSample := s.getRuntimeMetricsSnapshot()

	metrics := WebhookDeliveryMetrics{
		DispatchSuccess:    s.dispatchSuccess.Load(),
		DispatchFailure:    s.dispatchFailure.Load(),
		RetryQueued:        s.retryQueued.Load(),
		RetrySuccess:       s.retrySuccess.Load(),
		RetryFailure:       s.retryFailure.Load(),
		RetryDropped:       s.retryDropped.Load(),
		IdempotencySkipped: s.idempotencySkipped.Load(),
		InflightSkipped:    s.inflightSkipped.Load(),
		PendingRetry:       0,
		LastRetryRunAt:     lastRetryRunAt,
		LastErrorSample:    lastErrorSample,
	}

	if redis.Client != nil {
		pending, err := redis.Client.ZCard(redis.Ctx, webhookRetryZSetKey).Result()
		if err == nil {
			metrics.PendingRetry = pending
		}
	}

	return metrics
}

func (s *service) setLastRetryRunAt(t time.Time) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.lastRetryRunAt = t
}

func (s *service) setLastErrorSample(err error) {
	if err == nil {
		return
	}

	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.lastErrorSample = err.Error()
}

func (s *service) getRuntimeMetricsSnapshot() (string, string) {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	lastRun := ""
	if !s.lastRetryRunAt.IsZero() {
		lastRun = s.lastRetryRunAt.UTC().Format(time.RFC3339)
	}

	return lastRun, s.lastErrorSample
}

func (s *service) cleanupRetryJob(job webhookRetryJob, raw string) {
	if redis.Client == nil {
		return
	}

	_ = redis.Client.ZRem(redis.Ctx, webhookRetryZSetKey, raw).Err()
	_ = redis.Client.Del(redis.Ctx, s.retryQueuedKey(job)).Err()
}

func (s *service) getValidAccessToken(cred *MarketplaceCredential) (string, error) {
	if time.Now().Before(cred.ExpiresAt.Add(-30 * time.Second)) {
		return cred.AccessToken, nil
	}

	return s.forceRefreshAccessToken(cred)
}

func (s *service) forceRefreshAccessToken(cred *MarketplaceCredential) (string, error) {
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
		strings.Contains(errMsg, "status=429") ||
		strings.Contains(errMsg, "temporarily unavailable") ||
		strings.Contains(errMsg, "timeout")
}

func isMarketplaceUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "status=401") ||
		strings.Contains(errMsg, "unauthorized")
}
