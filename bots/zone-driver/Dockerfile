# Build stage
FROM rust:1.85-alpine AS builder

WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src ./src

RUN apk add --no-cache musl-dev && cargo build --release --locked

# Runtime stage
FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/target/release/zone-driver-bot /app/zone-driver-bot

ENV BOT_PORT=8082
ENV BOT_SECRET=""

EXPOSE 8082

CMD ["./zone-driver-bot"]
