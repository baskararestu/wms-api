package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/redis"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	gredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// Service defines the interface for business logic in the Auth domain
type Service interface {
	Register(req LoginRequest) error
	Login(req LoginRequest) (*LoginResponse, error)
	RefreshToken(req RefreshTokenRequest) (*LoginResponse, error)
	Logout(req LogoutRequest) error
	GenerateAccessToken(user *User) (string, error)
}

type service struct {
	repo Repository
}

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// NewService creates a new Auth Service instance
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Register creates a new user, hashing their password
func (s *service) Register(req LoginRequest) error {
	// Check if user already exists
	_, err := s.repo.FindUserByEmail(req.Email)
	if err == nil {
		return errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	return s.repo.CreateUser(user)
}

// Login verifies credentials and returns a JWT
func (s *service) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshHash, err := generateRefreshTokenPair()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(refreshTokenTTL)
	if err := s.repo.CreateRefreshToken(&RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		return nil, errors.New("failed to create refresh session")
	}

	s.cacheRefreshToken(refreshHash, user.ID.String(), time.Until(expiresAt))

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *service) RefreshToken(req RefreshTokenRequest) (*LoginResponse, error) {
	refreshHash := hashRefreshToken(req.RefreshToken)

	cachedUserID, cacheErr := s.getCachedRefreshUserID(refreshHash)
	hasCache := cacheErr == nil

	tokenRow, err := s.repo.FindRefreshTokenByHash(refreshHash)
	if err != nil {
		if hasCache {
			s.invalidateCachedRefreshToken(refreshHash)
		}
		return nil, errors.New("invalid or expired refresh token")
	}

	if tokenRow.RevokedAt != nil || time.Now().After(tokenRow.ExpiresAt) {
		s.invalidateCachedRefreshToken(refreshHash)
		return nil, errors.New("invalid or expired refresh token")
	}

	var userID uuid.UUID
	if hasCache {
		parsedUserID, parseErr := uuid.Parse(cachedUserID)
		if parseErr == nil && parsedUserID == tokenRow.UserID {
			userID = parsedUserID
		} else {
			userID = tokenRow.UserID
		}
	} else {
		userID = tokenRow.UserID
	}

	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if err := s.repo.RevokeRefreshTokenByHash(refreshHash); err != nil {
		return nil, errors.New("failed to rotate refresh token")
	}
	s.invalidateCachedRefreshToken(refreshHash)

	newRefreshToken, newRefreshHash, err := generateRefreshTokenPair()
	if err != nil {
		return nil, err
	}

	newExpiresAt := time.Now().Add(refreshTokenTTL)
	if err := s.repo.CreateRefreshToken(&RefreshToken{
		UserID:    user.ID,
		TokenHash: newRefreshHash,
		ExpiresAt: newExpiresAt,
	}); err != nil {
		return nil, errors.New("failed to create new refresh session")
	}

	s.cacheRefreshToken(newRefreshHash, user.ID.String(), time.Until(newExpiresAt))

	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *service) Logout(req LogoutRequest) error {
	refreshHash := hashRefreshToken(req.RefreshToken)

	tokenRow, err := s.repo.FindRefreshTokenByHash(refreshHash)
	if err != nil {
		return errors.New("invalid or expired refresh token")
	}

	if tokenRow.RevokedAt != nil || time.Now().After(tokenRow.ExpiresAt) {
		s.invalidateCachedRefreshToken(refreshHash)
		return errors.New("invalid or expired refresh token")
	}

	if err := s.repo.RevokeRefreshTokenByHash(refreshHash); err != nil {
		return errors.New("failed to revoke refresh token")
	}

	s.invalidateCachedRefreshToken(refreshHash)
	return nil
}

// GenerateAccessToken creates an access JWT for a given user
func (s *service) GenerateAccessToken(user *User) (string, error) {
	secret := config.App.JWTSecret
	
	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"type":    "access",
		"exp":     time.Now().Add(accessTokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func generateRefreshTokenPair() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", errors.New("failed to generate refresh token")
	}

	rawToken = base64.RawURLEncoding.EncodeToString(b)
	tokenHash = hashRefreshToken(rawToken)

	return rawToken, tokenHash, nil
}

func hashRefreshToken(token string) string {
	hashed := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hashed)
}

func (s *service) refreshCacheKey(tokenHash string) string {
	return "auth:refresh:" + tokenHash
}

func (s *service) cacheRefreshToken(tokenHash, userID string, ttl time.Duration) {
	if redis.Client == nil || ttl <= 0 {
		return
	}

	_ = redis.Client.Set(redis.Ctx, s.refreshCacheKey(tokenHash), userID, ttl).Err()
}

func (s *service) getCachedRefreshUserID(tokenHash string) (string, error) {
	if redis.Client == nil {
		return "", gredis.Nil
	}

	return redis.Client.Get(redis.Ctx, s.refreshCacheKey(tokenHash)).Result()
}

func (s *service) invalidateCachedRefreshToken(tokenHash string) {
	if redis.Client == nil {
		return
	}

	_ = redis.Client.Del(redis.Ctx, s.refreshCacheKey(tokenHash)).Err()
}
