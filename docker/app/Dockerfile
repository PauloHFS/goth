# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache gcc musl-dev npm make

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install tools from go.mod tool directive
RUN go install github.com/a-h/templ/cmd/templ
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc
RUN go install github.com/swaggo/swag/cmd/swag

# Build the application
RUN make build

# Production stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/bin/goth .

# Copy config files
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/migrations ./migrations

# Create storage directory (will be mounted as volume)
RUN mkdir -p /root/storage/avatars

# Health check - Traefik will use this
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run as non-root user (optional, recommended for security)
# RUN adduser -D -g '' appuser
# USER appuser

CMD ["./goth", "server"]
