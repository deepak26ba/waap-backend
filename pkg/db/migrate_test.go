package db

import (
	"context"
	"testing"
	"time"

	"waap-backend/internal/config"
)

func TestAutoMigrate_LiveDB(t *testing.T) {
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

	if err := AutoMigrate(ctx, pool); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Verify idempotency: running a second time should succeed without error
	if err := AutoMigrate(ctx, pool); err != nil {
		t.Fatalf("AutoMigrate idempotent run failed: %v", err)
	}
}

func TestAutoMigrate_NilPool(t *testing.T) {
	err := AutoMigrate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error with nil pool, got nil")
	}
}
