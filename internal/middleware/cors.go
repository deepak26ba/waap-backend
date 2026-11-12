package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that enforces a strict CORS whitelist.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	// Pre-normalize whitelist into a lookup map and check for wildcard
	allowAll := false
	whitelist := make(map[string]bool)

	for _, o := range allowedOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed == "*" {
			allowAll = true
		} else if trimmed != "" {
			whitelist[strings.ToLower(trimmed)] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" {
			originLower := strings.ToLower(origin)
			matched := allowAll || whitelist[originLower]

			if matched {
				if allowAll {
					c.Header("Access-Control-Allow-Origin", "*")
				} else {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Vary", "Origin")
				}

				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept, Accept-Encoding, Authorization, X-Requested-With, X-Cart-ID")
				c.Header("Access-Control-Expose-Headers", "Content-Length, X-Total-Count, Retry-After")
				c.Header("Access-Control-Max-Age", "86400")
			} else if c.Request.Method == http.MethodOptions {
				// Preflight from disallowed origin
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// Handle OPTIONS preflight request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
