package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages per-IP rate limiters using a token bucket algorithm.
type IPRateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*clientLimiter
	rate     rate.Limit
	burst    int
	ttl      time.Duration
	stopChan chan struct{}
}

// NewIPRateLimiter creates an IPRateLimiter and starts a periodic cleanup worker for stale IPs.
func NewIPRateLimiter(r rate.Limit, burst int, ttl time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients:  make(map[string]*clientLimiter),
		rate:     r,
		burst:    burst,
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	// Background worker to remove inactive client limiters and avoid memory leaks
	go limiter.cleanupLoop()

	return limiter
}

// GetLimiter returns the rate.Limiter for a given IP address, creating one if necessary.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	client, exists := i.clients[ip]
	now := time.Now()

	if !exists {
		client = &clientLimiter{
			limiter:  rate.NewLimiter(i.rate, i.burst),
			lastSeen: now,
		}
		i.clients[ip] = client
		return client.limiter
	}

	client.lastSeen = now
	return client.limiter
}

func (i *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(i.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			i.cleanup()
		case <-i.stopChan:
			return
		}
	}
}

func (i *IPRateLimiter) cleanup() {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	for ip, client := range i.clients {
		if now.Sub(client.lastSeen) > i.ttl {
			delete(i.clients, ip)
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (i *IPRateLimiter) Stop() {
	select {
	case <-i.stopChan:
		// already stopped
	default:
		close(i.stopChan)
	}
}

// Middleware returns a Gin middleware enforcing the IP rate limit.
func (i *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		limiter := i.GetLimiter(ip)
		if !limiter.Allow() {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "too many requests, please try again later",
				},
			})
			return
		}

		c.Next()
	}
}

// RateLimiter is a convenience helper to create and return a rate limiting middleware.
func RateLimiter(rps float64, burst int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(rate.Limit(rps), burst, 10*time.Minute)
	return limiter.Middleware()
}
