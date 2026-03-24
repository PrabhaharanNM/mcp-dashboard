FROM golang:1.22-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# SQLite requires CGO
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o dashboard ./cmd/dashboard

# --- Runtime ---
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/dashboard .
COPY --from=builder /build/web/ ./web/

RUN mkdir -p /data

EXPOSE 9090
CMD ["./dashboard", "-addr=:9090", "-db=/data/dashboard.db"]
