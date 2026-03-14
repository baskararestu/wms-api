package marketplace

import (
	"errors"

	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
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
	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Starting OAuth flow for shop")

	// 1. Get Authorization Code
	authResp, err := s.client.Authorize(shopID)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to authorize shop")
		return errors.New("failed to retrieve authorization code from marketplace")
	}

	// 2. Exchange Code for Token
	tokenResp, err := s.client.GetToken(authResp.Data.Code)
	if err != nil {
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to exchange token")
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
		xlogger.Logger.Error().Str("shop_id", shopID).Err(err).Msg("Failed to save credentials to DB")
		return errors.New("failed to save marketplace credentials")
	}

	xlogger.Logger.Info().Str("shop_id", shopID).Msg("Successfully linked shop and stored credentials")
	return nil
}
