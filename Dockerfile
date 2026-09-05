# ==============================================================================
# Multi-Stage Minimal Dockerfile for SearXGo (SearXNG in Golang)
# Zero external runtime dependencies, standalone binary
# ==============================================================================

# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ca-certificates and git
RUN apk add --no-cache ca-certificates git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o searxgo ./cmd/server

# Final Lightweight Runtime Stage
FROM alpine:3.20

WORKDIR /app

# Install root TLS certificates and tzdata
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' searxgo && chown -R searxgo:searxgo /app

# Copy binary from builder
COPY --from=builder /app/searxgo /app/searxgo
COPY --from=builder /app/settings.yml /app/settings.yml

USER searxgo

EXPOSE 8184

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8184/healthz || exit 1

ENTRYPOINT ["/app/searxgo"]
CMD ["-port", "8184"]
