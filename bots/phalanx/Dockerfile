# Build stage
FROM rust:1.85-alpine AS builder

WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src ./src

RUN apk add --no-cache musl-dev && cargo build --release --locked

# Runtime stage
FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/target/release/phalanx-bot /app/phalanx-bot

ENV BOT_PORT=8090
ENV BOT_SECRET=""

EXPOSE 8090

CMD ["./phalanx-bot"]
