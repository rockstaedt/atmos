package migrations

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a database schema migration.
type Migration struct {
	Version     int
	Description string
	SQL         string
}

var registry []Migration

// Register adds a migration to the registry. Called by init() in migration files.
func Register(m Migration) {
	registry = append(registry, m)
}

// Run executes all pending migrations in order.
func Run(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	// Sort migrations by version
	sorted := make([]Migration, len(registry))
	copy(sorted, registry)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	for _, m := range sorted {
		if applied[m.Version] {
			continue
		}

		if err := runMigration(db, m); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Description, err)
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL
		)
	`)
	return err
}

func getAppliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func runMigration(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.SQL); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		m.Version, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
