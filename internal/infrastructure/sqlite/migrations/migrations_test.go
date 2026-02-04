package migrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db
}

func TestRun_FreshDatabase(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := Run(db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify schema_migrations table exists and has version 1
	var version int
	err := db.QueryRow("SELECT version FROM schema_migrations WHERE version = 1").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}

	// Verify rooms table exists
	_, err = db.Exec("SELECT id, name, first_seen_at, last_measurement_at FROM rooms LIMIT 1")
	if err != nil {
		t.Errorf("rooms table not created properly: %v", err)
	}

	// Verify measurements table exists
	_, err = db.Exec("SELECT id, room_id, timestamp, temperature, humidity, pressure, co2 FROM measurements LIMIT 1")
	if err != nil {
		t.Errorf("measurements table not created properly: %v", err)
	}
}

func TestRun_Idempotency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Run migrations twice
	if err := Run(db); err != nil {
		t.Fatalf("First Run failed: %v", err)
	}

	if err := Run(db); err != nil {
		t.Fatalf("Second Run failed: %v", err)
	}

	// Verify only one entry in schema_migrations for version 1
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}
}

func TestRun_MigrationOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Clear registry and add migrations out of order
	originalRegistry := registry
	registry = nil
	defer func() { registry = originalRegistry }()

	Register(Migration{
		Version:     3,
		Description: "Third",
		SQL:         "CREATE TABLE IF NOT EXISTS test_three (id INTEGER PRIMARY KEY)",
	})
	Register(Migration{
		Version:     1,
		Description: "First",
		SQL:         "CREATE TABLE IF NOT EXISTS test_one (id INTEGER PRIMARY KEY)",
	})
	Register(Migration{
		Version:     2,
		Description: "Second",
		SQL:         "CREATE TABLE IF NOT EXISTS test_two (id INTEGER PRIMARY KEY)",
	})

	if err := Run(db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{"test_one", "test_two", "test_three"}
	for _, table := range tables {
		_, err := db.Exec("SELECT * FROM " + table + " LIMIT 1")
		if err != nil {
			t.Errorf("Table %s not created: %v", table, err)
		}
	}

	// Verify migrations were recorded in order
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("Failed to query migrations: %v", err)
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("Failed to scan version: %v", err)
		}
		versions = append(versions, v)
	}

	expected := []int{1, 2, 3}
	if len(versions) != len(expected) {
		t.Fatalf("Expected %d versions, got %d", len(expected), len(versions))
	}
	for i, v := range versions {
		if v != expected[i] {
			t.Errorf("Expected version %d at index %d, got %d", expected[i], i, v)
		}
	}
}

func TestRun_PartialMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Clear registry
	originalRegistry := registry
	registry = nil
	defer func() { registry = originalRegistry }()

	// Run with just migration 1
	Register(Migration{
		Version:     1,
		Description: "First",
		SQL:         "CREATE TABLE IF NOT EXISTS partial_one (id INTEGER PRIMARY KEY)",
	})

	if err := Run(db); err != nil {
		t.Fatalf("First Run failed: %v", err)
	}

	// Add migration 2 and run again
	Register(Migration{
		Version:     2,
		Description: "Second",
		SQL:         "CREATE TABLE IF NOT EXISTS partial_two (id INTEGER PRIMARY KEY)",
	})

	if err := Run(db); err != nil {
		t.Fatalf("Second Run failed: %v", err)
	}

	// Verify both tables exist
	_, err := db.Exec("SELECT * FROM partial_one LIMIT 1")
	if err != nil {
		t.Errorf("Table partial_one not found: %v", err)
	}

	_, err = db.Exec("SELECT * FROM partial_two LIMIT 1")
	if err != nil {
		t.Errorf("Table partial_two not found: %v", err)
	}

	// Verify only 2 migration records
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count migrations: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 migration records, got %d", count)
	}
}
