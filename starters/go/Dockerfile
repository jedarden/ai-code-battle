FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY main.go .

RUN CGO_ENABLED=0 go build -o bot .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/bot .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["./bot"]
