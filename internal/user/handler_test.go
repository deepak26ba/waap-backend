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
	"github.com/gofrs/uuid/v5"

	"waap-backend/pkg/response"
)

type mockAuthService struct {
	registerFunc           func(ctx context.Context, in RegisterInput) (*RegisterResponse, error)
	loginFunc              func(ctx context.Context, in LoginInput) (*LoginResponse, error)
	refreshTokenFunc       func(ctx context.Context, refreshToken string) (*TokenPair, error)
	getProfileFunc         func(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
	verifyEmailFunc        func(ctx context.Context, email, otp string) error
	resendVerificationFunc func(ctx context.Context, email string) error
	forgotPasswordFunc     func(ctx context.Context, email string) error
	resetPasswordFunc      func(ctx context.Context, token, newPassword string) error
}

func (m *mockAuthService) Register(ctx context.Context, in RegisterInput) (*RegisterResponse, error) {
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

func (m *mockAuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	if m.getProfileFunc != nil {
		return m.getProfileFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockAuthService) VerifyEmail(ctx context.Context, email, otp string) error {
	if m.verifyEmailFunc != nil {
		return m.verifyEmailFunc(ctx, email, otp)
	}
	return nil
}

func (m *mockAuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	if m.resendVerificationFunc != nil {
		return m.resendVerificationFunc(ctx, email)
	}
	return nil
}

func (m *mockAuthService) ForgotPassword(ctx context.Context, email string) error {
	if m.forgotPasswordFunc != nil {
		return m.forgotPasswordFunc(ctx, email)
	}
	return nil
}

func (m *mockAuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if m.resetPasswordFunc != nil {
		return m.resetPasswordFunc(ctx, token, newPassword)
	}
	return nil
}

func (m *mockAuthService) SetPasswordResetStore(store PasswordResetStore) {}
func (m *mockAuthService) SetPasswordResetTTL(ttl time.Duration)          {}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/auth")
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
	rg.POST("/verify-email", h.VerifyEmail)
	rg.POST("/resend-verification", h.ResendVerification)
	return r
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("Valid registration returns 201 Created", func(t *testing.T) {
		svc := &mockAuthService{
			registerFunc: func(ctx context.Context, in RegisterInput) (*RegisterResponse, error) {
				return &RegisterResponse{
					Message: "Registration successful. Please enter the OTP sent to your email to activate your account.",
					User: UserResponse{
						ID:        uuid.Must(uuid.NewV4()),
						Email:     in.Email,
						FullName:  in.FullName,
						Role:      "customer",
						CreatedAt: time.Now(),
					},
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

		body := []byte(`{"email":`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Service conflict returns 409 Conflict", func(t *testing.T) {
		svc := &mockAuthService{
			registerFunc: func(ctx context.Context, in RegisterInput) (*RegisterResponse, error) {
				return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "user exists")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"existing@example.com","password":"password123","full_name":"Existing"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", w.Code)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("Valid credentials return 200 OK with tokens", func(t *testing.T) {
		svc := &mockAuthService{
			loginFunc: func(ctx context.Context, in LoginInput) (*LoginResponse, error) {
				return &LoginResponse{
					TokenPair: TokenPair{
						AccessToken:  "valid-access-token",
						RefreshToken: "valid-refresh-token",
						TokenType:    "Bearer",
					},
					User: UserResponse{
						ID:       uuid.Must(uuid.NewV4()),
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
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Invalid credentials return 401 Unauthorized", func(t *testing.T) {
		svc := &mockAuthService{
			loginFunc: func(ctx context.Context, in LoginInput) (*LoginResponse, error) {
				return nil, response.NewUnauthorizedError("invalid credentials")
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

	t.Run("Malformed JSON returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestAuthHandler_RefreshToken(t *testing.T) {
	t.Run("Valid refresh token returns 200 OK with new tokens", func(t *testing.T) {
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

func TestAuthHandler_VerifyEmail(t *testing.T) {
	t.Run("Valid OTP and email returns 200 OK", func(t *testing.T) {
		svc := &mockAuthService{
			verifyEmailFunc: func(ctx context.Context, email, otp string) error {
				if email != "test@example.com" || otp != "123456" {
					return response.NewBadRequestError("INVALID_OTP", "invalid verification otp")
				}
				return nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","otp":"123456"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode response envelope: %v", err)
		}
		if !env.Success {
			t.Errorf("expected success to be true")
		}
	})

	t.Run("Missing or empty otp returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Missing or empty email returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		body := []byte(`{"otp":"123456"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Invalid otp returns 400 Bad Request with error envelope", func(t *testing.T) {
		svc := &mockAuthService{
			verifyEmailFunc: func(ctx context.Context, email, otp string) error {
				return response.NewBadRequestError("INVALID_OTP", "invalid verification otp")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","otp":"000000"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Mismatched email returns 400 Bad Request", func(t *testing.T) {
		svc := &mockAuthService{
			verifyEmailFunc: func(ctx context.Context, email, otp string) error {
				return response.NewBadRequestError("INVALID_OTP", "verification otp does not match the provided email address")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"other@example.com","otp":"123456"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Expired otp returns 400 Bad Request", func(t *testing.T) {
		svc := &mockAuthService{
			verifyEmailFunc: func(ctx context.Context, email, otp string) error {
				return response.NewBadRequestError("OTP_EXPIRED", "verification otp has expired")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","otp":"123456"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Already used otp returns 400 Bad Request", func(t *testing.T) {
		svc := &mockAuthService{
			verifyEmailFunc: func(ctx context.Context, email, otp string) error {
				return response.NewBadRequestError("OTP_ALREADY_USED", "verification otp has already been used")
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"test@example.com","otp":"123456"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestAuthHandler_ResendVerification(t *testing.T) {
	t.Run("Valid request returns 200 OK", func(t *testing.T) {
		svc := &mockAuthService{
			resendVerificationFunc: func(ctx context.Context, email string) error {
				if email != "user@example.com" {
					return response.NewValidationError("validation failed", []response.FieldError{{Field: "email", Message: "invalid"}})
				}
				return nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"user@example.com"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/resend-verification", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode response envelope: %v", err)
		}
		if !env.Success {
			t.Errorf("expected success to be true")
		}
	})

	t.Run("Missing or malformed JSON returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := setupTestRouter(h)

		body := []byte(`{}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/resend-verification", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Already verified account returns generic 200 OK (anti-enumeration)", func(t *testing.T) {
		svc := &mockAuthService{
			resendVerificationFunc: func(ctx context.Context, email string) error {
				// Anti-enumeration: returns nil for verified account
				return nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		body := []byte(`{"email":"verified@example.com"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/resend-verification", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})
}

func TestAuthHandler_GetProfile(t *testing.T) {
	testUserID := uuid.Must(uuid.NewV4())

	t.Run("Authenticated user receives profile", func(t *testing.T) {
		svc := &mockAuthService{
			getProfileFunc: func(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
				return &UserResponse{
					ID:       userID,
					Email:    "profile@example.com",
					FullName: "Profile User",
				}, nil
			},
		}
		h := NewHandler(svc, nil)
		r := gin.New()
		r.GET("/me", func(c *gin.Context) {
			c.Set(ContextUserID, testUserID)
			h.GetProfile(c)
		})

		req, _ := http.NewRequest(http.MethodGet, "/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Missing context user_id returns 401", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := gin.New()
		r.GET("/me", h.GetProfile)

		req, _ := http.NewRequest(http.MethodGet, "/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}

func TestAuthHandler_ForgotPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid request returns 200 OK", func(t *testing.T) {
		called := false
		svc := &mockAuthService{
			forgotPasswordFunc: func(ctx context.Context, email string) error {
				called = true
				if email != "user@example.com" {
					t.Errorf("expected email user@example.com, got %s", email)
				}
				return nil
			},
		}
		h := NewHandler(svc, nil)
		r := gin.New()
		r.POST("/auth/forgot-password", h.ForgotPassword)

		body := []byte(`{"email":"user@example.com"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		if !called {
			t.Error("expected ForgotPassword service method to be invoked")
		}
	})

	t.Run("Missing or malformed JSON returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := gin.New()
		r.POST("/auth/forgot-password", h.ForgotPassword)

		req, _ := http.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString("not-json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestAuthHandler_ResetPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid request returns 200 OK", func(t *testing.T) {
		called := false
		svc := &mockAuthService{
			resetPasswordFunc: func(ctx context.Context, token, newPassword string) error {
				called = true
				if token != "valid-token-123" {
					t.Errorf("expected token valid-token-123, got %s", token)
				}
				if newPassword != "NewPassword123!" {
					t.Errorf("expected new password NewPassword123!, got %s", newPassword)
				}
				return nil
			},
		}
		h := NewHandler(svc, nil)
		r := gin.New()
		r.POST("/auth/reset-password", h.ResetPassword)

		body := []byte(`{"token":"valid-token-123","new_password":"NewPassword123!"}`)
		req, _ := http.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		if !called {
			t.Error("expected ResetPassword service method to be invoked")
		}
	})

	t.Run("Missing fields returns 400 Bad Request", func(t *testing.T) {
		h := NewHandler(&mockAuthService{}, nil)
		r := gin.New()
		r.POST("/auth/reset-password", h.ResetPassword)

		body := []byte(`{"token":"valid-token"}`) // missing new_password
		req, _ := http.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}
