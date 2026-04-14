# AI Code Battle Matchmaker Container
# Internal service that runs tickers for bot pairing, health checking, and stale job reaping

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy engine package (if needed)
COPY engine/ ./engine/

# Copy matchmaker source
COPY cmd/acb-matchmaker/ ./cmd/acb-matchmaker/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-matchmaker ./cmd/acb-matchmaker

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS (for bot health checks)
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /acb-matchmaker /app/acb-matchmaker

# Create non-root user
RUN adduser -D -u 1000 acb
USER acb

# Environment variables (set at runtime)
# ACB_DATABASE_URL - PostgreSQL connection string
# ACB_VALKEY_ADDR - Valkey/Redis address (default: localhost:6379)
# ACB_VALKEY_PASSWORD - Valkey/Redis password (optional)
# ACB_ENCRYPTION_KEY - AES-256-GCM key for decrypting shared secrets (64 hex chars)
# ACB_DISCORD_WEBHOOK - Discord webhook URL for alerts (optional)
# ACB_SLACK_WEBHOOK - Slack webhook URL for alerts (optional)
# ACB_MATCHMAKER_INTERVAL - Seconds between matchmaking cycles (default: 60)
# ACB_HEALTHCHECK_INTERVAL - Seconds between health checks (default: 900)
# ACB_REAPER_INTERVAL - Seconds between stale job reaper runs (default: 300)
# ACB_BOT_TIMEOUT - HTTP timeout for bot health checks in seconds (default: 5)
# ACB_STALE_JOB_MINUTES - Minutes before a running job is considered stale (default: 15)
# ACB_MAX_CONSEC_FAILS - Consecutive failures before marking bot inactive (default: 3)

ENTRYPOINT ["/app/acb-matchmaker"]
