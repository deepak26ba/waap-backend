package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

// ErrorHandler returns a Gin middleware that intercepts errors appended to c.Errors
// and writes a standardized JSON error envelope if no response was sent yet.
func ErrorHandler(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 && !c.Writer.Written() {
			lastErr := c.Errors.Last()
			var appErr *response.AppError

			if errors.As(lastErr.Err, &appErr) {
				if appErr.StatusCode >= http.StatusInternalServerError && log != nil {
					log.Error("Internal application error",
						zap.Error(appErr),
						zap.String("path", c.Request.URL.Path),
						zap.Int("status", appErr.StatusCode),
					)
				}
				response.HandleError(c, appErr)
			} else {
				if log != nil {
					log.Error("Unhandled request error",
						zap.Error(lastErr.Err),
						zap.String("path", c.Request.URL.Path),
					)
				}
				response.InternalServerError(c, response.ErrCodeInternalServerError, "an unexpected internal server error occurred")
			}
		}
	}
}
