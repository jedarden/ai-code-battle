FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY main.go .
COPY strategy.go .

RUN go build -o gatherer .

FROM alpine:3.19

WORKDIR /app
COPY --from=builder /app/gatherer .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["./gatherer"]
