# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Cache dependency downloads separately from source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is not needed (modernc.org/sqlite is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /app/server \
      ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates — needed for TLS calls to Proxmox / flag submitter
# python3 + bash  — used by the A&D sploit runner (scripts execute in-container)
RUN apk add --no-cache ca-certificates python3 bash && \
    mkdir -p /tmp/sploits

WORKDIR /app

COPY --from=builder /app/server .
COPY frontend/ ./frontend/

EXPOSE 8080

ENTRYPOINT ["./server"]
# Default config path; override via CMD or Docker Compose volume mount
CMD ["config.yaml"]
