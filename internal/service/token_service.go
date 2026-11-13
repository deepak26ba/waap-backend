package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"waap-backend/internal/model"
)

const (
	// BcryptCost12 is the required hashing cost factor specified in TICKET-06.
	BcryptCost12 = 12

	DefaultAccessTTL  = 15 * time.Minute
	DefaultRefreshTTL = 7 * 24 * time.Hour
)

// Precomputed dummy bcrypt hash (cost 12) for constant-time password comparisons
// when user is not found, effectively eliminating timing-based user enumeration attacks.
const dummyBcryptHash = "$2a$12$e8Yk2uKzLqO2Vl5h4v1lQO8sC6P0f9F2a6D1H4J7K9L2M5N7O0P3Q"

// HashPassword hashes a plaintext password using bcrypt with cost 12.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies that a plaintext password matches the stored bcrypt hash.
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// CheckDummyPassword executes a bcrypt comparison against a dummy hash to maintain
// constant execution time when the target account does not exist.
func CheckDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password))
}

// GenerateTokenPair issues a short-lived access token and a longer-lived refresh token.
func GenerateTokenPair(userID int64, email, role, secret string, accessTTL, refreshTTL time.Duration) (*model.TokenPair, error) {
	if secret == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	now := time.Now()
	accessExpiry := now.Add(accessTTL)
	refreshExpiry := now.Add(refreshTTL)

	// 1. Build Access Token Claims
	accessClaims := &model.Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: model.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Build Refresh Token Claims
	refreshClaims := &model.Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: model.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &model.TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTTL.Seconds()),
		ExpiresAt:    accessExpiry,
	}, nil
}

// ValidateToken parses, validates signature and expiration of a JWT token string.
func ValidateToken(tokenString, secret string) (*model.Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &model.Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*model.Claims)
	if !ok || !token.Valid {
		return nil, model.ErrInvalidToken
	}

	return claims, nil
}
