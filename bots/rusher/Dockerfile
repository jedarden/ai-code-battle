# Build stage
FROM rust:1.85-alpine AS builder

WORKDIR /app
COPY Cargo.toml ./
COPY src ./src

RUN cargo build --release

# Runtime stage
FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/target/release/rusher-bot /app/rusher-bot

ENV BOT_PORT=8082
ENV BOT_SECRET=""

EXPOSE 8082

CMD ["./rusher-bot"]
