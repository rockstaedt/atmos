package domain

import (
	"context"
	"time"
)

// MeasurementRepository defines the interface for measurement persistence
type MeasurementRepository interface {
	// Save stores a measurement and auto-registers the room if needed
	Save(ctx context.Context, m *Measurement) error

	// GetLatestByRoom returns the most recent measurement for a room
	GetLatestByRoom(ctx context.Context, roomID string) (*Measurement, error)

	// GetByRoomAndTimeRange retrieves measurements within a time range
	GetByRoomAndTimeRange(ctx context.Context, roomID string, start, end time.Time) ([]*Measurement, error)

	// GetAllRooms returns all registered rooms with their last measurement time
	GetAllRooms(ctx context.Context) ([]*Room, error)
}
