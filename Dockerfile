# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git nodejs npm gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy package.json
COPY package.json package-lock.json ./
RUN npm ci

# Copy source code
COPY . .

# Generate code (Templ, SQLC, Swagger)
RUN go generate ./...

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /app/goth ./cmd/...

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite-libs

# Copy binary from builder
COPY --from=builder /app/goth /app/goth

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Copy config
COPY --from=builder /app/config.yaml /app/config.yaml

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["/app/goth"]
