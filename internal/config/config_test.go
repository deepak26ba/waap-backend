package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_JWTSecretValidation(t *testing.T) {
	origEnv := os.Getenv("ENV")
	origSecret := os.Getenv("JWT_SECRET")
	defer func() {
		_ = os.Setenv("ENV", origEnv)
		_ = os.Setenv("JWT_SECRET", origSecret)
	}()

	t.Run("Development allows fallback secret", func(t *testing.T) {
		_ = os.Setenv("ENV", "development")
		_ = os.Setenv("JWT_SECRET", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("expected success in development, got error: %v", err)
		}
		if cfg.JWTSecret == "" {
			t.Errorf("expected non-empty fallback JWT secret")
		}
	})

	t.Run("Production rejects empty JWT_SECRET", func(t *testing.T) {
		_ = os.Setenv("ENV", "production")
		_ = os.Setenv("JWT_SECRET", "")

		_, err := LoadConfig()
		if err == nil {
			t.Fatalf("expected error in production when JWT_SECRET is empty")
		}
		if !strings.Contains(err.Error(), "JWT_SECRET environment variable is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Production rejects short JWT_SECRET (< 32 chars)", func(t *testing.T) {
		_ = os.Setenv("ENV", "production")
		_ = os.Setenv("JWT_SECRET", "too-short-secret")

		_, err := LoadConfig()
		if err == nil {
			t.Fatalf("expected error in production when JWT_SECRET < 32 chars")
		}
		if !strings.Contains(err.Error(), "must be at least 32 characters long") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Production accepts valid 32+ char secret", func(t *testing.T) {
		_ = os.Setenv("ENV", "production")
		_ = os.Setenv("JWT_SECRET", "super-secure-production-jwt-secret-key-32-chars-long!")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("expected success with strong secret, got: %v", err)
		}
		if cfg.JWTSecret != "super-secure-production-jwt-secret-key-32-chars-long!" {
			t.Errorf("unexpected secret value: %s", cfg.JWTSecret)
		}
	})
}

