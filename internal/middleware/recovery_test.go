package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

func TestRecovery_CatchesPanicAndReturns500Envelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Recovery(zap.NewNop()))

	r.GET("/panic", func(c *gin.Context) {
		panic("database connection collapsed abruptly")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d. Body: %s", w.Code, w.Body.String())
	}

	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}

	if env.Success {
		t.Errorf("expected success false on panic recovery")
	}

	if env.Error == nil {
		t.Fatalf("expected error object in envelope, got nil")
	}

	if env.Error.Code != response.ErrCodeInternalServerError {
		t.Errorf("expected error code %s, got %s", response.ErrCodeInternalServerError, env.Error.Code)
	}

	if env.Error.Message == "" {
		t.Errorf("expected non-empty error message")
	}
}

func TestRecovery_NormalRequestPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Recovery(zap.NewNop()))

	r.GET("/ok", func(c *gin.Context) {
		response.Success(c, map[string]string{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}

	if !env.Success {
		t.Errorf("expected success true")
	}
}
