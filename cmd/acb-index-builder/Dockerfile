# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-index-builder ./cmd/acb-index-builder

# Runtime stage
FROM node:22-alpine

# Install wrangler CLI for Cloudflare Pages deployment
RUN npm install -g wrangler@3

# Install ca-certificates for HTTPS calls
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /acb-index-builder /usr/local/bin/acb-index-builder

# Create data directory
RUN mkdir -p /data

# Set environment defaults
ENV ACB_OUTPUT_DIR=/data
ENV ACB_BUILD_INTERVAL=15m
ENV ACB_DEPLOY_INTERVAL=6
ENV ACB_MAX_LIFETIME=4h
ENV ACB_BUILD_TIMEOUT=10m

# Run as non-root user
RUN addgroup -g 1001 acb && adduser -u 1001 -G acb -s /bin/sh -D acb
RUN chown -R acb:acb /data
USER acb

ENTRYPOINT ["/usr/local/bin/acb-index-builder"]
