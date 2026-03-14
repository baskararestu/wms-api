package auth

import (
	"errors"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service defines the interface for business logic in the Auth domain
type Service interface {
	Register(req LoginRequest) error
	Login(req LoginRequest) (*LoginResponse, error)
	GenerateToken(user *User) (string, error)
}

type service struct {
	repo Repository
}

// NewService creates a new Auth Service instance
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Register creates a new user, hashing their password
func (s *service) Register(req LoginRequest) error {
	// Check if user already exists
	_, err := s.repo.FindUserByUsername(req.Username)
	if err == nil {
		return errors.New("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
	}

	return s.repo.CreateUser(user)
}

// Login verifies credentials and returns a JWT
func (s *service) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindUserByUsername(req.Username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{Token: token}, nil
}

// GenerateToken creates a JWT for a given user
func (s *service) GenerateToken(user *User) (string, error) {
	secret := config.App.JWTSecret
	
	claims := jwt.MapClaims{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 hours
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
