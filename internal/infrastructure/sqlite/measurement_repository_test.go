package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := NewConnection(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func TestMeasurementRepository_Save(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	m := &domain.Measurement{
		RoomID:      "test-room",
		Timestamp:   time.Now().UTC(),
		Temperature: 22.5,
		Humidity:    45.0,
		Pressure:    1013.25,
	}

	err := repo.Save(ctx, m)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if m.ID == 0 {
		t.Error("Expected ID to be set after save")
	}

	// Verify room was auto-registered
	rooms, err := repo.GetAllRooms(ctx)
	if err != nil {
		t.Fatalf("GetAllRooms failed: %v", err)
	}

	if len(rooms) != 1 {
		t.Errorf("Expected 1 room, got %d", len(rooms))
	}

	if rooms[0].ID != "test-room" {
		t.Errorf("Expected room ID 'test-room', got %s", rooms[0].ID)
	}
}

func TestMeasurementRepository_SaveWithCO2(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	co2 := 400.0
	m := &domain.Measurement{
		RoomID:      "living-room",
		Timestamp:   time.Now().UTC(),
		Temperature: 23.0,
		Humidity:    50.0,
		Pressure:    1015.0,
		CO2:         &co2,
	}

	err := repo.Save(ctx, m)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Retrieve and verify CO2 is stored
	retrieved, err := repo.GetLatestByRoom(ctx, "living-room")
	if err != nil {
		t.Fatalf("GetLatestByRoom failed: %v", err)
	}

	if retrieved.CO2 == nil {
		t.Error("Expected CO2 to be set")
	} else if *retrieved.CO2 != 400.0 {
		t.Errorf("Expected CO2 400.0, got %f", *retrieved.CO2)
	}
}

func TestMeasurementRepository_GetLatestByRoom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert multiple measurements
	measurements := []*domain.Measurement{
		{
			RoomID:      "room1",
			Timestamp:   now.Add(-2 * time.Hour),
			Temperature: 20.0,
			Humidity:    40.0,
			Pressure:    1013.0,
		},
		{
			RoomID:      "room1",
			Timestamp:   now.Add(-1 * time.Hour),
			Temperature: 21.0,
			Humidity:    42.0,
			Pressure:    1014.0,
		},
		{
			RoomID:      "room1",
			Timestamp:   now,
			Temperature: 22.0,
			Humidity:    44.0,
			Pressure:    1015.0,
		},
	}

	for _, m := range measurements {
		if err := repo.Save(ctx, m); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Get latest measurement
	latest, err := repo.GetLatestByRoom(ctx, "room1")
	if err != nil {
		t.Fatalf("GetLatestByRoom failed: %v", err)
	}

	if latest == nil {
		t.Fatal("Expected latest measurement, got nil")
	}

	if latest.Temperature != 22.0 {
		t.Errorf("Expected temperature 22.0, got %f", latest.Temperature)
	}

	// Test non-existent room
	latest, err = repo.GetLatestByRoom(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetLatestByRoom failed for non-existent room: %v", err)
	}

	if latest != nil {
		t.Error("Expected nil for non-existent room")
	}
}

func TestMeasurementRepository_GetByRoomAndTimeRange(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert measurements at different times
	measurements := []*domain.Measurement{
		{
			RoomID:      "room1",
			Timestamp:   now.Add(-3 * time.Hour),
			Temperature: 19.0,
			Humidity:    38.0,
			Pressure:    1012.0,
		},
		{
			RoomID:      "room1",
			Timestamp:   now.Add(-2 * time.Hour),
			Temperature: 20.0,
			Humidity:    40.0,
			Pressure:    1013.0,
		},
		{
			RoomID:      "room1",
			Timestamp:   now.Add(-1 * time.Hour),
			Temperature: 21.0,
			Humidity:    42.0,
			Pressure:    1014.0,
		},
		{
			RoomID:      "room1",
			Timestamp:   now,
			Temperature: 22.0,
			Humidity:    44.0,
			Pressure:    1015.0,
		},
	}

	for _, m := range measurements {
		if err := repo.Save(ctx, m); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Query for last 2 hours
	results, err := repo.GetByRoomAndTimeRange(ctx, "room1", now.Add(-2*time.Hour), now)
	if err != nil {
		t.Fatalf("GetByRoomAndTimeRange failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 measurements, got %d", len(results))
	}

	// Verify ordering (should be ASC by timestamp)
	if len(results) >= 2 && results[0].Timestamp.After(results[1].Timestamp) {
		t.Error("Expected results to be ordered by timestamp ASC")
	}

	// Verify temperature values match expected range
	expectedTemps := []float64{20.0, 21.0, 22.0}
	for i, m := range results {
		if m.Temperature != expectedTemps[i] {
			t.Errorf("Expected temperature %f at index %d, got %f", expectedTemps[i], i, m.Temperature)
		}
	}
}

func TestMeasurementRepository_GetAllRooms(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert measurements for multiple rooms
	rooms := []string{"kitchen", "bedroom", "living-room"}
	for _, roomID := range rooms {
		m := &domain.Measurement{
			RoomID:      roomID,
			Timestamp:   now,
			Temperature: 22.0,
			Humidity:    45.0,
			Pressure:    1013.0,
		}
		if err := repo.Save(ctx, m); err != nil {
			t.Fatalf("Save failed for room %s: %v", roomID, err)
		}
	}

	// Get all rooms
	allRooms, err := repo.GetAllRooms(ctx)
	if err != nil {
		t.Fatalf("GetAllRooms failed: %v", err)
	}

	if len(allRooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(allRooms))
	}

	// Verify room IDs
	roomIDs := make(map[string]bool)
	for _, room := range allRooms {
		roomIDs[room.ID] = true

		if room.LastMeasurement == nil {
			t.Errorf("Expected LastMeasurement to be set for room %s", room.ID)
		}
	}

	for _, expectedID := range rooms {
		if !roomIDs[expectedID] {
			t.Errorf("Expected room %s to be in results", expectedID)
		}
	}
}

func TestMeasurementRepository_AutoRegistration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMeasurementRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// Save first measurement for a room
	m1 := &domain.Measurement{
		RoomID:      "test-room",
		Timestamp:   now.Add(-1 * time.Hour),
		Temperature: 20.0,
		Humidity:    40.0,
		Pressure:    1013.0,
	}

	if err := repo.Save(ctx, m1); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Save second measurement for the same room
	m2 := &domain.Measurement{
		RoomID:      "test-room",
		Timestamp:   now,
		Temperature: 22.0,
		Humidity:    45.0,
		Pressure:    1014.0,
	}

	if err := repo.Save(ctx, m2); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Verify only one room exists
	rooms, err := repo.GetAllRooms(ctx)
	if err != nil {
		t.Fatalf("GetAllRooms failed: %v", err)
	}

	if len(rooms) != 1 {
		t.Errorf("Expected 1 room, got %d", len(rooms))
	}

	// Verify last measurement timestamp was updated
	if rooms[0].LastMeasurement == nil {
		t.Fatal("Expected LastMeasurement to be set")
	}

	// The last measurement time should be close to 'now'
	timeDiff := now.Sub(*rooms[0].LastMeasurement)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("Expected LastMeasurement to be close to now, diff: %v", timeDiff)
	}
}
