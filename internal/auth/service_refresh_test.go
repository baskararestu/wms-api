package auth

import (
	"testing"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/google/uuid"
)

func TestRefreshTokenRotationFallbackToDBWhenRedisMiss(t *testing.T) {
	config.App.JWTSecret = "test-secret"

	repo := newMockAuthRepository()
	userID := uuid.New()
	user := &User{
		BaseModel: BaseModel{ID: userID},
		Email:     "admin@wms.com",
	}
	repo.usersByID[userID] = user
	repo.usersByEmail[user.Email] = user

	rawRefreshToken := "raw-refresh-token"
	hashedRefreshToken := hashForTest(rawRefreshToken)
	oldToken := &RefreshToken{
		BaseModel: BaseModel{ID: uuid.New()},
		UserID:    userID,
		TokenHash: hashedRefreshToken,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	repo.tokensByHash[hashedRefreshToken] = oldToken

	svc := NewService(repo)

	res, err := svc.RefreshToken(RefreshTokenRequest{RefreshToken: rawRefreshToken})
	if err != nil {
		t.Fatalf("expected refresh success, got error: %v", err)
	}

	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("expected both access and refresh token in response")
	}

	if oldToken.RevokedAt == nil {
		t.Fatal("expected old refresh token to be revoked")
	}

	if len(repo.tokensByHash) != 2 {
		t.Fatalf("expected token rotation to create new token, got %d total tokens", len(repo.tokensByHash))
	}
}
