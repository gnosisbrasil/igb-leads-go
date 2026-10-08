// Package service holds business logic and external integrations.
package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AccessClaims mirrors the jsonwebtoken payload: {id, email, role}.
type AccessClaims struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims mirrors {id, type:'refresh'}.
type RefreshClaims struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	jwt.RegisteredClaims
}

var (
	// ErrTokenExpired maps to the Node TokenExpiredError branch.
	ErrTokenExpired = errors.New("expired")
	// ErrTokenInvalid maps to the Node JsonWebTokenError branch.
	ErrTokenInvalid = errors.New("invalid")
)

// TokenService signs and verifies access and refresh tokens.
type TokenService struct {
	secret        []byte
	expiresIn     time.Duration
	refreshSecret []byte
	refreshIn     time.Duration
	now           func() time.Time
}

func NewTokenService(secret string, expiresIn time.Duration, refreshSecret string, refreshIn time.Duration) *TokenService {
	return &TokenService{
		secret:        []byte(secret),
		expiresIn:     expiresIn,
		refreshSecret: []byte(refreshSecret),
		refreshIn:     refreshIn,
		now:           time.Now,
	}
}

// SignAccess issues an access token for the user.
func (s *TokenService) SignAccess(id, email, role string) (string, error) {
	now := s.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, AccessClaims{
		ID: id, Email: email, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiresIn)),
		},
	})
	return tok.SignedString(s.secret)
}

// VerifyAccess parses an access token.
func (s *TokenService) VerifyAccess(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("método inesperado: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, mapJWTError(err)
	}
	return claims, nil
}

// SignRefresh issues a refresh token for the user id.
func (s *TokenService) SignRefresh(id string) (string, error) {
	now := s.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, RefreshClaims{
		ID: id, Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshIn)),
		},
	})
	return tok.SignedString(s.refreshSecret)
}

// VerifyRefresh parses a refresh token.
func (s *TokenService) VerifyRefresh(raw string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("método inesperado: %v", t.Header["alg"])
		}
		return s.refreshSecret, nil
	})
	if err != nil {
		return nil, mapJWTError(err)
	}
	return claims, nil
}

func mapJWTError(err error) error {
	if errors.Is(err, jwt.ErrTokenExpired) {
		return ErrTokenExpired
	}
	return ErrTokenInvalid
}

// HashPassword hashes with bcrypt cost 10, like the Node backend.
// Hashes are $2a$-compatible both ways with bcryptjs.
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a stored hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
