package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a Gin middleware that injects recommended OWASP security headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking by prohibiting the page from being rendered in <frame>, <iframe>, <embed> or <object>
		c.Header("X-Frame-Options", "DENY")

		// Prevent browsers from MIME-sniffing a response away from the declared content-type
		c.Header("X-Content-Type-Options", "nosniff")

		// Enforce HTTPS connections with HTTP Strict Transport Security (HSTS)
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Control how much referrer information should be included with requests
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable legacy cross-site scripting filter in supported browsers
		c.Header("X-XSS-Protection", "1; mode=block")

		// Content Security Policy restricting resources to own origin
		c.Header("Content-Security-Policy", "default-src 'self'")

		// Restrict access to browser features / APIs
		c.Header("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

		c.Next()
	}
}
