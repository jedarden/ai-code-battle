# AI Code Battle Evolver Container
# Runs the autonomous evolution pipeline: generates candidate bots via LLM,
# validates them (syntax → schema → sandbox), runs arena tournaments,
# and promotes winners to the live bot fleet.
#
# Needs multiple language runtimes for the 3-stage validation pipeline.

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install git for module downloads
RUN apk --no-cache add git

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy engine package
COPY engine/ ./engine/

# Copy bots (used as seed programs)
COPY bots/ ./bots/

# Copy evolver source
COPY cmd/acb-evolver/ ./cmd/acb-evolver/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-evolver ./cmd/acb-evolver

# Runtime stage — needs language runtimes for validation sandbox
FROM golang:1.25-alpine

WORKDIR /app

# Install language runtimes for bot validation:
# - go (included in base image)
# - python3, nodejs (typescript), rust (rustup-init), java, php
# - ca-certificates for HTTPS (LLM calls, K8s API)
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    python3 \
    py3-pip \
    nodejs \
    npm \
    curl \
    bash

# Install TypeScript compiler globally
RUN npm install -g typescript ts-node

# Copy binary from builder
COPY --from=builder /acb-evolver /app/acb-evolver

# Create non-root user
RUN addgroup -g 1000 acb && adduser -D -u 1000 -G acb acb
USER acb

# Environment variables (set at runtime)
# ACB_DATABASE_URL      - PostgreSQL connection string
# ACB_LLM_URL           - LLM proxy base URL (e.g., http://zai-proxy:8080)
# ACB_LLM_API_KEY       - LLM API key (optional, for ZAI proxy)
# ACB_ENCRYPTION_KEY    - AES-256-GCM key for bots table shared secrets
# ACB_KUBECTL_SERVER    - kubectl API server URL for bot deployment
# ACB_REGISTRY          - Container registry for evolved bot images
# ACB_REPO_DIR          - Path to declarative-config repo for K8s manifest writes
# ACB_R2_ENDPOINT       - R2 endpoint for live.json upload
# ACB_R2_BUCKET         - R2 bucket name
# ACB_R2_ACCESS_KEY     - R2 access key
# ACB_R2_SECRET_KEY     - R2 secret key
# ACB_EVOLUTION_OUT     - Output path for live.json (default: evolution/live.json)

ENTRYPOINT ["/app/acb-evolver"]
CMD ["run"]
