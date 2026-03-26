# AI Code Battle API Server
# Provides bot registration, job coordination, matchmaking, health checks,
# and Glicko-2 rating updates. Connects to PostgreSQL and Valkey.

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy engine package (needed for shared types)
COPY engine/ ./engine/

# Copy API server source
COPY cmd/acb-api/ ./cmd/acb-api/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-api ./cmd/acb-api

# Runtime stage
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /acb-api /app/acb-api

# Create non-root user
RUN adduser -D -u 1000 acb
USER acb

# Environment variables (set at runtime)
# ACB_LISTEN_ADDR       - Listen address (default: :8080)
# ACB_DATABASE_URL      - PostgreSQL connection string
# ACB_VALKEY_ADDR       - Valkey/Redis address (default: localhost:6379)
# ACB_VALKEY_PASSWORD   - Valkey password
# ACB_WORKER_API_KEY    - API key for worker authentication
# ACB_ENCRYPTION_KEY    - AES-256-GCM key for shared secret encryption (64 hex chars)
# ACB_MATCHMAKER_INTERVAL  - Matchmaker tick interval in seconds (default: 60)
# ACB_HEALTHCHECK_INTERVAL - Health check interval in seconds (default: 900)
# ACB_REAPER_INTERVAL      - Stale job reaper interval in seconds (default: 300)

EXPOSE 8080

ENTRYPOINT ["/app/acb-api"]
