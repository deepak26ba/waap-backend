package auth

import (
	"testing"
	"time"
)

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "SecretPassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == password {
		t.Errorf("hash should not equal plaintext password")
	}

	if err := CheckPassword(hash, password); err != nil {
		t.Errorf("expected password match, got error: %v", err)
	}

	if err := CheckPassword(hash, "WrongPassword"); err == nil {
		t.Errorf("expected error on mismatched password, got nil")
	}
}

func TestCheckDummyPassword(t *testing.T) {
	// Must run without panic or issue
	CheckDummyPassword("arbitrary_password")
}

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "super-secure-jwt-test-secret-key-32b"
	userID := int64(1001)
	email := "user@example.com"
	role := "customer"
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour

	tokens, err := GenerateTokenPair(userID, email, role, secret, accessTTL, refreshTTL)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected non-empty access and refresh tokens")
	}

	// Validate Access Token
	claims, err := ValidateToken(tokens.AccessToken, secret)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %d, got %d", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("expected role %s, got %s", role, claims.Role)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("expected token_type %s, got %s", TokenTypeAccess, claims.TokenType)
	}

	// Validate Refresh Token
	refreshClaims, err := ValidateToken(tokens.RefreshToken, secret)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if refreshClaims.TokenType != TokenTypeRefresh {
		t.Errorf("expected token_type %s, got %s", TokenTypeRefresh, refreshClaims.TokenType)
	}

	// Validate with wrong secret fails
	_, err = ValidateToken(tokens.AccessToken, "wrong-secret-key")
	if err == nil {
		t.Errorf("expected error when validating with wrong secret")
	}

	// Validate malformed token fails
	_, err = ValidateToken("not.a.valid.jwt", secret)
	if err == nil {
		t.Errorf("expected error when validating malformed token")
	}
}

func TestValidateExpiredToken(t *testing.T) {
	secret := "secret-key"
	// 1 millisecond expiration
	tokens, err := GenerateTokenPair(1, "test@test.com", "customer", secret, -1*time.Minute, -1*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	_, err = ValidateToken(tokens.AccessToken, secret)
	if err == nil {
		t.Errorf("expected error for expired token, got nil")
	}
}
