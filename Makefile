.PHONY: build test run docker-build docker-run docker-stop clean help

# Build binary
build:
	@echo "Building atmos..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -o bin/atmos ./cmd/server
	@echo "Build complete: bin/atmos"

# Run tests with coverage
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run locally (set API_KEY and DASHBOARD_KEY env vars or override defaults)
API_KEY ?= dev-api-key
DASHBOARD_KEY ?= dev-dashboard-key

run:
	@echo "Starting atmos server..."
	@DB_PATH=./atmos.db PORT=8080 API_KEY=$(API_KEY) DASHBOARD_KEY=$(DASHBOARD_KEY) go run ./cmd/server

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t atmos:latest .
	@echo "Docker image built: atmos:latest"

# Run Docker container
docker-run:
	@echo "Starting Docker container..."
	@docker-compose up -d
	@echo "Container started. Access at http://localhost:8080"

# Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	@docker-compose down
	@echo "Container stopped"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/ coverage.out coverage.html *.db *.db-shm *.db-wal
	@echo "Clean complete"

# Show help
help:
	@echo "Atmos - Climate Monitoring Application"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build the application binary"
	@echo "  test         - Run tests with coverage report"
	@echo "  run          - Run the application locally"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Start Docker container"
	@echo "  docker-stop  - Stop Docker container"
	@echo "  clean        - Clean build artifacts"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables for 'run':"
	@echo "  API_KEY       - API key for sensor authentication (default: dev-api-key)"
	@echo "  DASHBOARD_KEY - Key for dashboard access (default: dev-dashboard-key)"
	@echo ""
	@echo "Example: make run API_KEY=my-secret DASHBOARD_KEY=my-dashboard-key"
