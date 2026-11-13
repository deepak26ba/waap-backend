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

	// Detect legacy schema incompatibility: if users.id is bigint instead of uuid,
	// reset legacy tables so 000001_init_schema can reapply the canonical UUID schema.
	var idDataType string
	_ = pool.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns 
		WHERE table_name = 'users' AND column_name = 'id'
	`).Scan(&idDataType)

	if idDataType == "bigint" {
		downCandidates := []string{
			"sql/migrations/000001_init_schema.down.sql",
			"/app/sql/migrations/000001_init_schema.down.sql",
			"../sql/migrations/000001_init_schema.down.sql",
			"../../sql/migrations/000001_init_schema.down.sql",
		}
		for _, p := range downCandidates {
			if ddl, readErr := os.ReadFile(p); readErr == nil {
				_, _ = pool.Exec(ctx, string(ddl))
				_, _ = pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = '000001_init_schema'")
				break
			}
		}
	}

	// Ensure email_verified column exists if users table was already created by an earlier run
	_, _ = pool.Exec(ctx, `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables WHERE table_name = 'users'
			) THEN
				ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
			END IF;
		END $$;
	`)

	migrations := []struct {
		version string
		file    string
	}{
		{version: "000001_init_schema", file: "000001_init_schema.up.sql"},
	}

	for _, m := range migrations {
		var applied bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM schema_migrations WHERE version = $1
			);
		`, m.version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("failed to query migration status for %s: %w", m.version, err)
		}

		if applied {
			continue
		}

		candidates := []string{
			fmt.Sprintf("sql/migrations/%s", m.file),
			fmt.Sprintf("/app/sql/migrations/%s", m.file),
			fmt.Sprintf("../sql/migrations/%s", m.file),
			fmt.Sprintf("../../sql/migrations/%s", m.file),
		}

		var ddl []byte
		for _, p := range candidates {
			if data, readErr := os.ReadFile(p); readErr == nil {
				ddl = data
				break
			}
		}

		if len(ddl) == 0 {
			return fmt.Errorf("migration file %s not found in candidate paths", m.file)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to start migration transaction for %s: %w", m.version, err)
		}

		if _, err := tx.Exec(ctx, string(ddl)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to apply migration %s: %w", m.version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record schema version for %s: %w", m.version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", m.version, err)
		}
	}

	return nil
}
