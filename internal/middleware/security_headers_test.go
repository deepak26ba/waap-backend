package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSecurityHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	expectedHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"X-XSS-Protection":          "1; mode=block",
		"Content-Security-Policy":   "default-src 'self'",
		"Permissions-Policy":        "geolocation=(), camera=(), microphone=()",
	}

	for header, expectedVal := range expectedHeaders {
		actualVal := w.Header().Get(header)
		if actualVal != expectedVal {
			t.Errorf("header %s: expected %q, got %q", header, expectedVal, actualVal)
		}
	}
}
