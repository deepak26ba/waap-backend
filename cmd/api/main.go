package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"waap-backend/internal/config"
	"waap-backend/internal/server"
	"waap-backend/pkg/db"
	"waap-backend/pkg/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fallbackLog, _ := zap.NewProduction()
		fallbackLog.Fatal("failed to load configuration", zap.Error(err))
	}

	log, err := logger.New(cfg.Env)
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer func() { _ = log.Sync() }()

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

	srv := server.New(cfg, pool, db.New(pool), log)

	// Graceful shutdown listener on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("Shutting down HTTP server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited cleanly")
}
