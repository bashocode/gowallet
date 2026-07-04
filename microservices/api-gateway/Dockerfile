# === Stage 1: Builder ===
FROM golang:1.26.4-alpine AS builder
WORKDIR /app

# Copy go.work and local module dependencies
COPY go.work ./
COPY shared/ ./shared/
COPY api-gateway/ ./api-gateway/
COPY auth-service/ ./auth-service/
COPY user-service/ ./user-service/
COPY wallet-service/ ./wallet-service/
COPY ledger-service/ ./ledger-service/

WORKDIR /app/api-gateway
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /service ./cmd/main.go

# === Stage 2: Production Image ===
FROM alpine:3.24.1
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /service .

USER appuser
EXPOSE 8080
CMD ["./service"]
