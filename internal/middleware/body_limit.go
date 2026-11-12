package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"waap-backend/pkg/response"
)

// ErrPayloadTooLarge is returned when a request body exceeds the configured maximum bytes.
var ErrPayloadTooLarge = errors.New("request payload too large")

type limitedReadCloser struct {
	reader   io.ReadCloser
	maxBytes int64
	read     int64
	context  *gin.Context
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.read >= l.maxBytes {
		l.abortPayloadTooLarge()
		return 0, ErrPayloadTooLarge
	}

	remaining := l.maxBytes - l.read
	toRead := p
	if int64(len(p)) > remaining+1 {
		toRead = p[:remaining+1]
	}

	n, err := l.reader.Read(toRead)
	l.read += int64(n)

	if l.read > l.maxBytes {
		l.abortPayloadTooLarge()
		return n, ErrPayloadTooLarge
	}

	return n, err
}

func (l *limitedReadCloser) abortPayloadTooLarge() {
	if !l.context.IsAborted() {
		response.PayloadTooLarge(l.context, response.ErrCodePayloadTooLarge, fmt.Sprintf("request body exceeds maximum allowed size of %d bytes", l.maxBytes))
	}
}

func (l *limitedReadCloser) Close() error {
	if l.reader == nil {
		return nil
	}
	return l.reader.Close()
}

// BodySizeLimiter limits incoming request body size to maxBytes (e.g. 2MB).
func BodySizeLimiter(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "PAYLOAD_TOO_LARGE",
					"message": fmt.Sprintf("request body exceeds maximum allowed size of %d bytes", maxBytes),
				},
			})
			return
		}

		if c.Request.Body != nil {
			c.Request.Body = &limitedReadCloser{
				reader:   c.Request.Body,
				maxBytes: maxBytes,
				context:  c,
			}
		}

		c.Next()
	}
}
