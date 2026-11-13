package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/internal/middleware"
	"waap-backend/internal/server/routes"
	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

// NewRouter constructs and configures the Gin engine with all global middleware and routes.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, queries db.Querier, log *zap.Logger) *gin.Engine {
	router := gin.New()

	setupMiddleware(router, cfg, log)

	routes.Register(router, routes.Dependencies{
		Config:  cfg,
		Pool:    pool,
		Queries: queries,
		Log:     log,
	})

	return router
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, log *zap.Logger) {
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
}
