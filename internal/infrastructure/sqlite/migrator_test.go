package sqlite

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := Migrate(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify tables exist
	tables := []string{"rooms", "measurements"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_measurements_room_timestamp",
		"idx_measurements_timestamp",
		"idx_measurements_room",
	}
	for _, index := range indexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
			index,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", index, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations twice - should not error
	if err := Migrate(db); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second migration failed (should be idempotent): %v", err)
	}
}

func TestMigrateDown(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations up
	if err := Migrate(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Run migrations down
	if err := MigrateDown(db); err != nil {
		t.Fatalf("failed to rollback migrations: %v", err)
	}

	// Verify tables no longer exist
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('rooms', 'measurements')",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query tables: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tables after rollback, got %d", count)
	}
}

func TestGetMigrationVersion(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Before migrations, version should be 0
	version, dirty, err := GetMigrationVersion(db)
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 before migrations, got %d", version)
	}
	if dirty {
		t.Error("expected clean state before migrations")
	}

	// After migrations, version should be 1
	if err := Migrate(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	version, dirty, err = GetMigrationVersion(db)
	if err != nil {
		t.Fatalf("failed to get version after migration: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1 after migrations, got %d", version)
	}
	if dirty {
		t.Error("expected clean state after migrations")
	}
}
