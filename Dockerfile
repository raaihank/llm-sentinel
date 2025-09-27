# Multi-stage build for optimal Go binary
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy Go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the Go binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o sentinel ./cmd/sentinel

# Production stage - use alpine instead of scratch for better file system support
FROM alpine:3.19

# Install ca-certificates (user 65534 already exists as 'nobody')
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /app/sentinel /sentinel

# Copy configuration files
COPY configs/ /configs/

# Copy dashboard HTML file
COPY web/ /web/

# Create directories with proper ownership (using nobody user)
RUN mkdir -p /logs /models/cache && \
    chown -R nobody:nobody /logs /models && \
    chmod -R 755 /models

# Create directories for logs and models (using VOLUME for persistence)
VOLUME ["/logs", "/models"]

# Expose the default port
EXPOSE 8080

# Health check using the binary itself
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/sentinel", "--config", "/configs/default.yaml", "--health-check"]

# Run as non-root user (nobody)
USER nobody:nobody

# Start the application
ENTRYPOINT ["/sentinel"]
CMD ["--config", "/configs/default.yaml"]