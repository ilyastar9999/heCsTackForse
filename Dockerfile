FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /app/server \
      ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates python3 bash docker-cli tzdata wget && \
    mkdir -p /tmp/sploits

WORKDIR /app

COPY --from=builder /app/server .
COPY frontend/ ./frontend/
COPY python/ ./python/
COPY challenges/ ./challenges/
COPY themes/ ./themes/
COPY config.example.yaml ./config.example.yaml

EXPOSE 8080

ENTRYPOINT ["./server"]
CMD ["config.yaml"]
