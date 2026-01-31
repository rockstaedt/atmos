package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
type mockSessionRepository struct {
	sessions map[string]*domain.Session
}

func newMockSessionRepository() *mockSessionRepository {
	return &mockSessionRepository{
		sessions: make(map[string]*domain.Session),
	}
}

func (m *mockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	m.sessions[session.Token] = session
	return nil
}

func (m *mockSessionRepository) FindByToken(ctx context.Context, token string) (*domain.Session, error) {
	session, ok := m.sessions[token]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockSessionRepository) UpdateLastAccessed(ctx context.Context, id string, accessedAt time.Time) error {
	for _, s := range m.sessions {
		if s.ID == id {
			s.LastAccessedAt = accessedAt
			return nil
		}
	}
	return domain.ErrSessionNotFound
}

func (m *mockSessionRepository) Delete(ctx context.Context, id string) error {
	for token, s := range m.sessions {
		if s.ID == id {
			delete(m.sessions, token)
			return nil
		}
	}
	return nil
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now().UTC()
	for token, s := range m.sessions {
		if s.ExpiresAt.Before(now) {
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

func TestDashboardAuthentication(t *testing.T) {
	server := &Server{
		service:      &mockMeasurementService{},
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
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testDashboardKey})
		w := httptest.NewRecorder()

		testHandler(w, req)

		if !authPassed {
			t.Error("Auth middleware should have passed with valid session")
		}
		if w.Code == http.StatusSeeOther {
			t.Error("Should not redirect with valid session")
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	server := &Server{
		service:      &mockMeasurementService{},
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-XSS-Protection":       "0",
	}

	for header, expected := range expectedHeaders {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("Expected %s header to be %q, got %q", header, expected, got)
		}
	}

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Expected Content-Security-Policy header to be set")
	}
	if !containsAll(csp, "default-src", "'self'", "script-src", "style-src") {
		t.Errorf("Content-Security-Policy missing expected directives: %s", csp)
	}
}

func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestOpaqueSessionAuthentication(t *testing.T) {
	sessionRepo := newMockSessionRepository()

	server := &Server{
		service:      &mockMeasurementService{},
		sessions:     sessionRepo,
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	t.Run("valid opaque session token grants access", func(t *testing.T) {
		// Create a valid session
		session, _ := domain.NewSession(24 * time.Hour)
		sessionRepo.sessions[session.Token] = session

		req := httptest.NewRequest("GET", "/api/rooms", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.Token})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d with valid session token, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("expired session token is rejected", func(t *testing.T) {
		// Create an expired session
		expiredSession := &domain.Session{
			ID:             "expired-id",
			Token:          "expired-token",
			CreatedAt:      time.Now().UTC().Add(-48 * time.Hour),
			ExpiresAt:      time.Now().UTC().Add(-24 * time.Hour),
			LastAccessedAt: time.Now().UTC().Add(-48 * time.Hour),
		}
		sessionRepo.sessions[expiredSession.Token] = expiredSession

		req := httptest.NewRequest("GET", "/api/rooms", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: expiredSession.Token})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d with expired session, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("invalid session token is rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/rooms", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-token"})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d with invalid token, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("legacy mode works when sessions is nil", func(t *testing.T) {
		legacyServer := &Server{
			service:      &mockMeasurementService{},
			sessions:     nil, // No session repository
			mux:          http.NewServeMux(),
			apiKey:       testAPIKey,
			dashboardKey: testDashboardKey,
		}
		legacyServer.routes(emptyFS)

		req := httptest.NewRequest("GET", "/api/rooms", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testDashboardKey})
		w := httptest.NewRecorder()

		legacyServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d with legacy auth, got %d", http.StatusOK, w.Code)
		}
	})
}

func TestCSRFValidation(t *testing.T) {
	server := &Server{
		service:      &mockMeasurementService{},
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	t.Run("POST without CSRF token is rejected", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d without CSRF token, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("POST with matching CSRF token is accepted", func(t *testing.T) {
		csrfToken := "test-csrf-token"

		formData := "csrf_token=" + csrfToken
		req := httptest.NewRequest("POST", "/logout", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		// Logout should redirect, not give forbidden
		if w.Code == http.StatusForbidden {
			t.Error("CSRF validation should have passed with matching token")
		}
	})

	t.Run("POST with mismatched CSRF token is rejected", func(t *testing.T) {
		formData := "csrf_token=wrong-token"
		req := httptest.NewRequest("POST", "/logout", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "correct-token"})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d with mismatched CSRF token, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("POST with CSRF header is accepted", func(t *testing.T) {
		csrfToken := "test-csrf-token"

		req := httptest.NewRequest("POST", "/logout", nil)
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		// Logout should redirect, not give forbidden
		if w.Code == http.StatusForbidden {
			t.Error("CSRF validation should have passed with matching header token")
		}
	})
}
