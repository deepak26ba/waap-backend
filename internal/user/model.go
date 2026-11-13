package user

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

// UserResponse represents safe, serialized user profile data.
type UserResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	PhoneNumber *string   `json:"phone_number,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RegisterInput models the registration request payload.
type RegisterInput struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

// LoginInput models the login request credentials.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains authentication tokens and the authenticated user's profile.
type LoginResponse struct {
	TokenPair
	User UserResponse `json:"user"`
}

// RefreshInput models the refresh token request payload.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

