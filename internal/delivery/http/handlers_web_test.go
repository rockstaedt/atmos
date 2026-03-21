package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/web"
)

func TestNewServer_ParsesAllTemplates(t *testing.T) {
	_, err := NewServer(Config{
		Assets:       web.Files,
		APIKey:       testAPIKey,
		DashboardKey: testDashboardKey,
	}, &mockMeasurementService{})

	if err != nil {
		t.Fatalf("NewServer() failed to parse templates: %v", err)
	}
}

func TestHandleMonthlyAverages_Unauthenticated(t *testing.T) {
	server := &Server{
		service:      &mockMeasurementService{},
		sessionRepo:  newMockSessionRepository(),
		mux:          http.NewServeMux(),
		apiKey:       testAPIKey,
		dashboardKey: testDashboardKey,
	}
	server.routes(emptyFS)

	req := httptest.NewRequest("GET", "/monthly", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect %d, got %d", http.StatusSeeOther, w.Code)
	}
	if location := w.Header().Get("Location"); location != "/login" {
		t.Errorf("Expected redirect to /login, got %s", location)
	}
}

func TestHandleMonthlyAverages_ServiceError(t *testing.T) {
	server := &Server{
		service: &mockMeasurementService{
			getMonthlyAveragesFn: func(ctx context.Context) (*application.MonthlyAveragesPageDTO, error) {
				return nil, errors.New("db error")
			},
		},
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest("GET", "/monthly", nil)
	w := httptest.NewRecorder()

	server.handleMonthlyAverages(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
