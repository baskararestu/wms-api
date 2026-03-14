package marketplace

import (
	"errors"
	"log/slog"

	"github.com/baskararestu/wms-api/internal/auth"
)

// Service defines the orchestration logic for marketplace integrations
type Service interface {
	LinkShop(shopID string) error
}

type service struct {
	client   Client
	authRepo auth.Repository
}

// NewService creates a new Marketplace Integration Service
func NewService(client Client, authRepo auth.Repository) Service {
	return &service{
		client:   client,
		authRepo: authRepo,
	}
}

// LinkShop performs the full OAuth flow automatically
func (s *service) LinkShop(shopID string) error {
	slog.Info("Starting OAuth flow for shop", "shop_id", shopID)

	// 1. Get Authorization Code
	authResp, err := s.client.Authorize(shopID)
	if err != nil {
		slog.Error("Failed to authorize shop", "shop_id", shopID, "error", err)
		return errors.New("failed to retrieve authorization code from marketplace")
	}

	// 2. Exchange Code for Token
	tokenResp, err := s.client.GetToken(authResp.Data.Code)
	if err != nil {
		slog.Error("Failed to exchange token", "shop_id", shopID, "error", err)
		return errors.New("failed to exchange authorization code for access token")
	}

	// 3. Save directly to Database using the Auth Repository
	cred := &auth.MarketplaceCredential{
		ShopID:       shopID,
		AccessToken:  tokenResp.Data.AccessToken,
		RefreshToken: tokenResp.Data.RefreshToken,
	}
	
	err = s.authRepo.UpsertMarketplaceCredential(cred, tokenResp.Data.ExpiresIn)
	if err != nil {
		slog.Error("Failed to save credentials to DB", "shop_id", shopID, "error", err)
		return errors.New("failed to save marketplace credentials")
	}

	slog.Info("Successfully linked shop and stored credentials", "shop_id", shopID)
	return nil
}
