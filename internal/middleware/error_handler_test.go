package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

func TestErrorHandler_Interception(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("AppError intercepted", func(t *testing.T) {
		r := gin.New()
		r.Use(ErrorHandler(zap.NewNop()))

		r.GET("/not-found", func(c *gin.Context) {
			_ = c.Error(response.NewNotFoundError("order #123 not found"))
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/not-found", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}

		if env.Success {
			t.Errorf("expected success false")
		}
		if env.Error == nil || env.Error.Code != response.ErrCodeNotFound {
			t.Errorf("expected code NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("Standard error intercepted as 500", func(t *testing.T) {
		r := gin.New()
		r.Use(ErrorHandler(zap.NewNop()))

		r.GET("/db-err", func(c *gin.Context) {
			_ = c.Error(errors.New("raw sql timeout"))
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/db-err", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}

		var env response.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}

		if env.Error.Code != response.ErrCodeInternalServerError {
			t.Errorf("expected code INTERNAL_SERVER_ERROR, got %s", env.Error.Code)
		}
	})
}

