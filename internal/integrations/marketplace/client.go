package marketplace

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
)

const (
	PathAuthorize = "/oauth/authorize"
	PathToken     = "/oauth/token"
	StateParam    = "pm"
	maxRetryCount = 3
	retryDelay    = 300 * time.Millisecond
)

type Client interface {
	Authorize(shopID string, redirectURL string) (*AuthCodeResponse, *AuthorizeContext, error)
	GetToken(code string, authorizeCtx *AuthorizeContext) (*TokenResponse, error)
	RefreshToken(refreshToken string) (*TokenResponse, error)
	GetShopDetail(accessToken string) (*ShopDetailResponse, error)
	GetOrderList(accessToken string) (*OrderListResponse, error)
	GetOrderDetail(accessToken, orderSN string) (*OrderDetailResponse, error)
	CancelOrder(accessToken string, req CancelOrderRequest) (*CancelOrderResponse, error)
	ShipOrder(accessToken string, req ShipExternalOrderRequest) (*ShipExternalOrderResponse, error)
	GetLogisticChannels(accessToken string) (*LogisticChannelsResponse, error)
	NotifyOrderStatus(req WebhookStatusNotifyRequest) (*OrderStatusNotifyResponse, error)
	NotifyShippingStatus(req WebhookStatusNotifyRequest) (*ShippingStatusNotifyResponse, error)
}

type AuthorizeContext struct {
	ShopID    string
	Timestamp int64
	Sign      string
}

type client struct {
	baseURL    string
	partnerID  string
	partnerKey string
	httpClient *http.Client
}

func buildHTTPError(operation string, req *http.Request, resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := strings.TrimSpace(string(bodyBytes))
	if body == "" {
		body = "<empty>"
	}

	return fmt.Errorf(
		"%s failed: method=%s url=%s status=%d body=%q",
		operation,
		req.Method,
		req.URL.String(),
		resp.StatusCode,
		body,
	)
}

func parseRetryAfter(headerValue string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(headerValue)
	if trimmed == "" {
		return 0, false
	}

	if seconds, err := time.ParseDuration(trimmed + "s"); err == nil {
		if seconds > 0 {
			return seconds, true
		}
	}

	if retryAt, err := http.ParseTime(trimmed); err == nil {
		delay := time.Until(retryAt)
		if delay > 0 {
			return delay, true
		}
	}

	return 0, false
}

func backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	delay := retryDelay * time.Duration(1<<uint(attempt-1))
	maxDelay := 5 * time.Second
	if delay > maxDelay {
		return maxDelay
	}

	return delay
}

func shouldRetryStatus(resp *http.Response, attempt int) (time.Duration, bool) {
	if attempt >= maxRetryCount {
		return 0, false
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return retryAfter, true
		}
		return backoffDelay(attempt), true
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return backoffDelay(attempt), true
	}

	return 0, false
}

func NewClient() Client {
	return &client{
		baseURL:    config.App.MarketplaceBaseURL,
		partnerID:  config.App.PartnerID,
		partnerKey: config.App.PartnerKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *client) generateSignature(apiPath string, timestamp int64, extra string) string {
	var base string
	if extra != "" {
		base = fmt.Sprintf("%s%s%d%s", c.partnerID, apiPath, timestamp, extra)
	} else {
		base = fmt.Sprintf("%s%s%d", c.partnerID, apiPath, timestamp)
	}

	h := hmac.New(sha256.New, []byte(c.partnerKey))
	h.Write([]byte(base))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *client) Authorize(shopID string, redirectURL string) (*AuthCodeResponse, *AuthorizeContext, error) {
	timestamp := time.Now().Unix()
	sign := c.generateSignature(PathAuthorize, timestamp, shopID)
	if redirectURL == "" {
		redirectURL = "https://example.com/callback"
	}

	reqURL := fmt.Sprintf("%s%s?shop_id=%s&state=%s&partner_id=%s&timestamp=%d&sign=%s&redirect=%s",
		c.baseURL, PathAuthorize, shopID, StateParam, c.partnerID, timestamp, sign, url.QueryEscape(redirectURL))
	// log timestamp and sign for debugging
	xlogger.Logger.Info().Str("shop_id", shopID).Int64("timestamp", timestamp).Str("sign", sign).Msg("Generated authorize URL with signature")
	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Add("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("authorize", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, nil, httpErr
		}

		var authResp AuthCodeResponse
		err = json.NewDecoder(resp.Body).Decode(&authResp)
		resp.Body.Close()
		if err != nil {
			return nil, nil, err
		}
		return &authResp, &AuthorizeContext{
			ShopID:    shopID,
			Timestamp: timestamp,
			Sign:      sign,
		}, nil
	}

	if lastErr != nil {
		return nil, nil, lastErr
	}

	return nil, nil, fmt.Errorf("authorize failed after retries")
}

func (c *client) GetToken(code string, authorizeCtx *AuthorizeContext) (*TokenResponse, error) {
	timestamp := time.Now().Unix()
	if authorizeCtx != nil && authorizeCtx.Timestamp > 0 {
		timestamp = authorizeCtx.Timestamp
	}
	sign := c.generateSignature(PathToken, timestamp, code)

	url := fmt.Sprintf("%s%s?partner_id=%s&timestamp=%d&sign=%s",
		c.baseURL, PathToken, c.partnerID, timestamp, sign)
	payload := TokenRequest{
		GrantType: "authorization_code",
		Code:      code,
	}
	// log timestamp and sign for debugging
	xlogger.Logger.Info().Int64("timestamp", timestamp).Str("sign", sign).Msg("Generated token exchange request with signature")
	body, _ := json.Marshal(payload)

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("token exchange", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var tokenResp TokenResponse
		err = json.NewDecoder(resp.Body).Decode(&tokenResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return &tokenResp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("token exchange failed after retries")
}

func (c *client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	timestamp := time.Now().Unix()
	sign := c.generateSignature(PathToken, timestamp, refreshToken)

	url := fmt.Sprintf("%s%s?partner_id=%s&timestamp=%d&sign=%s",
		c.baseURL, PathToken, c.partnerID, timestamp, sign)

	payload := TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	}
	body, _ := json.Marshal(payload)

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("token refresh", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var tokenResp TokenResponse
		err = json.NewDecoder(resp.Body).Decode(&tokenResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return &tokenResp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("token refresh failed after retries")
}

func (c *client) GetShopDetail(accessToken string) (*ShopDetailResponse, error) {
	url := fmt.Sprintf("%s/shop/get", c.baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(backoffDelay(attempt))
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("get shop detail", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var shopResp ShopDetailResponse
		err = json.NewDecoder(resp.Body).Decode(&shopResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &shopResp, nil
	}

	return nil, lastErr
}

func (c *client) GetOrderList(accessToken string) (*OrderListResponse, error) {
	url := fmt.Sprintf("%s/order/list", c.baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("get order list", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var listResp OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &listResp, nil
	}

	return nil, lastErr
}

func (c *client) GetOrderDetail(accessToken, orderSN string) (*OrderDetailResponse, error) {
	url := fmt.Sprintf("%s/order/detail?order_sn=%s", c.baseURL, orderSN)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("get order detail", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var detailResp OrderDetailResponse
		err = json.NewDecoder(resp.Body).Decode(&detailResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &detailResp, nil
	}

	return nil, lastErr
}

func (c *client) CancelOrder(accessToken string, req CancelOrderRequest) (*CancelOrderResponse, error) {
	url := fmt.Sprintf("%s/order/cancel", c.baseURL)

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, err
		}

		httpReq.Header.Add("Accept", "application/json")
		httpReq.Header.Add("Content-Type", "application/json")
		httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("cancel order", httpReq, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var cancelResp CancelOrderResponse
		err = json.NewDecoder(resp.Body).Decode(&cancelResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &cancelResp, nil
	}

	return nil, lastErr
}

func (c *client) ShipOrder(accessToken string, req ShipExternalOrderRequest) (*ShipExternalOrderResponse, error) {
	url := fmt.Sprintf("%s/logistic/ship", c.baseURL)

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Add("Accept", "application/json")
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("ship order", httpReq, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var shipResp ShipExternalOrderResponse
		err = json.NewDecoder(resp.Body).Decode(&shipResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &shipResp, nil
	}

	return nil, lastErr
}

func (c *client) GetLogisticChannels(accessToken string) (*LogisticChannelsResponse, error) {
	url := fmt.Sprintf("%s/logistic/channels", c.baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("get logistic channels", req, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var channelsResp LogisticChannelsResponse
		err = json.NewDecoder(resp.Body).Decode(&channelsResp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &channelsResp, nil
	}

	return nil, lastErr
}

func (c *client) NotifyOrderStatus(req WebhookStatusNotifyRequest) (*OrderStatusNotifyResponse, error) {
	url := fmt.Sprintf("%s/webhook/order-status", c.baseURL)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	xlogger.Logger.Info().
		Str("webhook", "order-status").
		Str("url", url).
		Str("order_sn", req.OrderSN).
		Str("status", req.Status).
		Msg("[Webhook] Dispatching order-status to marketplace")

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Add("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			xlogger.Logger.Error().
				Str("webhook", "order-status").
				Str("order_sn", req.OrderSN).
				Int("attempt", attempt).
				Err(err).
				Msg("[Webhook] order-status HTTP request failed")
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("notify order status", httpReq, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			xlogger.Logger.Warn().
				Str("webhook", "order-status").
				Str("order_sn", req.OrderSN).
				Int("http_status", resp.StatusCode).
				Int("attempt", attempt).
				Err(httpErr).
				Msg("[Webhook] order-status non-200 response from marketplace")
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var out OrderStatusNotifyResponse
		err = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		xlogger.Logger.Info().
			Str("webhook", "order-status").
			Str("order_sn", out.Data.OrderSN).
			Str("status", out.Data.Status).
			Int("http_status", resp.StatusCode).
			Msg("[Webhook] order-status acknowledged by marketplace")

		return &out, nil
	}

	return nil, lastErr
}

func (c *client) NotifyShippingStatus(req WebhookStatusNotifyRequest) (*ShippingStatusNotifyResponse, error) {
	url := fmt.Sprintf("%s/webhook/shipping-status", c.baseURL)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	xlogger.Logger.Info().
		Str("webhook", "shipping-status").
		Str("url", url).
		Str("order_sn", req.OrderSN).
		Str("status", req.Status).
		Msg("[Webhook] Dispatching shipping-status to marketplace")

	var lastErr error
	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Add("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			xlogger.Logger.Error().
				Str("webhook", "shipping-status").
				Str("order_sn", req.OrderSN).
				Int("attempt", attempt).
				Err(err).
				Msg("[Webhook] shipping-status HTTP request failed")
			lastErr = err
			if attempt < maxRetryCount {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := buildHTTPError("notify shipping status", httpReq, resp)
			retryDelay, shouldRetry := shouldRetryStatus(resp, attempt)
			resp.Body.Close()
			xlogger.Logger.Warn().
				Str("webhook", "shipping-status").
				Str("order_sn", req.OrderSN).
				Int("http_status", resp.StatusCode).
				Int("attempt", attempt).
				Err(httpErr).
				Msg("[Webhook] shipping-status non-200 response from marketplace")
			lastErr = httpErr
			if shouldRetry {
				time.Sleep(retryDelay)
				continue
			}
			return nil, httpErr
		}

		var out ShippingStatusNotifyResponse
		err = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		xlogger.Logger.Info().
			Str("webhook", "shipping-status").
			Str("order_sn", out.Data.OrderSN).
			Str("shipping_state", out.Data.ShippingState).
			Int("http_status", resp.StatusCode).
			Msg("[Webhook] shipping-status acknowledged by marketplace")

		return &out, nil
	}

	return nil, lastErr
}
