package auth

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type mockAuthRepository struct {
	usersByEmail map[string]*User
	usersByID    map[uuid.UUID]*User
	tokensByHash map[string]*RefreshToken
}

func newMockAuthRepository() *mockAuthRepository {
	return &mockAuthRepository{
		usersByEmail: make(map[string]*User),
		usersByID:    make(map[uuid.UUID]*User),
		tokensByHash: make(map[string]*RefreshToken),
	}
}

func (m *mockAuthRepository) FindUserByEmail(email string) (*User, error) {
	user, ok := m.usersByEmail[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (m *mockAuthRepository) FindUserByID(userID uuid.UUID) (*User, error) {
	user, ok := m.usersByID[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (m *mockAuthRepository) CreateUser(user *User) error {
	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockAuthRepository) CreateRefreshToken(token *RefreshToken) error {
	m.tokensByHash[token.TokenHash] = token
	return nil
}

func (m *mockAuthRepository) FindRefreshTokenByHash(tokenHash string) (*RefreshToken, error) {
	token, ok := m.tokensByHash[tokenHash]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return token, nil
}

func (m *mockAuthRepository) RevokeRefreshTokenByHash(tokenHash string) error {
	token, ok := m.tokensByHash[tokenHash]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	now := time.Now()
	token.RevokedAt = &now
	return nil
}

func (m *mockAuthRepository) UpsertMarketplaceCredential(_ *MarketplaceCredential, _ int) error {
	return nil
}

func hashForTest(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}
