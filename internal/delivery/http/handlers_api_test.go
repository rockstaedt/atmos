package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

const testAPIKey = "test-api-key"

// Mock service
type mockMeasurementService struct {
	recordMeasurementFn     func(ctx context.Context, m *domain.Measurement) error
	getLatestMeasurementsFn func(ctx context.Context) (map[string]*domain.Measurement, error)
	getMeasurementHistoryFn func(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error)
	getAllRoomsFn           func(ctx context.Context) ([]*domain.Room, error)
}

func (m *mockMeasurementService) RecordMeasurement(ctx context.Context, measurement *domain.Measurement) error {
	if m.recordMeasurementFn != nil {
		return m.recordMeasurementFn(ctx, measurement)
	}
	return nil
}

func (m *mockMeasurementService) GetLatestMeasurements(ctx context.Context) (map[string]*domain.Measurement, error) {
	if m.getLatestMeasurementsFn != nil {
		return m.getLatestMeasurementsFn(ctx)
	}
	return nil, nil
}

func (m *mockMeasurementService) GetMeasurementHistory(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error) {
	if m.getMeasurementHistoryFn != nil {
		return m.getMeasurementHistoryFn(ctx, roomID, duration)
	}
	return nil, nil
}

func (m *mockMeasurementService) GetAllRooms(ctx context.Context) ([]*domain.Room, error) {
	if m.getAllRoomsFn != nil {
		return m.getAllRoomsFn(ctx)
	}
	return nil, nil
}

func TestHandlePostMeasurement(t *testing.T) {
	tests := []struct {
		name           string
		payload        interface{}
		recordFn       func(ctx context.Context, m *domain.Measurement) error
		expectedStatus int
	}{
		{
			name: "valid measurement",
			payload: application.MeasurementDTO{
				RoomID:      "test-room",
				Temperature: 22.5,
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			recordFn: func(ctx context.Context, m *domain.Measurement) error {
				m.ID = 1 // Simulate DB insert
				return nil
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "valid measurement with CO2",
			payload: application.MeasurementDTO{
				RoomID:      "living-room",
				Temperature: 23.0,
				Humidity:    50.0,
				Pressure:    1015.0,
				CO2:         func() *float64 { v := 400.0; return &v }(),
			},
			recordFn: func(ctx context.Context, m *domain.Measurement) error {
				m.ID = 2
				return nil
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid JSON",
			payload:        "invalid json",
			recordFn:       nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "validation error - invalid temperature",
			payload: application.MeasurementDTO{
				RoomID:      "test-room",
				Temperature: 150.0, // Invalid
				Humidity:    45.0,
				Pressure:    1013.25,
			},
			recordFn: func(ctx context.Context, m *domain.Measurement) error {
				return domain.ErrInvalidTemperature
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockMeasurementService{
				recordMeasurementFn: tt.recordFn,
			}

			server := &Server{
				service: mockService,
				mux:     http.NewServeMux(),
				apiKey:  testAPIKey,
			}
			server.routes("")

			var body []byte
			if str, ok := tt.payload.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.payload)
			}

			req := httptest.NewRequest("POST", "/api/measurements", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+testAPIKey)

			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response["status"] != "created" {
					t.Errorf("Expected status 'created', got %v", response["status"])
				}
			}
		})
	}
}

func TestHandleGetRooms(t *testing.T) {
	expectedRooms := []*domain.Room{
		{ID: "room1", Name: "Room 1"},
		{ID: "room2", Name: "Room 2"},
	}

	mockService := &mockMeasurementService{
		getAllRoomsFn: func(ctx context.Context) ([]*domain.Room, error) {
			return expectedRooms, nil
		},
	}

	server := &Server{
		service: mockService,
		mux:     http.NewServeMux(),
		apiKey:  testAPIKey,
	}
	server.routes("")

	req := httptest.NewRequest("GET", "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var rooms []*domain.Room
	if err := json.NewDecoder(w.Body).Decode(&rooms); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(rooms) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(rooms))
	}
}

func TestHandleGetMeasurements(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name           string
		roomID         string
		rangeParam     string
		mockHistory    []*domain.Measurement
		expectedStatus int
		expectedLength int
	}{
		{
			name:       "24h range",
			roomID:     "room1",
			rangeParam: "24h",
			mockHistory: []*domain.Measurement{
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
			},
			expectedStatus: http.StatusOK,
			expectedLength: 2,
		},
		{
			name:       "7d range",
			roomID:     "room1",
			rangeParam: "7d",
			mockHistory: []*domain.Measurement{
				{
					ID:          1,
					RoomID:      "room1",
					Timestamp:   now.Add(-48 * time.Hour),
					Temperature: 19.0,
					Humidity:    38.0,
					Pressure:    1012.0,
				},
			},
			expectedStatus: http.StatusOK,
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockMeasurementService{
				getMeasurementHistoryFn: func(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error) {
					return tt.mockHistory, nil
				},
			}

			server := &Server{
				service: mockService,
				mux:     http.NewServeMux(),
				apiKey:  testAPIKey,
			}
			server.routes("")

			url := "/api/rooms/" + tt.roomID + "/measurements"
			if tt.rangeParam != "" {
				url += "?range=" + tt.rangeParam
			}

			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var chartData application.ChartDataDTO
			if err := json.NewDecoder(w.Body).Decode(&chartData); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(chartData.Labels) != tt.expectedLength {
				t.Errorf("Expected %d data points, got %d", tt.expectedLength, len(chartData.Labels))
			}

			if len(chartData.Temperature) != tt.expectedLength {
				t.Errorf("Expected %d temperature points, got %d", tt.expectedLength, len(chartData.Temperature))
			}
		})
	}
}

func TestAPIKeyAuthentication(t *testing.T) {
	server := &Server{
		service: &mockMeasurementService{},
		mux:     http.NewServeMux(),
		apiKey:  testAPIKey,
	}
	server.routes("")

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid API key",
			authHeader:     "Bearer wrong-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing Bearer prefix",
			authHeader:     testAPIKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid API key",
			authHeader:     "Bearer " + testAPIKey,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/rooms", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	server := &Server{
		mux: http.NewServeMux(),
	}
	server.routes("")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}
}
