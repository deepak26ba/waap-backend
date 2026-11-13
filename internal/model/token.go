package model

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// ErrInvalidToken indicates the token is malformed, expired, or failed signature check.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims represents the JWT claims payload containing essential identity and authorization data.
type Claims struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// TokenPair contains the signed access and refresh tokens returned upon successful authentication.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RefreshInput models the refresh token request payload.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
