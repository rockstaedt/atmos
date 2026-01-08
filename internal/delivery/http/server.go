package http

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

// MeasurementService defines the interface for measurement operations
type MeasurementService interface {
	RecordMeasurement(ctx context.Context, m *domain.Measurement) error
	GetLatestMeasurements(ctx context.Context) (map[string]*domain.Measurement, error)
	GetMeasurementHistory(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error)
	GetAllRooms(ctx context.Context) ([]*domain.Room, error)
}

type Server struct {
	addr      string
	service   MeasurementService
	templates map[string]*template.Template
	mux       *http.ServeMux
}

type Config struct {
	Port         int
	TemplatesDir string
	StaticDir    string
}

func NewServer(cfg Config, service MeasurementService) (*Server, error) {
	// Load templates individually to avoid conflicts
	templates := make(map[string]*template.Template)
	funcMap := template.FuncMap{
		"derefFloat": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
	}

	basePath := fmt.Sprintf("%s/base.html", cfg.TemplatesDir)
	dashboardTmpl, err := template.New("base.html").Funcs(funcMap).ParseFiles(
		basePath,
		fmt.Sprintf("%s/dashboard.html", cfg.TemplatesDir),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load dashboard template: %w", err)
	}
	templates["dashboard.html"] = dashboardTmpl

	roomDetailTmpl, err := template.New("base.html").Funcs(funcMap).ParseFiles(
		basePath,
		fmt.Sprintf("%s/room-detail.html", cfg.TemplatesDir),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load room-detail template: %w", err)
	}
	templates["room-detail.html"] = roomDetailTmpl

	s := &Server{
		addr:      fmt.Sprintf(":%d", cfg.Port),
		service:   service,
		templates: templates,
		mux:       http.NewServeMux(),
	}

	s.routes(cfg.StaticDir)

	return s, nil
}

func (s *Server) routes(staticDir string) {
	// API endpoints
	s.mux.HandleFunc("POST /api/measurements", s.handlePostMeasurement)
	s.mux.HandleFunc("GET /api/rooms", s.handleGetRooms)
	s.mux.HandleFunc("GET /api/rooms/{roomID}/measurements", s.handleGetMeasurements)

	// Web UI endpoints
	s.mux.HandleFunc("GET /", s.handleDashboard)
	s.mux.HandleFunc("GET /rooms/{roomID}", s.handleRoomDetail)

	// Static files
	fs := http.FileServer(http.Dir(staticDir))
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Logging middleware
	start := time.Now()
	s.mux.ServeHTTP(w, r)
	duration := time.Since(start)
	log.Printf("%s %s - %v\n", r.Method, r.URL.Path, duration)
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting server on %s\n", s.addr)
	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
