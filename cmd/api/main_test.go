package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"waap-backend/internal/config"
	"waap-backend/internal/server"
	"waap-backend/pkg/db"
)

func TestHealthCheckProbe_Success(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Skipf("skipping live database test (cannot connect): %v", err)
	}
	defer pool.Close()

	srv := server.New(cfg, pool, db.New(pool), zap.NewNop())
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
	if xcto := w.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got '%v'", xcto)
	}
	if hsts := w.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Errorf("expected Strict-Transport-Security header to be set")
	}

	// Verify CORS header
	if acao := w.Header().Get("Access-Control-Allow-Origin"); acao != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got '%v'", acao)
	}
}

func TestHealthCheckProbe_NilPool(t *testing.T) {
	router := server.NewRouter(nil, nil, nil, zap.NewNop())

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

	// Even on 503 error, security headers must be present
	if xfo := w.Header().Get("X-Frame-Options"); xfo != "DENY" {
		t.Errorf("expected X-Frame-Options 'DENY' on error response, got '%v'", xfo)
	}
}
