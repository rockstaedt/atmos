package domain

import (
	"errors"
	"time"
)

// Measurement represents a single climate measurement from a sensor
type Measurement struct {
	ID          int64
	RoomID      string
	Timestamp   time.Time
	Temperature float64  // Celsius
	Humidity    float64  // Percentage (0-100)
	Pressure    float64  // hPa
	CO2         *float64 // ppm (optional)
}

// Validation errors
var (
	ErrInvalidRoomID      = errors.New("room ID cannot be empty")
	ErrInvalidTemperature = errors.New("temperature out of valid range (-50 to 100°C)")
	ErrInvalidHumidity    = errors.New("humidity must be between 0 and 100%")
	ErrInvalidPressure    = errors.New("pressure out of valid range (300-1100 hPa)")
	ErrInvalidCO2         = errors.New("CO2 must be positive")
	ErrInvalidTimestamp   = errors.New("timestamp cannot be in the future")
)

// Validate ensures the measurement is valid according to business rules
func (m *Measurement) Validate() error {
	if m.RoomID == "" {
		return ErrInvalidRoomID
	}

	if m.Temperature < -50 || m.Temperature > 100 {
		return ErrInvalidTemperature
	}

	if m.Humidity < 0 || m.Humidity > 100 {
		return ErrInvalidHumidity
	}

	if m.Pressure < 300 || m.Pressure > 1100 {
		return ErrInvalidPressure
	}

	if m.CO2 != nil && *m.CO2 < 0 {
		return ErrInvalidCO2
	}

	if m.Timestamp.After(time.Now().UTC()) {
		return ErrInvalidTimestamp
	}

	return nil
}

// Room represents a registered room/sensor location
type Room struct {
	ID              string
	Name            string
	FirstSeenAt     time.Time
	LastMeasurement *time.Time
}
