package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestRateLimiter_AllowedAndBlocked(t *testing.T) {
	// Limiter allowing 5 requests burst with 1 req/sec
	ipLimiter := NewIPRateLimiter(rate.Limit(1), 3, 1*time.Minute)
	defer ipLimiter.Stop()

	router := gin.New()
	router.Use(ipLimiter.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	clientIP := "192.168.1.100:1234"

	// First 3 requests (burst) should pass
	for i := 1; i <= 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// 4th immediate request should be rate-limited
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d. Body: %s", w.Code, w.Body.String())
	}

	if retryAfter := w.Header().Get("Retry-After"); retryAfter == "" {
		t.Errorf("expected Retry-After header to be set on 429 response")
	}
}

func TestRateLimiter_DifferentIPsAreIsolated(t *testing.T) {
	ipLimiter := NewIPRateLimiter(rate.Limit(1), 1, 1*time.Minute)
	defer ipLimiter.Stop()

	router := gin.New()
	router.Use(ipLimiter.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	ip1 := "10.0.0.1:1001"
	ip2 := "10.0.0.2:1002"

	// IP 1: first request passes
	req1, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req1.RemoteAddr = ip1
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1: expected 200, got %d", w1.Code)
	}

	// IP 1: second request fails
	req1Again, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req1Again.RemoteAddr = ip1
	w1Again := httptest.NewRecorder()
	router.ServeHTTP(w1Again, req1Again)
	if w1Again.Code != http.StatusTooManyRequests {
		t.Fatalf("IP1: expected 429, got %d", w1Again.Code)
	}

	// IP 2: first request passes despite IP 1 being exhausted
	req2, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req2.RemoteAddr = ip2
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("IP2: expected 200, got %d", w2.Code)
	}
}

