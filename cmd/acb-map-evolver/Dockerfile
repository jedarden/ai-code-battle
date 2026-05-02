FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk --no-cache add git

COPY go.mod go.sum ./
RUN go mod download

COPY engine/ ./engine/
COPY cmd/acb-map-evolver/ ./cmd/acb-map-evolver/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /acb-map-evolver ./cmd/acb-map-evolver

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 acb && adduser -D -u 1000 -G acb acb

COPY --from=builder /acb-map-evolver /app/acb-map-evolver

USER acb

# ACB_DATABASE_URL      - PostgreSQL connection string (required)
# ACB_MIN_SEED_COUNT    - Maps to seed per player count on startup [default: 20]
# ACB_EVOLUTION_PERIOD  - Sleep between evolution cycles [default: 30m]

ENTRYPOINT ["/app/acb-map-evolver"]
