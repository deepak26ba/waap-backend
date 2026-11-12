package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBodySizeLimiter_AllowedPayload(t *testing.T) {
	limit := int64(1024) // 1KB
	router := gin.New()
	router.Use(BodySizeLimiter(limit))
	router.POST("/upload", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "read %d bytes", len(body))
	})

	smallBody := strings.Repeat("A", 500)
	req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString(smallBody))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestBodySizeLimiter_ContentLengthExceeded(t *testing.T) {
	limit := int64(1024) // 1KB
	router := gin.New()
	router.Use(BodySizeLimiter(limit))
	router.POST("/upload", func(c *gin.Context) {
		c.String(http.StatusOK, "should not be reached")
	})

	oversizedBody := strings.Repeat("A", 2048)
	req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString(oversizedBody))
	req.Header.Set("Content-Type", "text/plain")
	req.ContentLength = int64(len(oversizedBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 Payload Too Large, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodySizeLimiter_StreamingBodyExceeded(t *testing.T) {
	limit := int64(512) // 512 bytes
	router := gin.New()
	router.Use(BodySizeLimiter(limit))
	router.POST("/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			// Handled by limitedReadCloser which aborts with 413
			return
		}
		c.String(http.StatusOK, "should not succeed")
	})

	oversizedStream := strings.Repeat("A", 1024)
	req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString(oversizedStream))
	req.Header.Set("Content-Type", "text/plain")
	// simulate chunked encoding / unknown content length
	req.ContentLength = -1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 Payload Too Large, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodySizeLimiter_Default2MBExceeded(t *testing.T) {
	maxBytes2MB := int64(2 * 1024 * 1024) // 2MB
	router := gin.New()
	router.Use(BodySizeLimiter(maxBytes2MB))
	router.POST("/upload", func(c *gin.Context) {
		c.String(http.StatusOK, "should not be reached")
	})

	// 2MB + 1 byte
	req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewReader([]byte{}))
	req.ContentLength = maxBytes2MB + 1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 for >2MB payload, got %d", w.Code)
	}
}
