# AI Code Battle Enrichment Container
# Generates AI commentary for featured match replays.
# Polls for matches without commentary, downloads replays from B2/R2,
# generates turn-by-turn narrative highlights via LLM, and stores results.

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy engine package
COPY engine/ ./engine/
COPY metrics/ ./metrics/

# Copy enrichment source
COPY cmd/acb-enrichment/ ./cmd/acb-enrichment/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-enrichment ./cmd/acb-enrichment

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS (LLM API calls, R2/B2 storage)
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /acb-enrichment /app/acb-enrichment

# Create non-root user
RUN adduser -D -u 1000 acb
USER acb

# Environment variables (set at runtime via K8s manifests)
# ACB_DATABASE_URL              - PostgreSQL connection string
# ACB_DATABASE_NAME             - Database name (default: acb)
# ACB_LLM_BASE_URL              - LLM API base URL (e.g., https://api.openai.com/v1)
# ACB_LLM_API_KEY               - LLM API key
# ACB_LLM_MODEL                 - Model to use (default: gpt-4o-mini)
# ACB_LLM_MAX_TOKENS            - Max tokens per request (default: 3000)
# ACB_LLM_TEMPERATURE           - Generation temperature (default: 0.7)
# ACB_ENRICHMENT_MAX_PER_HOUR   - Rate limit: max enrichments per hour (default: 20)
# ACB_ENRICHMENT_MAX_CONCURRENT - Max parallel LLM requests (default: 3)
# ACB_B2_ENDPOINT               - B2 endpoint URL
# ACB_B2_BUCKET                 - B2 bucket name
# ACB_B2_ACCESS_KEY_ID          - B2 access key ID
# ACB_B2_SECRET_ACCESS_KEY      - B2 secret access key
# ACB_R2_ENDPOINT               - R2 endpoint URL
# ACB_R2_BUCKET                 - R2 bucket name
# ACB_R2_ACCESS_KEY_ID          - R2 access key ID
# ACB_R2_SECRET_ACCESS_KEY      - R2 secret access key
# ACB_ENRICHMENT_MIN_TURNS      - Minimum turn count for enrichment (default: 100)
# ACB_ENRICHMENT_MIN_CROSSINGS  - Minimum win prob crossings (default: 3)
# ACB_ENRICHMENT_UPSET_THRESHOLD - Rating diff for upset consideration (default: 150)
# ACB_ENRICHMENT_INTERVAL       - Cycle interval (default: 30m)
# ACB_ENRICHMENT_TIMEOUT        - Max duration per cycle (default: 25m)
# ACB_ENRICHMENT_MAX_LIFETIME   - Process lifetime before restart (default: 4h)

# Default values
ENV ACB_DATABASE_NAME=acb
ENV ACB_LLM_MODEL=gpt-4o-mini
ENV ACB_R2_BUCKET=acb-data
ENV ACB_ENRICHMENT_MAX_PER_HOUR=20
ENV ACB_ENRICHMENT_MAX_CONCURRENT=3
ENV ACB_ENRICHMENT_MIN_TURNS=100
ENV ACB_ENRICHMENT_MIN_CROSSINGS=3
ENV ACB_ENRICHMENT_UPSET_THRESHOLD=150
ENV ACB_ENRICHMENT_INTERVAL=30m
ENV ACB_ENRICHMENT_MAX_LIFETIME=4h

ENTRYPOINT ["/app/acb-enrichment"]
