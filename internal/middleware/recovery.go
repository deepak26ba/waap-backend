package middleware

import (
	"errors"
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

// Recovery returns a Gin middleware that recovers from any panic, logs the full stack trace,
// and returns a consistent JSON 500 error envelope without terminating the server.
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Check for broken pipe or connection reset
				var brokenPipe bool
				if ne, ok := r.(*net.OpError); ok {
					var se *os.SyscallError
					if errors.As(ne, &se) {
						errStr := strings.ToLower(se.Error())
						if strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				if log != nil {
					fields := []zap.Field{
						zap.Any("error", r),
						zap.String("method", c.Request.Method),
						zap.String("path", c.Request.URL.Path),
						zap.String("query", c.Request.URL.RawQuery),
						zap.String("ip", c.ClientIP()),
						zap.String("user-agent", c.Request.UserAgent()),
						zap.Stack("stack"),
					}

					if brokenPipe {
						log.Warn("Client connection broken during request processing", fields...)
					} else {
						log.Error("Unhandled panic recovered", fields...)
					}
				}

				if brokenPipe {
					// If the connection is dead, we cannot write a response to it.
					c.Abort()
					return
				}

				// If response has not yet been committed to the client, write standard 500 envelope
				if !c.Writer.Written() {
					response.InternalServerError(c, response.ErrCodeInternalServerError, "an unexpected internal server error occurred")
				} else {
					c.Abort()
				}
			}
		}()

		c.Next()
	}
}

// ZapRecovery is an alias preserved for backwards compatibility with the logger package.
func ZapRecovery(log *zap.Logger) gin.HandlerFunc {
	return Recovery(log)
}
