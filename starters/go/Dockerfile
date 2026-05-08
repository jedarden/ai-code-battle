FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and download dependencies
COPY go.mod .
RUN go mod download

# Copy source files
COPY main.go .
COPY game/ ./game/

# Build the bot
RUN CGO_ENABLED=0 go build -o bot .

FROM alpine:3.21

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bot .

# Set environment variables
ENV BOT_PORT=8080
ENV BOT_SECRET=""

# Expose the bot port
EXPOSE 8080

# Run the bot
CMD ["./bot"]
