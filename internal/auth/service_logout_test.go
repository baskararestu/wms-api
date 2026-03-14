package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLogoutRevokesRefreshToken(t *testing.T) {
	repo := newMockAuthRepository()
	userID := uuid.New()

	rawRefreshToken := "logout-refresh-token"
	hashedRefreshToken := hashForTest(rawRefreshToken)
	row := &RefreshToken{
		BaseModel: BaseModel{ID: uuid.New()},
		UserID:    userID,
		TokenHash: hashedRefreshToken,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	repo.tokensByHash[hashedRefreshToken] = row

	svc := NewService(repo)
	err := svc.Logout(LogoutRequest{RefreshToken: rawRefreshToken})
	if err != nil {
		t.Fatalf("expected logout success, got error: %v", err)
	}

	if row.RevokedAt == nil {
		t.Fatal("expected refresh token to be revoked on logout")
	}
}

func TestLogoutFailsForInvalidRefreshToken(t *testing.T) {
	repo := newMockAuthRepository()
	svc := NewService(repo)

	err := svc.Logout(LogoutRequest{RefreshToken: "invalid-token"})
	if err == nil {
		t.Fatal("expected invalid token error on logout")
	}
}
