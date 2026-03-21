package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rockstaedt/atmos/internal/application"
)

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
