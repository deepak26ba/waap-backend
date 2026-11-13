package routes

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"waap-backend/internal/cart"
	"waap-backend/internal/middleware"
	"waap-backend/internal/product"
	"waap-backend/internal/user"
	"waap-backend/pkg/email"
	"waap-backend/pkg/response"
)

// RegisterV1 mounts all version 1 API endpoints under the /api/v1 route group.
func RegisterV1(r *gin.Engine, deps Dependencies) {
	if deps.Queries == nil {
		return
	}

	jwtSecret := resolveJWTSecret(deps)

	var mailer email.Sender
	if deps.Config != nil && deps.Config.BrevoAPIKey != "" {
		mailer = email.NewBrevoSender(deps.Config.BrevoAPIKey, deps.Config.BrevoFromEmail, deps.Config.BrevoFromName)
	} else {
		mailer = &email.NoopSender{}
	}

	verificationTTL := 5 * time.Minute
	if deps.Config != nil && deps.Config.VerificationTokenTTL > 0 {
		verificationTTL = deps.Config.VerificationTokenTTL
	}

	tokenStore := user.NewMemoryTokenStore(1 * time.Minute)

	// Domain module wiring
	userRepo := user.NewRepository(deps.Queries)
	userService := user.NewService(userRepo, tokenStore, mailer, deps.Log, jwtSecret, 15*time.Minute, 7*24*time.Hour, verificationTTL)
	userHandler := user.NewHandler(userService, deps.Log)

	productRepo := product.NewRepository(deps.Queries)
	productService := product.NewService(productRepo)
	productHandler := product.NewHandler(productService, deps.Log)

	cartRepo := cart.NewRepository(deps.Pool, deps.Queries)
	cartService := cart.NewService(cartRepo)
	cartHandler := cart.NewHandler(cartService, deps.Log)

	apiV1 := r.Group("/api/v1")
	{
		// Public Authentication Routes
		auth := apiV1.Group("/auth")
		{
			auth.POST("/register", userHandler.Register)
			auth.POST("/login", userHandler.Login)
			auth.POST("/refresh", userHandler.Refresh)
			auth.POST("/verify-email", userHandler.VerifyEmail)
			auth.POST("/resend-verification", userHandler.ResendVerification)
		}

		// Public Product Catalog Routes
		catalog := apiV1.Group("/catalog")
		{
			catalog.GET("/categories", productHandler.ListCategories)
			catalog.GET("/products/:id", productHandler.GetProduct)
		}

		// Shopping Cart Routes (Optional Authentication)
		cartGroup := apiV1.Group("/cart")
		cartGroup.Use(middleware.OptionalAuth(jwtSecret))
		{
			cartGroup.GET("", cartHandler.GetCart)
		}

		// Authenticated Customer Profile Routes
		users := apiV1.Group("/users")
		users.Use(middleware.RequireAuth(jwtSecret))
		{
			users.GET("/me", userHandler.GetProfile)
		}

		// Protected Admin Routes
		admin := apiV1.Group("/admin")
		admin.Use(middleware.RequireAuth(jwtSecret), middleware.RequireAdmin())
		{
			admin.GET("/ping", func(c *gin.Context) {
				response.OK(c, gin.H{"status": "admin access granted"})
			})
		}
	}
}

func resolveJWTSecret(deps Dependencies) string {
	jwtSecret := ""
	if deps.Config != nil {
		jwtSecret = strings.TrimSpace(deps.Config.JWTSecret)
	}

	if jwtSecret == "" {
		if deps.Config != nil && deps.Config.Env == "production" {
			if deps.Log != nil {
				deps.Log.Error("Fatal security misconfiguration: JWT_SECRET must be configured in production")
			}
			panic("Fatal security misconfiguration: JWT_SECRET must be configured in production")
		}
		jwtSecret = "dev-insecure-secret-key-change-in-production-32b"
	}
	return jwtSecret
}
