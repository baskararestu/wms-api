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
	ShipOrder(accessToken string, req ShipExternalOrderRequest) (*ShipExternalOrderResponse, error)
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, buildHTTPError("get shop detail", req, resp)
	}

	var shopResp ShopDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&shopResp); err != nil {
		return nil, err
	}

	return &shopResp, nil
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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
			resp.Body.Close()
			lastErr = httpErr
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetryCount {
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
