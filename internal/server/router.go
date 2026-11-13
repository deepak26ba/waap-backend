package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/internal/handler"
	"waap-backend/internal/middleware"
	"waap-backend/internal/model"
	"waap-backend/internal/repository"
	"waap-backend/internal/service"
	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

// NewRouter constructs and configures the Gin engine with all global middleware and routes.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, queries *db.Queries, log *zap.Logger) *gin.Engine {
	router := gin.New()

	if log != nil {
		router.Use(middleware.ZapLogger(log))
	}
	router.Use(middleware.Recovery(log))
	router.Use(middleware.ErrorHandler(log))

	// Handle 404 and 405 uniformly with response.Envelope
	router.NoRoute(func(c *gin.Context) {
		response.NotFound(c, response.ErrCodeNotFound, fmt.Sprintf("path %s not found", c.Request.URL.Path))
	})
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", fmt.Sprintf("method %s not allowed on path %s", c.Request.Method, c.Request.URL.Path))
	})

	// Security & Limits Middleware Pipeline
	router.Use(middleware.SecurityHeaders())

	if cfg != nil {
		router.Use(middleware.CORS(cfg.CORSAllowedOrigins))
		if cfg.MaxBodySizeBytes > 0 {
			router.Use(middleware.BodySizeLimiter(cfg.MaxBodySizeBytes))
		}
		if cfg.RateLimitRPS > 0 {
			router.Use(middleware.RateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst))
		}
	} else {
		router.Use(middleware.CORS([]string{"*"}))
		router.Use(middleware.BodySizeLimiter(2 * 1024 * 1024))
	}

	// Observability & System Probes
	registerObservabilityRoutes(router, pool, log)

	// API v1 Routes
	registerAPIRoutes(router, cfg, queries, log)

	return router
}

func registerObservabilityRoutes(r *gin.Engine, pool *pgxpool.Pool, log *zap.Logger) {
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if err := db.HealthCheck(ctx, pool); err != nil {
			if log != nil {
				log.Warn("Health check failed", zap.Error(err))
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "unhealthy",
				"database": "disconnected",
				"error":    err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "healthy",
			"database": "connected",
		})
	})
}

func registerAPIRoutes(r *gin.Engine, cfg *config.Config, queries *db.Queries, log *zap.Logger) {
	if queries == nil {
		return
	}

	jwtSecret := ""
	if cfg != nil {
		jwtSecret = strings.TrimSpace(cfg.JWTSecret)
	}

	if jwtSecret == "" {
		if cfg != nil && cfg.Env == "production" {
			if log != nil {
				log.Fatal("Fatal security misconfiguration: JWT_SECRET must be configured in production")
			}
			panic("Fatal security misconfiguration: JWT_SECRET must be configured in production")
		}
		jwtSecret = "dev-insecure-secret-key-change-in-production-32b"
	}

	// Wire layered components: Repository -> Service -> Handler
	repos := repository.NewRepositories(queries)
	authService := service.NewAuthService(repos.Users, jwtSecret, 15*time.Minute, 7*24*time.Hour)
	authHandler := handler.NewAuthHandler(authService, log)

	apiV1 := r.Group("/api/v1")
	{
		// 1. Auth routes (public)
		authGroup := apiV1.Group("/auth")
		authHandler.RegisterRoutes(authGroup)

		// 2. Protected customer profile routes
		usersGroup := apiV1.Group("/users")
		usersGroup.Use(middleware.RequireAuth(jwtSecret))
		{
			usersGroup.GET("/me", func(c *gin.Context) {
				userID, exists := middleware.GetUserID(c)
				if !exists {
					response.Unauthorized(c, response.ErrCodeUnauthorized, "unauthenticated user context")
					return
				}
				user, err := queries.GetUserByID(c.Request.Context(), userID)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						response.NotFound(c, response.ErrCodeNotFound, "user profile not found")
						return
					}
					response.InternalServerError(c, response.ErrCodeInternalServerError, "failed to retrieve user profile")
					return
				}
				var phone *string
				if user.PhoneNumber.Valid {
					p := user.PhoneNumber.String
					phone = &p
				}
				response.OK(c, model.UserResponse{
					ID:          user.ID,
					Email:       user.Email,
					FullName:    user.FullName,
					Role:        user.Role,
					PhoneNumber: phone,
					CreatedAt:   user.CreatedAt.Time,
				})
			})
		}

		// 3. Protected admin routes
		adminGroup := apiV1.Group("/admin")
		adminGroup.Use(middleware.RequireAuth(jwtSecret), middleware.RequireAdmin())
		{
			adminGroup.GET("/ping", func(c *gin.Context) {
				response.OK(c, gin.H{"status": "admin access granted"})
			})
		}
	}
}
