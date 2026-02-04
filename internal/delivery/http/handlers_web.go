package http

import (
	"context"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

// pageData wraps any view data with common page fields
type pageData struct {
	Data    interface{}
	Version string
}

type roomDetailView struct {
	RoomID      string
	RoomName    string
	Measurement *domain.Measurement
}

func (s *Server) buildDashboardDTO(ctx context.Context) (*application.DashboardDTO, error) {
	measurements, err := s.service.GetLatestMeasurements(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to DTOs
	var roomCards []*application.RoomCardDTO
	for roomID, m := range measurements {
		roomCards = append(roomCards, &application.RoomCardDTO{
			ID:              roomID,
			Name:            roomID, // Can be enhanced later
			Temperature:     m.Temperature,
			Humidity:        m.Humidity,
			Pressure:        m.Pressure,
			CO2:             m.CO2,
			LastMeasurement: m.Timestamp,
		})
	}

	// Sort by room ID for consistent display
	sort.Slice(roomCards, func(i, j int) bool {
		return roomCards[i].ID < roomCards[j].ID
	})

	data := &application.DashboardDTO{
		Rooms:       roomCards,
		LastUpdated: time.Now().UTC(),
	}

	return data, nil
}

func (s *Server) buildRoomDetailView(ctx context.Context, roomID string) (*roomDetailView, error) {
	latest, err := s.service.GetLatestMeasurements(ctx)
	if err != nil {
		return nil, err
	}

	measurement, ok := latest[roomID]
	if !ok {
		return nil, nil
	}

	return &roomDetailView{
		RoomID:      roomID,
		RoomName:    roomID,
		Measurement: measurement,
	}, nil
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildDashboardDTO(r.Context())
	if err != nil {
		log.Printf("Failed to build dashboard data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["dashboard.html"]
	if err := tmpl.ExecuteTemplate(w, "base", pageData{Data: data, Version: s.version}); err != nil {
		log.Printf("Failed to render dashboard template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboardFragment(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildDashboardDTO(r.Context())
	if err != nil {
		log.Printf("Failed to build dashboard fragment data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["dashboard-fragment.html"]
	if err := tmpl.ExecuteTemplate(w, "dashboard-fragment", data); err != nil {
		log.Printf("Failed to render dashboard fragment template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRoomDetail(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomID")

	data, err := s.buildRoomDetailView(r.Context(), roomID)
	if err != nil {
		log.Printf("Failed to build room detail view for %s: %v", roomID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["room-detail.html"]
	if err := tmpl.ExecuteTemplate(w, "base", pageData{Data: data, Version: s.version}); err != nil {
		log.Printf("Failed to render room detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRoomDetailFragment(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomID")

	data, err := s.buildRoomDetailView(r.Context(), roomID)
	if err != nil {
		log.Printf("Failed to build room detail fragment for %s: %v", roomID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["room-detail-fragment.html"]
	if err := tmpl.ExecuteTemplate(w, "room-detail-fragment", data); err != nil {
		log.Printf("Failed to render room detail fragment template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
