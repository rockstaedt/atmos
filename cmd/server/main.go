package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rockstaedt/atmos/internal/application"
	"github.com/rockstaedt/atmos/internal/delivery/http"
	"github.com/rockstaedt/atmos/internal/infrastructure/sqlite"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Initialize database
	dbPath := getEnv("DB_PATH", "./atmos.db")
	db, err := sqlite.NewConnection(sqlite.Config{Path: dbPath})
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = db.Close() }()

	fmt.Printf("Database initialized at: %s\n", dbPath)

	// Initialize repository
	measurementRepo := sqlite.NewMeasurementRepository(db)

	// Initialize service
	measurementService := application.NewMeasurementService(measurementRepo)

	// Initialize HTTP server
	port := getEnvInt("PORT", 8080)
	templatesDir := getEnv("TEMPLATES_DIR", "./web/templates")
	staticDir := getEnv("STATIC_DIR", "./web/static")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return fmt.Errorf("API_KEY environment variable is required")
	}
	dashboardKey := os.Getenv("DASHBOARD_KEY")
	if dashboardKey == "" {
		return fmt.Errorf("DASHBOARD_KEY environment variable is required")
	}

	server, err := http.NewServer(http.Config{
		Port:         port,
		TemplatesDir: templatesDir,
		StaticDir:    staticDir,
		APIKey:       apiKey,
		DashboardKey: dashboardKey,
		Version:      version,
	}, measurementService)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v. Shutting down gracefully...\n", sig)
		return nil
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
