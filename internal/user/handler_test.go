package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"waap-backend/pkg/response"
)

type mockAuthService struct {
	registerFunc     func(ctx context.Context, in RegisterInput) (*UserResponse, error)
	loginFunc        func(ctx context.Context, in LoginInput) (*LoginResponse, error)
	refreshTokenFunc func(ctx context.Context, refreshToken string) (*TokenPair, error)
}

func (m *mockAuthService) Register(ctx context.Context, in RegisterInput) (*UserResponse, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, in)
	}
	return nil, nil
}

func (m *mockAuthService) Login(ctx context.Context, in LoginInput) (*LoginResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, in)
	}
	return nil, nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, refreshToken)
	}
	return nil, nil
}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/auth")
	h.RegisterRoutes(rg)
	return r
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("Valid registration returns 201 Created", func(t *testing.T) {
		svc := &mockAuthService{
			registerFunc: func(ctx context.Context, in RegisterInput) (*UserResponse, error) {
				return &UserResponse{
					ID:        1,
					Email:     in.Email,
					FullName:  in.FullName,
					Role:      "customer",
					CreatedAt: time.Now(),
				}, nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","password":"password123","full_name":"Test User"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode response envelope: %v", err)
		}
		if !env.Success {
			t.Errorf("expected success to be true")
		}
	})

	t.Run("Malformed JSON returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{invalid-json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Service validation error returns 422", func(t *testing.T) {
		svc := &mockAuthService{
			registerFunc: func(ctx context.Context, in RegisterInput) (*UserResponse, error) {
				return nil, response.NewValidationError("validation failed", []response.FieldError{
					{Field: "email", Message: "invalid email format"},
				})
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"bad","password":"password123","full_name":"Test"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 Unprocessable Entity, got %d", w.Code)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("Valid credentials returns 200 OK", func(t *testing.T) {
		svc := &mockAuthService{
			loginFunc: func(ctx context.Context, in LoginInput) (*LoginResponse, error) {
				return &LoginResponse{
					TokenPair: TokenPair{
						AccessToken:  "mock-access-token",
						RefreshToken: "mock-refresh-token",
						TokenType:    "Bearer",
					},
					User: UserResponse{
						ID:       1,
						Email:    in.Email,
						FullName: "Test User",
						Role:     "customer",
					},
				}, nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","password":"password123"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Invalid credentials returns 401 Unauthorized", func(t *testing.T) {
		svc := &mockAuthService{
			loginFunc: func(ctx context.Context, in LoginInput) (*LoginResponse, error) {
				return nil, response.NewUnauthorizedError("invalid email or password")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","password":"wrongpassword"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	t.Run("Valid refresh token returns 200 OK", func(t *testing.T) {
		svc := &mockAuthService{
			refreshTokenFunc: func(ctx context.Context, refreshToken string) (*TokenPair, error) {
				return &TokenPair{
					AccessToken:  "new-access-token",
					RefreshToken: "new-refresh-token",
					TokenType:    "Bearer",
				}, nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"refresh_token":"valid-refresh-token"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Missing refresh token returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		body := []byte(`{}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

