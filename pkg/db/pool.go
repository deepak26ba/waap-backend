package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"waap-backend/internal/config"
)

// NewPool initializes and validates a PostgreSQL connection pool using pgxpool.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	if cfg.DBMaxConns > 0 {
		poolConfig.MaxConns = cfg.DBMaxConns
	}
	if cfg.DBMinConns > 0 {
		poolConfig.MinConns = cfg.DBMinConns
	}
	if cfg.DBMaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	}
	if cfg.DBMaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime
	}

	// Connect with timeout
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}

// HealthCheck verifies database connectivity and query execution.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database connection pool is nil")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	queries := New(pool)
	_, err := queries.CheckHealth(pingCtx)
	if err != nil {
		return fmt.Errorf("health query failed: %w", err)
	}

	return nil
}

