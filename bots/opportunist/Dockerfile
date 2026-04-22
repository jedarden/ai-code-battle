FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY main.go grid.go strategy.go .

RUN CGO_ENABLED=0 go build -o opportunist .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/opportunist .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["./opportunist"]
