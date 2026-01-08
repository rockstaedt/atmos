package domain

import (
	"testing"
	"time"
)

func TestMeasurement_Validate(t *testing.T) {
	now := time.Now().UTC()
	validCO2 := 400.0

	tests := []struct {
		name        string
		measurement Measurement
		wantErr     error
	}{
		{
			name: "valid measurement without CO2",
			measurement: Measurement{
				RoomID:      "living-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			wantErr: nil,
		},
		{
			name: "valid measurement with CO2",
			measurement: Measurement{
				RoomID:      "bedroom",
				Timestamp:   now.Add(-1 * time.Hour),
				Temperature: 20.0,
				Humidity:    50.0,
				Pressure:    1015.0,
				CO2:         &validCO2,
			},
			wantErr: nil,
		},
		{
			name: "empty room ID",
			measurement: Measurement{
				RoomID:      "",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidRoomID,
		},
		{
			name: "temperature too low",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: -51.0,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidTemperature,
		},
		{
			name: "temperature too high",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 101.0,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidTemperature,
		},
		{
			name: "humidity negative",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    -1.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidHumidity,
		},
		{
			name: "humidity over 100",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    101.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidHumidity,
		},
		{
			name: "pressure too low",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    299.0,
			},
			wantErr: ErrInvalidPressure,
		},
		{
			name: "pressure too high",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1101.0,
			},
			wantErr: ErrInvalidPressure,
		},
		{
			name: "negative CO2",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
				CO2:         func() *float64 { v := -100.0; return &v }(),
			},
			wantErr: ErrInvalidCO2,
		},
		{
			name: "timestamp in future",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(1 * time.Hour),
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			wantErr: ErrInvalidTimestamp,
		},
		{
			name: "boundary values - minimum temperature",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: -50.0,
				Humidity:    0.0,
				Pressure:    300.0,
			},
			wantErr: nil,
		},
		{
			name: "boundary values - maximum temperature",
			measurement: Measurement{
				RoomID:      "test-room",
				Timestamp:   now.Add(-1 * time.Minute),
				Temperature: 100.0,
				Humidity:    100.0,
				Pressure:    1100.0,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.measurement.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
