package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/internal/config"
	"waap-backend/internal/middleware"
	"waap-backend/pkg/db"
)

// NewRouter constructs and configures the Gin engine with all global middleware and routes.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, queries *db.Queries, log *zap.Logger) *gin.Engine {
	var router *gin.Engine

	if log != nil {
		router = gin.New()
		router.Use(middleware.ZapLogger(log), middleware.ZapRecovery(log))
	} else {
		router = gin.Default()
	}

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

