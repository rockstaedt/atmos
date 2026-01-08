# Atmos - Climate Monitoring System

A local-first climate monitoring web application that collects time-series sensor data from ESP32 devices and visualizes it via a mobile-first web UI.

## Features

- **Auto-Registration**: ESP32 sensors automatically register when sending first measurement
- **Real-time Dashboard**: View current conditions across all rooms
- **Time-Series Charts**: Interactive Chart.js visualizations (24h, 7d, 30d ranges)
- **Mobile-First Design**: Responsive UI built with Tailwind CSS
- **Hexagonal Architecture**: Clean separation of domain, application, infrastructure, and delivery layers
- **SQLite Backend**: Efficient time-series storage with WAL mode
- **Docker Support**: Single container deployment with persistent volumes

## Quick Start

### Local Development

```bash
# Build the application
make build

# Run tests
make test

# Start the server
make run
```

The server will start on http://localhost:8080

### Docker Deployment

```bash
# Build Docker image
make docker-build

# Run container
make docker-run
```

Access the application at http://localhost:8080

## ESP32 Integration

Configure your ESP32 to POST measurements to `/api/measurements`:

```cpp
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>

const char* serverUrl = "http://YOUR_SERVER_IP:8080/api/measurements";
const char* roomId = "living-room";

void loop() {
  HTTPClient http;
  http.begin(serverUrl);
  http.addHeader("Content-Type", "application/json");

  StaticJsonDocument<256> doc;
  doc["room_id"] = roomId;
  doc["temperature"] = 22.5;
  doc["humidity"] = 45.0;
  doc["pressure"] = 1013.25;
  doc["co2"] = 400; // Optional

  String json;
  serializeJson(doc, json);

  int code = http.POST(json);
  Serial.printf("HTTP: %d\n", code);

  http.end();
  delay(60000);  // Send every 60 seconds
}
```

### JSON Payload Format

```json
{
  "room_id": "living-room",
  "temperature": 22.5,
  "humidity": 45.0,
  "pressure": 1013.25,
  "co2": 400,
  "timestamp": "2026-01-08T15:30:00Z"
}
```

Fields:
- `room_id` (required): Unique room identifier
- `temperature` (required): Temperature in Celsius (-50 to 100°C)
- `humidity` (required): Humidity percentage (0-100%)
- `pressure` (required): Atmospheric pressure in hPa (300-1100)
- `co2` (optional): CO₂ concentration in ppm
- `timestamp` (optional): UTC timestamp (ISO 8601). If omitted, server time is used.

## API Endpoints

### POST /api/measurements
Record a new measurement from an ESP32 sensor.

**Response:**
```json
{
  "id": 1,
  "room_id": "living-room",
  "status": "created"
}
```

### GET /api/rooms
List all registered rooms.

### GET /api/rooms/{roomID}/measurements?range=24h
Get measurement history for a room.

Query parameters:
- `range`: `24h`, `7d`, or `30d` (default: `24h`)

## Environment Variables

- `PORT` - HTTP port (default: `8080`)
- `DB_PATH` - SQLite database path (default: `./atmos.db`)
- `TEMPLATES_DIR` - HTML templates directory (default: `./web/templates`)
- `STATIC_DIR` - Static files directory (default: `./web/static`)

## Architecture

```
atmos/
├── cmd/server/              # Application entry point
├── internal/
│   ├── domain/              # Pure business logic
│   ├── application/         # Use cases
│   ├── infrastructure/      # SQLite adapter
│   └── delivery/            # HTTP handlers
├── web/
│   ├── static/              # CSS, JS
│   └── templates/           # HTML templates
├── Dockerfile
└── docker-compose.yml
```

### Hexagonal Architecture

The application follows hexagonal architecture principles:
- **Domain Layer**: Pure Go, no external dependencies
- **Application Layer**: Use cases and DTOs
- **Infrastructure Layer**: SQLite repository implementation
- **Delivery Layer**: HTTP handlers and templates

## Testing

Run tests with coverage:

```bash
make test
```

Coverage targets:
- Domain layer: 100%
- Application layer: 90%
- Infrastructure layer: 85%
- Delivery layer: 75%

## Performance

- Memory usage: <100MB
- Startup time: <1s
- API response: <50ms for POST /api/measurements
- Dashboard: <100ms to render
- Charts: <200ms for 24h data

## Database Schema

### rooms
```sql
CREATE TABLE rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    first_seen_at DATETIME NOT NULL,
    last_measurement_at DATETIME
);
```

### measurements
```sql
CREATE TABLE measurements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    temperature REAL NOT NULL,
    humidity REAL NOT NULL,
    pressure REAL NOT NULL,
    co2 REAL,
    FOREIGN KEY (room_id) REFERENCES rooms(id)
);

CREATE INDEX idx_measurements_room_timestamp ON measurements(room_id, timestamp DESC);
```

## Technology Stack

- **Backend**: Go 1.21+
- **Database**: SQLite with WAL mode
- **Frontend**: HTMX, Tailwind CSS, Chart.js
- **Containerization**: Docker, Docker Compose

## Development

### Project Structure

```
internal/
├── domain/                   # Core entities & interfaces
│   ├── measurement.go        # Measurement & Room entities
│   ├── repository.go         # Repository interfaces
│   └── measurement_test.go   # Domain tests
├── application/              # Use cases
│   ├── measurement_service.go
│   ├── dto.go
│   └── measurement_service_test.go
├── infrastructure/           # External adapters
│   └── sqlite/
│       ├── schema.go
│       ├── connection.go
│       ├── measurement_repository.go
│       └── measurement_repository_test.go
└── delivery/                 # HTTP layer
    └── http/
        ├── server.go
        ├── handlers_api.go
        ├── handlers_web.go
        └── handlers_api_test.go
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test -v ./internal/domain/...
```

### Building

```bash
# Local build
go build -o bin/atmos ./cmd/server

# Docker build
docker build -t atmos:latest .
```

## Deployment

### Synology NAS

1. Open Container Manager
2. Create a new project
3. Upload `docker-compose.yml`
4. Start the container
5. Map port 8080 to desired host port
6. Access via http://NAS_IP:PORT

### VPS

```bash
# Clone repository
git clone <repository-url>
cd atmos

# Start with Docker Compose
docker-compose up -d

# Check logs
docker logs atmos

# Stop
docker-compose down
```

## Troubleshooting

### Database locked errors
- WAL mode should prevent this, but if it occurs, check that only one instance is running
- Verify `_journal_mode=WAL` in connection string

### ESP32 can't connect
- Check firewall settings
- Verify server IP and port
- Test with curl: `curl -X POST http://SERVER:8080/api/measurements -H "Content-Type: application/json" -d '{"room_id":"test","temperature":22.5,"humidity":45,"pressure":1013}'`

### Charts not rendering
- Check browser console for errors
- Verify Chart.js is loading
- Ensure `/api/rooms/{roomID}/measurements` returns valid data

## License

MIT

## Contributing

Contributions welcome! Please open an issue or submit a pull request.
