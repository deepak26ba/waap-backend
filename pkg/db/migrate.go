package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutoMigrate applies initial schema migrations if not already applied.
func AutoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database connection pool is nil")
	}

	// Ensure tracking table exists
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to initialize schema_migrations table: %w", err)
	}

	// Check if initial schema is already applied
	var applied bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM schema_migrations WHERE version = '000001_init_schema'
		);
	`).Scan(&applied)
	if err != nil {
		return fmt.Errorf("failed to query migration status: %w", err)
	}

	if applied {
		return nil
	}

	// Locate migration script across standard local and container paths
	candidates := []string{
		"sql/migrations/000001_init_schema.up.sql",
		"/app/sql/migrations/000001_init_schema.up.sql",
		"../sql/migrations/000001_init_schema.up.sql",
		"../../sql/migrations/000001_init_schema.up.sql",
	}

	var ddl []byte
	for _, p := range candidates {
		if data, readErr := os.ReadFile(p); readErr == nil {
			ddl = data
			break
		}
	}

	if len(ddl) == 0 {
		return fmt.Errorf("migration file 000001_init_schema.up.sql not found in candidate paths")
	}

	// Apply migration atomically
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(ddl)); err != nil {
		return fmt.Errorf("failed to apply migration schema: %w", err)
	}

	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ('000001_init_schema')"); err != nil {
		return fmt.Errorf("failed to record schema version: %w", err)
	}

	return tx.Commit(ctx)
}
