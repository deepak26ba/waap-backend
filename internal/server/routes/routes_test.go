package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
)

func TestRegister_Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Swagger and Health Observability routes are registered", func(t *testing.T) {
		r := gin.New()
		deps := Dependencies{
			Config:  &config.Config{JWTSecret: "test-jwt-secret"},
			Pool:    nil,
			Queries: &db.Queries{},
			Log:     zap.NewNop(),
		}
		Register(r, deps)

		// Test Swagger doc route
		reqSwagger, _ := http.NewRequest(http.MethodGet, "/swagger/index.html", nil)
		reqSwagger.RequestURI = "/swagger/index.html"
		wSwagger := httptest.NewRecorder()
		r.ServeHTTP(wSwagger, reqSwagger)
		if wSwagger.Code != http.StatusOK {
			t.Errorf("expected 200 OK for /swagger/index.html, got %d", wSwagger.Code)
		}

		// Test Swagger convenience redirect from /api/v1/swagger/index.html
		reqSwaggerRedirect, _ := http.NewRequest(http.MethodGet, "/api/v1/swagger/index.html", nil)
		wSwaggerRedirect := httptest.NewRecorder()
		r.ServeHTTP(wSwaggerRedirect, reqSwaggerRedirect)
		if wSwaggerRedirect.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301 Moved Permanently for /api/v1/swagger/index.html, got %d", wSwaggerRedirect.Code)
		}

		// Test Healthz route (nil pool returns 503)
		reqHealth, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
		wHealth := httptest.NewRecorder()
		r.ServeHTTP(wHealth, reqHealth)
		if wHealth.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 Service Unavailable for nil pool, got %d", wHealth.Code)
		}
	})

	t.Run("Nil queries does not mount v1 routes", func(t *testing.T) {
		r := gin.New()
		deps := Dependencies{
			Config:  &config.Config{JWTSecret: "test-jwt-secret"},
			Pool:    nil,
			Queries: nil,
			Log:     zap.NewNop(),
		}
		Register(r, deps)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/catalog/categories", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unmounted routes, got %d", w.Code)
		}
	})

	t.Run("Production panic on empty JWT secret", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when JWT_SECRET is missing in production")
			}
		}()

		r := gin.New()
		deps := Dependencies{
			Config: &config.Config{
				Env:       "production",
				JWTSecret: "",
			},
			Queries: &db.Queries{},
			Log:     zap.NewNop(),
		}
		Register(r, deps)
	})
}
