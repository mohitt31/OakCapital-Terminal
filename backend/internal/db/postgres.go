package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

// RunMigrations executes sql files found in internal/db/migrations/sql that
// haven't been applied yet. Tracks applied migrations in a schema_migrations table.
func RunMigrations(ctx context.Context) error {
	// Create tracking table if it doesn't exist
	_, err := Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	dir := "internal/db/migrations/sql"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		// Check if already applied
		var exists bool
		err := Pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)",
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", filename, err)
		}
		if exists {
			fmt.Printf("Migration already applied, skipping: %s\n", filename)
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		// Run migration and record it in a transaction
		tx, err := Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to start transaction for %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (filename) VALUES ($1)", filename); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", filename, err)
		}

		fmt.Printf("Successfully ran migration: %s\n", filename)
	}
	return nil
}

// InitDB initializes the connection pool to PostgreSQL (Supabase).
func InitDB(ctx context.Context, dbURL string) error {
	if dbURL == "mock" {
		fmt.Println("⚠️  MOCK DATABASE MODE ENABLED (No real DB connection)")
		return nil
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	// Disable prepared statements for compatibility with Supabase Transaction Pooler (PgBouncer)
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// For Supabase we may want to limit concurrent connections if it's a small pooler,
	// but pgx defaults are usually fine.
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close() // prevent connection leak if ping fails
		return fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("🚀 Successfully connected to Database!")
	Pool = pool
	return nil
}

// CloseDB closes the connection pool.
func CloseDB() {
	if Pool != nil {
		Pool.Close()
	}
}
