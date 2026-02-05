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

// RoomStatsDTO represents aggregate statistics for a room
type RoomStatsDTO struct {
	AvgTemperature   float64  `json:"avg_temperature"`
	AvgHumidity      float64  `json:"avg_humidity"`
	AvgCO2           *float64 `json:"avg_co2,omitempty"`
	MinTemperature   float64  `json:"min_temperature"`
	MaxTemperature   float64  `json:"max_temperature"`
	MinHumidity      float64  `json:"min_humidity"`
	MaxHumidity      float64  `json:"max_humidity"`
	TemperatureTrend string   `json:"temperature_trend"`
	HumidityTrend    string   `json:"humidity_trend"`
	SampleCount      int      `json:"sample_count"`
}

// RoomCardDTO represents a room card view model
type RoomCardDTO struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Temperature     float64       `json:"temperature"`
	Humidity        float64       `json:"humidity"`
	Pressure        float64       `json:"pressure"`
	CO2             *float64      `json:"co2,omitempty"`
	LastMeasurement time.Time     `json:"last_measurement"`
	SensorHealth    string        `json:"sensor_health"`
	Stats           *RoomStatsDTO `json:"stats,omitempty"`
}
