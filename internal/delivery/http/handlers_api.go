package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

func (s *Server) handlePostMeasurement(w http.ResponseWriter, r *http.Request) {
	var dto application.MeasurementDTO

	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Use provided timestamp or server time
	timestamp := time.Now().UTC()
	if dto.Timestamp != nil {
		parsed, err := time.Parse(time.RFC3339, *dto.Timestamp)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timestamp format: %v", err), http.StatusBadRequest)
			return
		}
		timestamp = parsed.UTC()
	}

	// Convert DTO to domain entity
	measurement := &domain.Measurement{
		RoomID:      dto.RoomID,
		Timestamp:   timestamp,
		Temperature: dto.Temperature,
		Humidity:    dto.Humidity,
		Pressure:    dto.Pressure,
		CO2:         dto.CO2,
	}

	// Record measurement
	if err := s.service.RecordMeasurement(r.Context(), measurement); err != nil {
		http.Error(w, fmt.Sprintf("Failed to record measurement: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      measurement.ID,
		"room_id": measurement.RoomID,
		"status":  "created",
	})
}

func (s *Server) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.service.GetAllRooms(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rooms)
}

func (s *Server) handleGetMeasurements(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomID")

	// Parse time range from query params (default to 24h)
	rangeParam := r.URL.Query().Get("range")
	duration := 24 * time.Hour

	switch rangeParam {
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	}

	measurements, err := s.service.GetMeasurementHistory(r.Context(), roomID, duration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to chart-friendly format
	chartData := &application.ChartDataDTO{
		Labels:      make([]string, len(measurements)),
		Temperature: make([]float64, len(measurements)),
		Humidity:    make([]float64, len(measurements)),
		Pressure:    make([]float64, len(measurements)),
	}

	hasCO2 := false
	for i, m := range measurements {
		chartData.Labels[i] = m.Timestamp.Format(time.RFC3339)
		chartData.Temperature[i] = m.Temperature
		chartData.Humidity[i] = m.Humidity
		chartData.Pressure[i] = m.Pressure

		if m.CO2 != nil {
			hasCO2 = true
			if chartData.CO2 == nil {
				chartData.CO2 = make([]float64, len(measurements))
			}
			chartData.CO2[i] = *m.CO2
		}
	}

	// Only include CO2 in response if at least one measurement has it
	if !hasCO2 {
		chartData.CO2 = nil
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chartData)
}
