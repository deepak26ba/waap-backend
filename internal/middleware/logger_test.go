package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var logBuf bytes.Buffer
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(&logBuf),
		zap.DebugLevel,
	)
	log := zap.New(core)

	r := gin.New()
	r.Use(ZapLogger(log))

	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/warn", func(c *gin.Context) {
		c.Status(http.StatusBadRequest)
	})
	r.GET("/error", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	t.Run("Logs 200 OK at Info level", func(t *testing.T) {
		logBuf.Reset()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if !bytes.Contains(logBuf.Bytes(), []byte("HTTP Request")) {
			t.Errorf("expected log output to contain 'HTTP Request', got: %s", logBuf.String())
		}
	})

	t.Run("Logs 400 Bad Request at Warn level", func(t *testing.T) {
		logBuf.Reset()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/warn", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
		if !bytes.Contains(logBuf.Bytes(), []byte("HTTP Client Warning")) {
			t.Errorf("expected log output to contain 'HTTP Client Warning', got: %s", logBuf.String())
		}
	})

	t.Run("Logs 500 Server Error at Error level", func(t *testing.T) {
		logBuf.Reset()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/error", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
		if !bytes.Contains(logBuf.Bytes(), []byte("HTTP Server Error")) {
			t.Errorf("expected log output to contain 'HTTP Server Error', got: %s", logBuf.String())
		}
	})
}
