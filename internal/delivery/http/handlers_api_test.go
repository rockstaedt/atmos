package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

// emptyFS is an empty filesystem for tests
var emptyFS = fstest.MapFS{}

const testAPIKey = "test-api-key"
const testDashboardKey = "test-dashboard-key"

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

// Mock session repository
type mockSession struct {
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type mockSessionRepository struct {
	sessions map[string]*mockSession
}

func newMockSessionRepository() *mockSessionRepository {
	return &mockSessionRepository{
		sessions: make(map[string]*mockSession),
	}
}

func (m *mockSessionRepository) Create(ctx context.Context, token string, expiresAt time.Time) error {
	m.sessions[token] = &mockSession{
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *mockSessionRepository) Exists(ctx context.Context, token string) (bool, error) {
	session, ok := m.sessions[token]
	if !ok {
		return false, nil
	}
	return time.Now().Before(session.ExpiresAt), nil
}

func (m *mockSessionRepository) Delete(ctx context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now()
	for token, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
	return nil
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
		{
			name:           "request body too large",
			payload:        string(make([]byte, 2<<20)), // 2MB
			recordFn:       nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockMeasurementService{
				recordMeasurementFn: tt.recordFn,
			}

			server := &Server{
				service:      mockService,
				mux:          http.NewServeMux(),
				apiKey:       testAPIKey,
				dashboardKey: testDashboardKey,
			}
			server.routes(emptyFS)

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
		service:      mockService,
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

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
				service:      mockService,
				mux:          http.NewServeMux(),
				apiKey:       testAPIKey,
				dashboardKey: testDashboardKey,
			}
			server.routes(emptyFS)

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
		service:      &mockMeasurementService{},
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

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
		mux:          http.NewServeMux(),
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

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

func TestSecurityHeaders(t *testing.T) {
	server := &Server{
		mux:          http.NewServeMux(),
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := w.Header().Get(tt.header); got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.header, got, tt.expected)
			}
		})
	}

	// Check CSP contains required directives
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is missing")
	}
	if !contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src directive: %s", csp)
	}
	if !contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors directive: %s", csp)
	}

	// Check Permissions-Policy
	pp := w.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Error("Permissions-Policy header is missing")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDashboardAuthentication(t *testing.T) {
	sessionRepo := newMockSessionRepository()
	validToken := "valid-session-token"
	_ = sessionRepo.Create(context.Background(), validToken, time.Now().Add(24*time.Hour))

	server := &Server{
		service:      &mockMeasurementService{},
		sessionRepo:  sessionRepo,
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	t.Run("dashboard without session redirects to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if location := w.Header().Get("Location"); location != "/login" {
			t.Errorf("Expected redirect to /login, got %s", location)
		}
	})

	t.Run("dashboard with invalid session redirects to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "wrong-key"})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if location := w.Header().Get("Location"); location != "/login" {
			t.Errorf("Expected redirect to /login, got %s", location)
		}
	})

	t.Run("dashboard with valid session passes auth", func(t *testing.T) {
		// Test the middleware directly instead of full handler (which needs templates)
		authPassed := false
		testHandler := server.requireDashboardAuth(func(w http.ResponseWriter, r *http.Request) {
			authPassed = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: validToken})
		w := httptest.NewRecorder()

		testHandler(w, req)

		if !authPassed {
			t.Error("Auth middleware should have passed with valid session")
		}
		if w.Code == http.StatusSeeOther {
			t.Error("Should not redirect with valid session")
		}
	})

	t.Run("dashboard with expired session redirects to login", func(t *testing.T) {
		expiredToken := "expired-session-token"
		_ = sessionRepo.Create(context.Background(), expiredToken, time.Now().Add(-1*time.Hour))

		authPassed := false
		testHandler := server.requireDashboardAuth(func(w http.ResponseWriter, r *http.Request) {
			authPassed = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: expiredToken})
		w := httptest.NewRecorder()

		testHandler(w, req)

		if authPassed {
			t.Error("Auth middleware should not pass with expired session")
		}
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status %d, got %d", http.StatusSeeOther, w.Code)
		}
	})
}

func TestRoomIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		roomID  string
		isValid bool
	}{
		{"valid simple", "living-room", true},
		{"valid with underscore", "room_1", true},
		{"valid alphanumeric", "Room123", true},
		{"valid mixed", "My-Room_42", true},
		{"valid max length", string(make([]byte, 64)), false}, // 64 null bytes are invalid chars
		{"empty", "", false},
		{"too long", string(make([]byte, 65)), false},
		{"invalid space", "living room", false},
		{"invalid dot", "room.1", false},
		{"invalid slash", "room/1", false},
		{"invalid special chars", "room<>", false},
		{"invalid sql injection attempt", "room'; DROP TABLE--", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the max length test, use valid chars
			roomID := tt.roomID
			if tt.name == "valid max length" {
				roomID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64 'a's
			}
			if tt.name == "too long" {
				roomID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 65 'a's
			}

			got := isValidRoomID(roomID)

			// Adjust expected for our corrected test values
			expected := tt.isValid
			if tt.name == "valid max length" {
				expected = true
			}

			if got != expected {
				t.Errorf("isValidRoomID(%q) = %v, want %v", roomID, got, expected)
			}
		})
	}
}

func TestRoomIDValidationInAPI(t *testing.T) {
	server := &Server{
		service:      &mockMeasurementService{},
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	t.Run("invalid roomID in measurements endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/rooms/room<script>/measurements", nil)
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("valid roomID in measurements endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/rooms/living-room/measurements", nil)
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}
