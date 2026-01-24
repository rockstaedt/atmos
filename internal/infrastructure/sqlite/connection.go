package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Config struct {
	Path string
}

// NewConnection creates a SQLite connection with optimal settings for time-series data
func NewConnection(cfg Config) (*sql.DB, error) {
	// Connection string with pragmas for optimal performance
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode%%3DWAL&_pragma=busy_timeout%%3D5000&_pragma=synchronous%%3DNORMAL&_pragma=cache_size%%3D-64000&_pragma=foreign_keys%%3DON", cfg.Path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool settings (SQLite benefits from limited concurrency)
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}
