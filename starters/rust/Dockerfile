FROM rust:1.85-alpine AS builder

RUN apk add --no-cache musl-dev

WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src ./src

RUN cargo build --release

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/target/release/starter-bot ./bot

EXPOSE 8080

CMD ["./bot"]
