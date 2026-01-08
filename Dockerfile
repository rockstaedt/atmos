# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy dependency files first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o atmos \
    ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Create non-root user
RUN addgroup -g 1000 atmos && \
    adduser -D -u 1000 -G atmos atmos

# Create data directory
RUN mkdir -p /data && chown atmos:atmos /data

WORKDIR /app

# Copy binary and web assets
COPY --from=builder /build/atmos .
COPY --from=builder /build/web ./web

# Use non-root user
USER atmos

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Volume for persistent data
VOLUME ["/data"]

# Set environment variables
ENV PORT=8080 \
    DB_PATH=/data/atmos.db \
    TEMPLATES_DIR=/app/web/templates \
    STATIC_DIR=/app/web/static

ENTRYPOINT ["./atmos"]
