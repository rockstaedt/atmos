package http

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/domain"
)

const sessionCookieName = "atmos_session"

// MeasurementService defines the interface for measurement operations
type MeasurementService interface {
	RecordMeasurement(ctx context.Context, m *domain.Measurement) error
	GetLatestMeasurements(ctx context.Context) (map[string]*domain.Measurement, error)
	GetMeasurementHistory(ctx context.Context, roomID string, duration time.Duration) ([]*domain.Measurement, error)
	GetAllRooms(ctx context.Context) ([]*domain.Room, error)
	GetMonthlyAverages(ctx context.Context) (*application.MonthlyAveragesPageDTO, error)
}

// SessionRepository defines the interface for session persistence
type SessionRepository interface {
	Create(ctx context.Context, token string, expiresAt time.Time) error
	Exists(ctx context.Context, token string) (bool, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
}

type Server struct {
	addr         string
	service      MeasurementService
	sessionRepo  SessionRepository
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
	SessionRepo  SessionRepository
}

func NewServer(cfg Config, service MeasurementService) (*Server, error) {
	// Load timezone for display
	berlinTZ, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	// Load templates from embedded filesystem
	templates := make(map[string]*template.Template)
	funcMap := template.FuncMap{
		"derefFloat": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
		"formatTime": func(t time.Time, layout string) string {
			return t.In(berlinTZ).Format(layout)
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

	monthlyTmpl, err := parseTemplates("base.html",
		"templates/base.html",
		"templates/monthly-averages.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load monthly averages template: %w", err)
	}
	templates["monthly-averages.html"] = monthlyTmpl

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
		sessionRepo:  cfg.SessionRepo,
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
	s.mux.HandleFunc("GET /monthly", s.requireDashboardAuth(s.handleMonthlyAverages))
	s.mux.HandleFunc("GET /rooms/{roomID}", s.requireDashboardAuth(s.handleRoomDetail))
	s.mux.HandleFunc("GET /partials/dashboard", s.requireDashboardAuth(s.handleDashboardFragment))
	s.mux.HandleFunc("GET /partials/rooms/{roomID}", s.requireDashboardAuth(s.handleRoomDetailFragment))

	// Static files (embedded)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; "+
			"font-src 'self'; "+
			"connect-src 'self'; "+
			"frame-ancestors 'none'")

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
		IdleTimeout:  120 * time.Second,
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
// Also accepts valid dashboard session cookies for browser-based access
func (s *Server) requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check API key first
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.apiKey
		if auth == expected {
			next(w, r)
			return
		}

		// Fall back to dashboard session cookie
		if cookie, err := r.Cookie(sessionCookieName); err == nil && s.isValidSession(r.Context(), cookie.Value) {
			next(w, r)
			return
		}

		log.Printf("Failed API key authentication from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}
}

// requireDashboardAuth wraps a handler with dashboard session authentication
func (s *Server) requireDashboardAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.isValidSession(r.Context(), cookie.Value) {
			log.Printf("Failed dashboard authentication from %s", r.RemoteAddr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// isValidSession checks if the session token is valid in the database
func (s *Server) isValidSession(ctx context.Context, token string) bool {
	if s.sessionRepo == nil {
		// Fallback to legacy comparison if no session repo configured
		return subtle.ConstantTimeCompare([]byte(token), []byte(s.dashboardKey)) == 1
	}
	exists, err := s.sessionRepo.Exists(ctx, token)
	return err == nil && exists
}

// generateSessionToken creates a cryptographically secure random token
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isValidRoomID validates a room ID (max 64 chars, alphanumeric + hyphen/underscore)
func isValidRoomID(roomID string) bool {
	if len(roomID) == 0 || len(roomID) > 64 {
		return false
	}
	for _, c := range roomID {
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isAllowed := isLower || isUpper || isDigit || c == '-' || c == '_'
		if !isAllowed {
			return false
		}
	}
	return true
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if cookie, err := r.Cookie(sessionCookieName); err == nil && s.isValidSession(r.Context(), cookie.Value) {
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
		log.Printf("Failed login attempt from %s", r.RemoteAddr)
		data := map[string]interface{}{
			"Error": "Invalid access key",
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = s.templates["login.html"].ExecuteTemplate(w, "base", data)
		return
	}

	// Generate session token and store in database
	token, err := generateSessionToken()
	if err != nil {
		log.Printf("Failed to generate session token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(14 * 24 * time.Hour)
	if s.sessionRepo != nil {
		if err := s.sessionRepo.Create(r.Context(), token, expiresAt); err != nil {
			log.Printf("Failed to create session: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		// Legacy fallback: use dashboard key as token
		token = s.dashboardKey
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 14, // 14 days
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Delete session from database
	if cookie, err := r.Cookie(sessionCookieName); err == nil && s.sessionRepo != nil {
		_ = s.sessionRepo.Delete(r.Context(), cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete cookie
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
