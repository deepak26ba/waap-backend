package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/internal/config"
	"waap-backend/internal/middleware"
	"waap-backend/pkg/db"
	"waap-backend/pkg/logger"
)

// setupRouter sets up the Gin router with zap logging middleware and routes.
func setupRouter(pool *pgxpool.Pool, queries *db.Queries, log *zap.Logger) *gin.Engine {
	var router *gin.Engine

	if log != nil {
		router = gin.New()
		router.Use(middleware.ZapLogger(log), middleware.ZapRecovery(log))
	} else {
		router = gin.Default()
	}

	// Health check probe
	router.GET("/healthz", func(c *gin.Context) {
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

	return router
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		// Fallback logger if config fails
		fallbackLog, _ := zap.NewProduction()
		fallbackLog.Fatal("failed to load configuration", zap.Error(err))
	}

	log, err := logger.New(cfg.Env)
	if err != nil {
		panic("failed to initialize zap logger: " + err.Error())
	}
	defer func() {
		_ = log.Sync()
	}()

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("Connecting to database...",
		zap.String("host", cfg.DBHost),
		zap.String("port", cfg.DBPort),
		zap.String("database", cfg.DBName),
	)

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()
	log.Info("Database connection pool established successfully")

	queries := db.New(pool)
	router := setupRouter(pool, queries, log)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context
	go func() {
		log.Info("Starting HTTP server", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("HTTP server listen failed", zap.Error(err))
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down HTTP server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited cleanly")
}
