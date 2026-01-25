package http

import (
	"context"
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

const sessionCookieName = "atmos_session"

// MeasurementService defines the interface for measurement operations
type MeasurementService interface {
	RecordMeasurement(ctx context.Context, m *domain.Measurement) error
	GetLatestMeasurements(ctx context.Context) (map[string]*domain.Measurement, error)
	GetMeasurementHistory(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error)
	GetAllRooms(ctx context.Context) ([]*domain.Room, error)
}

type Server struct {
	addr         string
	service      MeasurementService
	templates    map[string]*template.Template
	mux          *http.ServeMux
	apiKey       string
	dashboardKey string
	version      string
}

type Config struct {
	Port         int
	Assets       embed.FS
	APIKey       string
	DashboardKey string
	Version      string
}

func NewServer(cfg Config, service MeasurementService) (*Server, error) {
	// Load templates from embedded filesystem
	templates := make(map[string]*template.Template)
	funcMap := template.FuncMap{
		"derefFloat": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
	}

	// Helper to parse templates from embed.FS
	parseTemplates := func(name string, files ...string) (*template.Template, error) {
		return template.New(name).Funcs(funcMap).ParseFS(cfg.Assets, files...)
	}

	dashboardTmpl, err := parseTemplates("base.html",
		"templates/base.html",
		"templates/dashboard.html",
		"templates/partials/dashboard-fragment.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load dashboard template: %w", err)
	}
	templates["dashboard.html"] = dashboardTmpl

	roomDetailTmpl, err := parseTemplates("base.html",
		"templates/base.html",
		"templates/room-detail.html",
		"templates/partials/room-detail-fragment.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load room-detail template: %w", err)
	}
	templates["room-detail.html"] = roomDetailTmpl

	dashboardFragmentTmpl, err := parseTemplates("dashboard-fragment.html",
		"templates/partials/dashboard-fragment.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load dashboard fragment template: %w", err)
	}
	templates["dashboard-fragment.html"] = dashboardFragmentTmpl

	roomDetailFragmentTmpl, err := parseTemplates("room-detail-fragment.html",
		"templates/partials/room-detail-fragment.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load room detail fragment template: %w", err)
	}
	templates["room-detail-fragment.html"] = roomDetailFragmentTmpl

	loginTmpl, err := parseTemplates("base.html",
		"templates/base.html",
		"templates/login.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load login template: %w", err)
	}
	templates["login.html"] = loginTmpl

	// Create sub-filesystem for static files
	staticFS, err := fs.Sub(cfg.Assets, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to create static filesystem: %w", err)
	}

	s := &Server{
		addr:         fmt.Sprintf(":%d", cfg.Port),
		service:      service,
		templates:    templates,
		mux:          http.NewServeMux(),
		apiKey:       cfg.APIKey,
		dashboardKey: cfg.DashboardKey,
		version:      cfg.Version,
	}

	s.routes(staticFS)

	return s, nil
}

func (s *Server) routes(staticFS fs.FS) {
	// API endpoints (protected by API key)
	s.mux.HandleFunc("POST /api/measurements", s.requireAPIKey(s.handlePostMeasurement))
	s.mux.HandleFunc("GET /api/rooms", s.requireAPIKey(s.handleGetRooms))
	s.mux.HandleFunc("GET /api/rooms/{roomID}/measurements", s.requireAPIKey(s.handleGetMeasurements))

	// Authentication endpoints
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)

	// Web UI endpoints (protected by dashboard auth)
	s.mux.HandleFunc("GET /", s.requireDashboardAuth(s.handleDashboard))
	s.mux.HandleFunc("GET /rooms/{roomID}", s.requireDashboardAuth(s.handleRoomDetail))
	s.mux.HandleFunc("GET /partials/dashboard", s.requireDashboardAuth(s.handleDashboardFragment))
	s.mux.HandleFunc("GET /partials/rooms/{roomID}", s.requireDashboardAuth(s.handleRoomDetailFragment))
	s.mux.HandleFunc("GET /dashboard/api/rooms/{roomID}/measurements", s.requireDashboardAuth(s.handleGetMeasurements))

	// Static files (embedded)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

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
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// requireAPIKey wraps a handler with API key authentication
func (s *Server) requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.apiKey

		if auth != expected {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		next(w, r)
	}
}

// requireDashboardAuth wraps a handler with dashboard session authentication
func (s *Server) requireDashboardAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.isValidSession(cookie.Value) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// isValidSession checks if the session cookie value is valid
func (s *Server) isValidSession(sessionValue string) bool {
	return subtle.ConstantTimeCompare([]byte(sessionValue), []byte(s.dashboardKey)) == 1
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if cookie, err := r.Cookie(sessionCookieName); err == nil && s.isValidSession(cookie.Value) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"Error": nil,
	}
	_ = s.templates["login.html"].ExecuteTemplate(w, "base", data)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	key := r.FormValue("key")
	if subtle.ConstantTimeCompare([]byte(key), []byte(s.dashboardKey)) != 1 {
		data := map[string]interface{}{
			"Error": "Invalid access key",
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = s.templates["login.html"].ExecuteTemplate(w, "base", data)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.dashboardKey,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30, // 30 days
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1, // Delete cookie
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
