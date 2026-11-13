package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"waap-backend/internal/model"
	"waap-backend/internal/service"
	"waap-backend/pkg/response"
)

const (
	ContextUserID    = "user_id"
	ContextUserEmail = "user_email"
	ContextUserRole  = "user_role"
	ContextClaims    = "claims"
)

// RequireAuth authenticates incoming requests using a Bearer JWT access token.
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "authorization header is required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "authorization header must be in format 'Bearer <token>'")
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "token string cannot be empty")
			return
		}

		claims, err := service.ValidateToken(tokenStr, jwtSecret)
		if err != nil {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "invalid or expired token")
			return
		}

		if claims.TokenType != model.TokenTypeAccess {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "access token required, refresh token is not permitted for authentication")
			return
		}

		// Inject authenticated claims into Gin context
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUserEmail, claims.Email)
		c.Set(ContextUserRole, claims.Role)
		c.Set(ContextClaims, claims)

		c.Next()
	}
}

// RequireRoles restricts route access to users holding one of the specified roles.
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowedRoles := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowedRoles[strings.ToLower(r)] = struct{}{}
	}

	return func(c *gin.Context) {
		roleVal, exists := c.Get(ContextUserRole)
		if !exists {
			response.Unauthorized(c, response.ErrCodeUnauthorized, "unauthenticated user context")
			return
		}

		userRole, ok := roleVal.(string)
		if !ok {
			response.Forbidden(c, response.ErrCodeForbidden, "invalid user role context")
			return
		}

		if _, allowed := allowedRoles[strings.ToLower(userRole)]; !allowed {
			response.Forbidden(c, response.ErrCodeForbidden, "access denied: insufficient role privileges")
			return
		}

		c.Next()
	}
}

// RequireAdmin is a convenience helper enforcing the 'admin' role.
func RequireAdmin() gin.HandlerFunc {
	return RequireRoles("admin")
}

// GetUserID retrieves the authenticated user's ID from the context.
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get(ContextUserID)
	if !exists {
		return 0, false
	}

	id, ok := val.(int64)
	return id, ok
}

// GetUserEmail retrieves the authenticated user's email from the context.
func GetUserEmail(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextUserEmail)
	if !exists {
		return "", false
	}

	email, ok := val.(string)
	return email, ok
}

// GetUserRole retrieves the authenticated user's role from the context.
func GetUserRole(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextUserRole)
	if !exists {
		return "", false
	}

	role, ok := val.(string)
	return role, ok
}

// GetClaims retrieves the full Claims pointer from the context.
func GetClaims(c *gin.Context) (*model.Claims, bool) {
	val, exists := c.Get(ContextClaims)
	if !exists {
		return nil, false
	}

	claims, ok := val.(*model.Claims)
	return claims, ok
}
