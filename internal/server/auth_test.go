package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/internal/auth"
	"waap-backend/internal/config"
	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

func TestAuthRouter_RegistrationValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Minimal queries stub / nil pool setup
	cfg := &config.Config{
		JWTSecret: "test-integration-secret",
	}
	router := NewRouter(cfg, nil, &db.Queries{}, zap.NewNop())

	t.Run("Malformed JSON Request Body Returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("{invalid-json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode response envelope: %v", err)
		}
		if env.Success {
			t.Errorf("expected success to be false")
		}
	})

	t.Run("Validation Failure Short Password Returns 422", func(t *testing.T) {
		payload := map[string]string{
			"email":     "valid@example.com",
			"password":  "short",
			"full_name": "Valid Name",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if env.Error == nil || env.Error.Code != response.ErrCodeValidationFailed {
			t.Errorf("expected VALIDATION_FAILED error code, got %+v", env.Error)
		}
	})
}

func TestAuthRouter_RBACProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "rbac-integration-secret"
	cfg := &config.Config{
		JWTSecret: secret,
	}
	router := NewRouter(cfg, nil, &db.Queries{}, zap.NewNop())

	t.Run("Protected Route Without Token Returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Error == nil || env.Error.Code != response.ErrCodeUnauthorized {
			t.Errorf("expected UNAUTHORIZED error code, got %+v", env.Error)
		}
	})

	t.Run("Admin Route With Customer Token Returns 403 Forbidden", func(t *testing.T) {
		customerTokens, _ := auth.GenerateTokenPair(10, "cust@test.com", "customer", secret, 15*time.Minute, 1*time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ping", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", customerTokens.AccessToken))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Error == nil || env.Error.Code != response.ErrCodeForbidden {
			t.Errorf("expected FORBIDDEN error code, got %+v", env.Error)
		}
	})

	t.Run("Admin Route With Admin Token Returns 200 OK", func(t *testing.T) {
		adminTokens, _ := auth.GenerateTokenPair(1, "admin@test.com", "admin", secret, 15*time.Minute, 1*time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ping", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminTokens.AccessToken))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if !env.Success {
			t.Errorf("expected success true")
		}
	})
}
