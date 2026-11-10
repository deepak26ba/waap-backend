package db

import (
	"context"
	"testing"
	"time"

	"waap-backend/internal/config"
)

func TestPoolAndQueries_HealthCheck(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Skipf("skipping live database test: %v", err)
	}
	defer pool.Close()

	if err := HealthCheck(ctx, pool); err != nil {
		t.Fatalf("health check failed: %v", err)
	}

	queries := New(pool)
	val, err := queries.CheckHealth(ctx)
	if err != nil {
		t.Fatalf("failed to run CheckHealth query: %v", err)
	}

	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
}

func TestHealthCheck_NilPool(t *testing.T) {
	ctx := context.Background()
	err := HealthCheck(ctx, nil)
	if err == nil {
		t.Fatal("expected error with nil pool, got nil")
	}
}

