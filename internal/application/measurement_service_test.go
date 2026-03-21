package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

// Mock repository
type mockMeasurementRepository struct {
	saveFn                  func(ctx context.Context, m *domain.Measurement) error
	getLatestByRoomFn       func(ctx context.Context, roomID string) (*domain.Measurement, error)
	getByRoomAndTimeRangeFn func(ctx context.Context, roomID string, start, end time.Time) ([]*domain.Measurement, error)
	getAllRoomsFn           func(ctx context.Context) ([]*domain.Room, error)
}

func (m *mockMeasurementRepository) Save(ctx context.Context, measurement *domain.Measurement) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, measurement)
	}
	return nil
}

func (m *mockMeasurementRepository) GetLatestByRoom(ctx context.Context, roomID string) (*domain.Measurement, error) {
	if m.getLatestByRoomFn != nil {
		return m.getLatestByRoomFn(ctx, roomID)
	}
	return nil, nil
}

func (m *mockMeasurementRepository) GetByRoomAndTimeRange(ctx context.Context, roomID string, start, end time.Time) ([]*domain.Measurement, error) {
	if m.getByRoomAndTimeRangeFn != nil {
		return m.getByRoomAndTimeRangeFn(ctx, roomID, start, end)
	}
	return nil, nil
}

func (m *mockMeasurementRepository) GetAllRooms(ctx context.Context) ([]*domain.Room, error) {
	if m.getAllRoomsFn != nil {
		return m.getAllRoomsFn(ctx)
	}
	return nil, nil
}

func (m *mockMeasurementRepository) GetMonthlyAverages(ctx context.Context) ([]*domain.MonthlyAverage, error) {
	return nil, nil
}

func TestMeasurementService_RecordMeasurement(t *testing.T) {
	tests := []struct {
		name        string
		measurement *domain.Measurement
		saveFn      func(ctx context.Context, m *domain.Measurement) error
		wantErr     bool
	}{
		{
			name: "valid measurement",
			measurement: &domain.Measurement{
				RoomID:      "test-room",
				Timestamp:   time.Now().UTC().Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			saveFn: func(ctx context.Context, m *domain.Measurement) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "invalid temperature",
			measurement: &domain.Measurement{
				RoomID:      "test-room",
				Timestamp:   time.Now().UTC(),
				Temperature: 150.0, // Invalid
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			saveFn: func(ctx context.Context, m *domain.Measurement) error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "repository save error",
			measurement: &domain.Measurement{
				RoomID:      "test-room",
				Timestamp:   time.Now().UTC().Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			saveFn: func(ctx context.Context, m *domain.Measurement) error {
				return errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockMeasurementRepository{
				saveFn: tt.saveFn,
			}

			service := NewMeasurementService(repo)
			err := service.RecordMeasurement(context.Background(), tt.measurement)

			if (err != nil) != tt.wantErr {
				t.Errorf("RecordMeasurement() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMeasurementService_GetLatestMeasurements(t *testing.T) {
	now := time.Now().UTC()

	repo := &mockMeasurementRepository{
		getAllRoomsFn: func(ctx context.Context) ([]*domain.Room, error) {
			return []*domain.Room{
				{ID: "room1", Name: "Room 1"},
				{ID: "room2", Name: "Room 2"},
			}, nil
		},
		getLatestByRoomFn: func(ctx context.Context, roomID string) (*domain.Measurement, error) {
			measurements := map[string]*domain.Measurement{
				"room1": {
					ID:          1,
					RoomID:      "room1",
					Timestamp:   now,
					Temperature: 22.5,
					Humidity:    45.0,
					Pressure:    1013.25,
				},
				"room2": {
					ID:          2,
					RoomID:      "room2",
					Timestamp:   now,
					Temperature: 20.0,
					Humidity:    50.0,
					Pressure:    1015.0,
				},
			}
			return measurements[roomID], nil
		},
	}

	service := NewMeasurementService(repo)
	result, err := service.GetLatestMeasurements(context.Background())

	if err != nil {
		t.Fatalf("GetLatestMeasurements() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 measurements, got %d", len(result))
	}

	if result["room1"] == nil {
		t.Error("Expected measurement for room1")
	}

	if result["room2"] == nil {
		t.Error("Expected measurement for room2")
	}

	if result["room1"].Temperature != 22.5 {
		t.Errorf("Expected room1 temperature 22.5, got %f", result["room1"].Temperature)
	}
}

func TestMeasurementService_GetLatestMeasurements_Error(t *testing.T) {
	repo := &mockMeasurementRepository{
		getAllRoomsFn: func(ctx context.Context) ([]*domain.Room, error) {
			return nil, errors.New("database error")
		},
	}

	service := NewMeasurementService(repo)
	_, err := service.GetLatestMeasurements(context.Background())

	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestMeasurementService_GetMeasurementHistory(t *testing.T) {
	now := time.Now().UTC()

	repo := &mockMeasurementRepository{
		getByRoomAndTimeRangeFn: func(ctx context.Context, roomID string, start, end time.Time) ([]*domain.Measurement, error) {
			if roomID != "room1" {
				return nil, nil
			}
			return []*domain.Measurement{
				{
					ID:          1,
					RoomID:      "room1",
					Timestamp:   now.Add(-2 * time.Hour),
					Temperature: 20.0,
					Humidity:    40.0,
					Pressure:    1013.0,
				},
				{
					ID:          2,
					RoomID:      "room1",
					Timestamp:   now.Add(-1 * time.Hour),
					Temperature: 21.0,
					Humidity:    42.0,
					Pressure:    1014.0,
				},
			}, nil
		},
	}

	service := NewMeasurementService(repo)
	result, err := service.GetMeasurementHistory(context.Background(), "room1", 24*time.Hour)

	if err != nil {
		t.Fatalf("GetMeasurementHistory() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 measurements, got %d", len(result))
	}

	if result[0].Temperature != 20.0 {
		t.Errorf("Expected first temperature 20.0, got %f", result[0].Temperature)
	}
}

func TestMeasurementService_GetAllRooms(t *testing.T) {
	expectedRooms := []*domain.Room{
		{ID: "room1", Name: "Room 1"},
		{ID: "room2", Name: "Room 2"},
	}

	repo := &mockMeasurementRepository{
		getAllRoomsFn: func(ctx context.Context) ([]*domain.Room, error) {
			return expectedRooms, nil
		},
	}

	service := NewMeasurementService(repo)
	result, err := service.GetAllRooms(context.Background())

	if err != nil {
		t.Fatalf("GetAllRooms() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(result))
	}

	if result[0].ID != "room1" {
		t.Errorf("Expected first room ID 'room1', got %s", result[0].ID)
	}
}
