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

# Production stage - minimal image
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/sentinel /sentinel

# Copy configuration files
COPY configs/ /configs/

# Copy dashboard HTML file
COPY web/ /web/

# Create directories for logs (using VOLUME for persistence)
VOLUME ["/logs"]

# Expose the default port
EXPOSE 8080

# Health check using the binary itself
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/sentinel", "--config", "/configs/default.yaml", "--health-check"]

# Run as non-root user (using numeric UID for scratch image)
USER 65534:65534

# Start the application
ENTRYPOINT ["/sentinel"]
CMD ["--config", "/configs/default.yaml"]