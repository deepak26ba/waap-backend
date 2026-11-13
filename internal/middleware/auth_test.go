package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"

	"waap-backend/internal/user"
	"waap-backend/pkg/response"
)

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-12345"

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(RequireAuth(secret))
		r.GET("/protected", func(c *gin.Context) {
			userID, _ := GetUserID(c)
			email, _ := GetUserEmail(c)
			role, _ := GetUserRole(c)
			response.OK(c, gin.H{
				"user_id": userID.String(),
				"email":   email,
				"role":    role,
			})
		})
		return r
	}

	t.Run("Missing Authorization Header Returns 401", func(t *testing.T) {
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Code)
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Success {
			t.Errorf("expected success false")
		}
	})

	t.Run("Malformed Authorization Header Returns 401", func(t *testing.T) {
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Token abcdef123456")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("Invalid Token Signature Returns 401", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "wrong@test.com", "customer", "another-secret", 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("Refresh Token Rejected for Resource Auth Returns 401", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "user@test.com", "customer", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.RefreshToken))
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 on refresh token usage, got %d", w.Code)
		}
	})

	t.Run("Valid Access Token Populates Context and Returns 200", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "alice@test.com", "customer", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		dataMap := env.Data.(map[string]interface{})
		if dataMap["user_id"] != testID.String() {
			t.Errorf("expected user_id %s, got %v", testID.String(), dataMap["user_id"])
		}
		if dataMap["email"] != "alice@test.com" {
			t.Errorf("expected email alice@test.com, got %v", dataMap["email"])
		}
	})
}

func TestRequireRolesAndAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "role-test-secret"

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(RequireAuth(secret))

		adminGroup := r.Group("/admin")
		adminGroup.Use(RequireAdmin())
		adminGroup.GET("/metrics", func(c *gin.Context) {
			response.OK(c, gin.H{"admin": true})
		})

		managerGroup := r.Group("/manager")
		managerGroup.Use(RequireRoles("manager", "admin"))
		managerGroup.GET("/reports", func(c *gin.Context) {
			response.OK(c, gin.H{"reports": true})
		})

		return r
	}

	t.Run("Customer Cannot Access Admin Route (403 Forbidden)", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "customer@test.com", "customer", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403 Forbidden, got %d. Body: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Error == nil || env.Error.Code != response.ErrCodeForbidden {
			t.Errorf("expected code FORBIDDEN, got %+v", env.Error)
		}
	})

	t.Run("Admin Can Access Admin Route (200 OK)", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "admin@test.com", "admin", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Manager Can Access Manager Route (200 OK)", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "manager@test.com", "manager", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/manager/reports", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}

func TestOptionalAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-12345"

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(OptionalAuth(secret))
		r.GET("/cart", func(c *gin.Context) {
			userID, exists := GetUserID(c)
			if exists {
				response.OK(c, gin.H{"user_id": userID.String()})
			} else {
				response.OK(c, gin.H{"user_id": nil})
			}
		})
		return r
	}

	t.Run("No token proceeds as guest with 200 OK", func(t *testing.T) {
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Valid token sets user_id and returns 200 OK", func(t *testing.T) {
		testID := uuid.Must(uuid.NewV4())
		tokens, _ := user.GenerateTokenPair(testID, "user@test.com", "customer", secret, 15*time.Minute, 1*time.Hour)
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Invalid token returns 401 Unauthorized", func(t *testing.T) {
		r := setupRouter()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Code)
		}
	})
}
