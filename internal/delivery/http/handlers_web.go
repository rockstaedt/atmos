package http

import (
	"net/http"
	"sort"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	measurements, err := s.service.GetLatestMeasurements(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["dashboard.html"]
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleRoomDetail(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomID")

	// Get latest measurement for room info
	latest, err := s.service.GetLatestMeasurements(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	measurement, ok := latest[roomID]
	if !ok {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	data := struct {
		RoomID      string
		RoomName    string
		Measurement *domain.Measurement
	}{
		RoomID:      roomID,
		RoomName:    roomID,
		Measurement: measurement,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := s.templates["room-detail.html"]
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
