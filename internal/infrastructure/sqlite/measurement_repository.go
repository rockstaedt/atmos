package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

type MeasurementRepository struct {
	db *sql.DB
}

func NewMeasurementRepository(db *sql.DB) *MeasurementRepository {
	return &MeasurementRepository{db: db}
}

func (r *MeasurementRepository) Save(ctx context.Context, m *domain.Measurement) error {
	// Start transaction to handle auto-registration + save atomically
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Auto-register room if it doesn't exist
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rooms (id, name, first_seen_at, last_measurement_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_measurement_at = excluded.last_measurement_at
	`, m.RoomID, m.RoomID, m.Timestamp.UTC(), m.Timestamp.UTC())
	if err != nil {
		return fmt.Errorf("failed to upsert room: %w", err)
	}

	// Insert measurement
	result, err := tx.ExecContext(ctx, `
		INSERT INTO measurements (room_id, timestamp, temperature, humidity, pressure, co2)
		VALUES (?, ?, ?, ?, ?, ?)
	`, m.RoomID, m.Timestamp.UTC(), m.Temperature, m.Humidity, m.Pressure, m.CO2)
	if err != nil {
		return fmt.Errorf("failed to insert measurement: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	m.ID = id

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *MeasurementRepository) GetLatestByRoom(ctx context.Context, roomID string) (*domain.Measurement, error) {
	m := &domain.Measurement{}
	var co2 sql.NullFloat64

	err := r.db.QueryRowContext(ctx, `
		SELECT id, room_id, timestamp, temperature, humidity, pressure, co2
		FROM measurements
		WHERE room_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, roomID).Scan(&m.ID, &m.RoomID, &m.Timestamp, &m.Temperature, &m.Humidity, &m.Pressure, &co2)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest measurement: %w", err)
	}

	if co2.Valid {
		m.CO2 = &co2.Float64
	}

	return m, nil
}

func (r *MeasurementRepository) GetByRoomAndTimeRange(ctx context.Context, roomID string, start, end time.Time) ([]*domain.Measurement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, room_id, timestamp, temperature, humidity, pressure, co2
		FROM measurements
		WHERE room_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, roomID, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to query measurements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var measurements []*domain.Measurement
	for rows.Next() {
		m := &domain.Measurement{}
		var co2 sql.NullFloat64

		err := rows.Scan(&m.ID, &m.RoomID, &m.Timestamp, &m.Temperature, &m.Humidity, &m.Pressure, &co2)
		if err != nil {
			return nil, fmt.Errorf("failed to scan measurement: %w", err)
		}

		if co2.Valid {
			m.CO2 = &co2.Float64
		}

		measurements = append(measurements, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating measurements: %w", err)
	}

	return measurements, nil
}

func (r *MeasurementRepository) GetMonthlyAverages(ctx context.Context) ([]*domain.MonthlyAverage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			room_id,
			strftime('%Y-%m', timestamp) AS month,
			AVG(temperature),
			AVG(humidity),
			AVG(pressure),
			AVG(co2),
			COUNT(*)
		FROM measurements
		GROUP BY room_id, month
		ORDER BY room_id ASC, month DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly averages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var averages []*domain.MonthlyAverage
	for rows.Next() {
		avg := &domain.MonthlyAverage{}
		var co2 sql.NullFloat64

		err := rows.Scan(&avg.RoomID, &avg.Month, &avg.AvgTemperature, &avg.AvgHumidity, &avg.AvgPressure, &co2, &avg.SampleCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monthly average: %w", err)
		}

		if co2.Valid {
			avg.AvgCO2 = &co2.Float64
		}

		averages = append(averages, avg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating monthly averages: %w", err)
	}

	return averages, nil
}

func (r *MeasurementRepository) GetAllRooms(ctx context.Context) ([]*domain.Room, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, first_seen_at, last_measurement_at
		FROM rooms
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query rooms: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		var lastMeasurement sql.NullTime

		err := rows.Scan(&room.ID, &room.Name, &room.FirstSeenAt, &lastMeasurement)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}

		if lastMeasurement.Valid {
			room.LastMeasurement = &lastMeasurement.Time
		}

		rooms = append(rooms, room)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rooms: %w", err)
	}

	return rooms, nil
}
