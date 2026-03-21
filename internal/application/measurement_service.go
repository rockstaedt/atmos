package application

import (
	"context"
	"fmt"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

type MeasurementService struct {
	repo domain.MeasurementRepository
}

func NewMeasurementService(repo domain.MeasurementRepository) *MeasurementService {
	return &MeasurementService{repo: repo}
}

// RecordMeasurement validates and stores a new measurement
func (s *MeasurementService) RecordMeasurement(ctx context.Context, m *domain.Measurement) error {
	// Validate domain rules
	if err := m.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Persist
	if err := s.repo.Save(ctx, m); err != nil {
		return fmt.Errorf("failed to save measurement: %w", err)
	}

	return nil
}

// GetLatestMeasurements returns the most recent measurement for all rooms
func (s *MeasurementService) GetLatestMeasurements(ctx context.Context) (map[string]*domain.Measurement, error) {
	rooms, err := s.repo.GetAllRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}

	result := make(map[string]*domain.Measurement)
	for _, room := range rooms {
		m, err := s.repo.GetLatestByRoom(ctx, room.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest measurement for room %s: %w", room.ID, err)
		}
		if m != nil {
			result[room.ID] = m
		}
	}

	return result, nil
}

// GetMeasurementHistory returns measurements for a room within a time range
func (s *MeasurementService) GetMeasurementHistory(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error) {
	end := time.Now().UTC()
	start := end.Add(-duration)

	measurements, err := s.repo.GetByRoomAndTimeRange(ctx, roomID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get measurement history: %w", err)
	}

	return measurements, nil
}

// GetAllRooms returns all registered rooms
func (s *MeasurementService) GetAllRooms(ctx context.Context) ([]*domain.Room, error) {
	return s.repo.GetAllRooms(ctx)
}

// GetMonthlyAverages returns per-month averages for all rooms grouped by room
func (s *MeasurementService) GetMonthlyAverages(ctx context.Context) (*MonthlyAveragesPageDTO, error) {
	averages, err := s.repo.GetMonthlyAverages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly averages: %w", err)
	}

	// Group by room, preserving order
	roomIndex := make(map[string]int)
	var rooms []*RoomMonthlyDTO

	for _, avg := range averages {
		idx, ok := roomIndex[avg.RoomID]
		if !ok {
			idx = len(rooms)
			roomIndex[avg.RoomID] = idx
			rooms = append(rooms, &RoomMonthlyDTO{RoomID: avg.RoomID})
		}

		dto := &MonthlyAverageDTO{
			Month:          avg.Month,
			AvgTemperature: avg.AvgTemperature,
			AvgHumidity:    avg.AvgHumidity,
			AvgPressure:    avg.AvgPressure,
			AvgCO2:         avg.AvgCO2,
			SampleCount:    avg.SampleCount,
		}
		rooms[idx].Months = append(rooms[idx].Months, dto)
	}

	return &MonthlyAveragesPageDTO{Rooms: rooms}, nil
}
