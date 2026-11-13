package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

func TestServerRouter_HealthCheckSuccess(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Skipf("skipping live database test: %v", err)
	}
	defer pool.Close()

	srv := New(cfg, pool, db.New(pool), zap.NewNop())
	router := srv.Router()

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%v'", resp["status"])
	}

	if resp["database"] != "connected" {
		t.Errorf("expected database 'connected', got '%v'", resp["database"])
	}

	// Verify OWASP security headers are present
	if xfo := w.Header().Get("X-Frame-Options"); xfo != "DENY" {
		t.Errorf("expected X-Frame-Options 'DENY', got '%v'", xfo)
	}

	// Verify CORS header
	if acao := w.Header().Get("Access-Control-Allow-Origin"); acao != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got '%v'", acao)
	}
}

func TestServerRouter_HealthCheckNilPool(t *testing.T) {
	router := NewRouter(nil, nil, nil, zap.NewNop())

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if resp["status"] != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got '%v'", resp["status"])
	}
}

func TestServerRouter_PanicRecovery(t *testing.T) {
	router := NewRouter(nil, nil, nil, zap.NewNop())
	router.GET("/crash", func(c *gin.Context) {
		panic("unexpected runtime panic")
	})

	req, _ := http.NewRequest(http.MethodGet, "/crash", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}

	if env.Success {
		t.Errorf("expected success to be false")
	}
	if env.Error == nil || env.Error.Code != response.ErrCodeInternalServerError {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %+v", env.Error)
	}
}

func TestServerRouter_NotFoundEnvelope(t *testing.T) {
	router := NewRouter(nil, nil, nil, zap.NewNop())

	req, _ := http.NewRequest(http.MethodGet, "/non-existent-path", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}

	if env.Success {
		t.Errorf("expected success false")
	}
	if env.Error == nil || env.Error.Code != response.ErrCodeNotFound {
		t.Errorf("expected NOT_FOUND, got %+v", env.Error)
	}
}

func TestServerRouter_MethodNotAllowedEnvelope(t *testing.T) {
	router := NewRouter(nil, nil, nil, zap.NewNop())

	req, _ := http.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", w.Code)
	}

	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}

	if env.Success {
		t.Errorf("expected success false")
	}
	if env.Error == nil || env.Error.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected METHOD_NOT_ALLOWED, got %+v", env.Error)
	}
}
