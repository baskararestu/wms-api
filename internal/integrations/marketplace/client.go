package marketplace

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
)

const (
	PathAuthorize = "/oauth/authorize"
	PathToken     = "/oauth/token"
	StateParam    = "pm"
)

type Client interface {
	Authorize(shopID string) (*AuthCodeResponse, error)
	GetToken(code string) (*TokenResponse, error)
	RefreshToken(refreshToken string) (*TokenResponse, error)
}

type client struct {
	baseURL     string
	partnerID   string
	partnerKey  string
	httpClient  *http.Client
}

func NewClient() Client {
	return &client{
		baseURL:    config.App.MarketplaceBaseURL,
		partnerID:  config.App.PartnerID,
		partnerKey: config.App.PartnerKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *client) generateSignature(apiPath string, timestamp int64, shopID string) string {
	var base string
	if shopID != "" {
		// Used for /oauth/authorize -> partnerId + apiPath + timestamp + shopId
		base = fmt.Sprintf("%s%s%d%s", c.partnerID, apiPath, timestamp, shopID)
	} else {
		// Used for /oauth/token -> partnerId + apiPath + timestamp
		base = fmt.Sprintf("%s%s%d", c.partnerID, apiPath, timestamp)
	}

	h := hmac.New(sha256.New, []byte(c.partnerKey))
	h.Write([]byte(base))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *client) Authorize(shopID string) (*AuthCodeResponse, error) {
	timestamp := time.Now().Unix()
	sign := c.generateSignature(PathAuthorize, timestamp, shopID)

	url := fmt.Sprintf("%s%s?shop_id=%s&state=%s&partner_id=%s&timestamp=%d&sign=%s&redirect=https://example.com/callback",
		c.baseURL, PathAuthorize, shopID, StateParam, c.partnerID, timestamp, sign)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorize failed: %d", resp.StatusCode)
	}

	var authResp AuthCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}
	return &authResp, nil
}

func (c *client) GetToken(code string) (*TokenResponse, error) {
	timestamp := time.Now().Unix()
	sign := c.generateSignature(PathToken, timestamp, "")

	url := fmt.Sprintf("%s%s?partner_id=%s&timestamp=%d&sign=%s", 
		c.baseURL, PathToken, c.partnerID, timestamp, sign)

	payload := TokenRequest{
		GrantType: "authorization_code",
		Code:      code,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

func (c *client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	timestamp := time.Now().Unix()
	sign := c.generateSignature(PathToken, timestamp, "")

	url := fmt.Sprintf("%s%s?partner_id=%s&timestamp=%d&sign=%s", 
		c.baseURL, PathToken, c.partnerID, timestamp, sign)

	payload := TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}
