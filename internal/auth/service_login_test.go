package auth

import (
	"testing"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginReturnsAccessAndRefreshToken(t *testing.T) {
	config.App.JWTSecret = "test-secret"

	repo := newMockAuthRepository()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &User{
		BaseModel:    BaseModel{ID: uuid.New()},
		Email:        "admin@wms.com",
		PasswordHash: string(passwordHash),
	}
	repo.usersByEmail[user.Email] = user
	repo.usersByID[user.ID] = user

	svc := NewService(repo)

	res, err := svc.Login(LoginRequest{
		Email:    "admin@wms.com",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("expected login success, got error: %v", err)
	}

	if res.AccessToken == "" {
		t.Fatal("expected access token to be set")
	}
	if res.RefreshToken == "" {
		t.Fatal("expected refresh token to be set")
	}

	if len(repo.tokensByHash) != 1 {
		t.Fatalf("expected 1 refresh token persisted, got %d", len(repo.tokensByHash))
	}
}
