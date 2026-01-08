package application

import (
	"time"
)

// MeasurementDTO represents the JSON structure from ESP32
type MeasurementDTO struct {
	RoomID      string   `json:"room_id"`
	Temperature float64  `json:"temperature"`
	Humidity    float64  `json:"humidity"`
	Pressure    float64  `json:"pressure"`
	CO2         *float64 `json:"co2,omitempty"`
	Timestamp   *string  `json:"timestamp,omitempty"` // Optional, use server time if missing
}

// ChartDataDTO represents data for Chart.js
type ChartDataDTO struct {
	Labels      []string  `json:"labels"`
	Temperature []float64 `json:"temperature"`
	Humidity    []float64 `json:"humidity"`
	Pressure    []float64 `json:"pressure"`
	CO2         []float64 `json:"co2,omitempty"`
}

// DashboardDTO represents the dashboard view model
type DashboardDTO struct {
	Rooms       []*RoomCardDTO `json:"rooms"`
	LastUpdated time.Time      `json:"last_updated"`
}

// RoomCardDTO represents a room card view model
type RoomCardDTO struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Temperature     float64   `json:"temperature"`
	Humidity        float64   `json:"humidity"`
	Pressure        float64   `json:"pressure"`
	CO2             *float64  `json:"co2,omitempty"`
	LastMeasurement time.Time `json:"last_measurement"`
}
