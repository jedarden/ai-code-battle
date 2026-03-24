# AI Code Battle Match Worker Container
# Polls for match jobs from the Worker API, executes matches, uploads replays to R2

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy engine package
COPY engine/ ./engine/

# Copy worker source
COPY cmd/acb-worker/ ./cmd/acb-worker/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-worker ./cmd/acb-worker

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /acb-worker /app/acb-worker

# Create non-root user
RUN adduser -D -u 1000 acb
USER acb

# Environment variables (set at runtime)
# ACB_API_ENDPOINT - Worker API URL (e.g., https://api.aicodebattle.com)
# ACB_API_KEY - Worker API key for authentication
# ACB_R2_ENDPOINT - R2 endpoint URL
# ACB_R2_BUCKET - R2 bucket name (default: acb-data)
# ACB_R2_ACCESS_KEY - R2 access key ID
# ACB_R2_SECRET_KEY - R2 secret access key
# ACB_WORKER_ID - Unique worker identifier (auto-generated if not set)
# ACB_VERBOSE - Enable verbose logging (true/false)

# Default values
ENV ACB_R2_BUCKET=acb-data

ENTRYPOINT ["/app/acb-worker"]
CMD ["-poll=5s", "-heartbeat=30s", "-timeout=3s"]
