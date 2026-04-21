FROM rust:1.85-alpine AS builder

WORKDIR /app
COPY Cargo.toml ./
COPY src ./src

RUN cargo build --release

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/target/release/acb-starter-bot /app/bot

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["./bot"]
