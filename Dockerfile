# jig-mcp Dockerfile
# Multi-stage build for minimal production image
#
# Usage:
#   docker build -t jig-mcp .
#   docker run -p 3001:3001 -v ./tools:/jig-mcp/tools -v ./scripts:/jig-mcp/scripts jig-mcp
#
# Environment variables:
#   JIG_TRANSPORT=stdio|sse (default: stdio)
#   JIG_SSE_PORT=3001 (default: 3001)
#   JIG_AUTH_TOKEN=<token> (required for SSE mode)
#   JIG_TOOL_TIMEOUT=30s (default: 30s)
#   JIG_LOG_LEVEL=info|debug|warn|error (default: info)

# Stage 1: Build
FROM golang:1.25 AS builder

WORKDIR /build

# Install dependencies first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.Version=${VERSION}" -o jig-mcp ./cmd/jig-mcp/

# Stage 2: Runtime
FROM debian:bookworm-slim

# Install ca-certificates for HTTPS requests and curl for healthcheck
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd --create-home --shell /bin/bash jig-mcp

# Copy binary from builder
COPY --from=builder /build/jig-mcp /usr/local/bin/jig-mcp

# Create tools and scripts directories
RUN mkdir -p /jig-mcp/tools /jig-mcp/scripts /jig-mcp/logs && \
    chown -R jig-mcp:jig-mcp /jig-mcp

WORKDIR /jig-mcp

# Default port for SSE transport
EXPOSE 3001

# Switch to non-root user
USER jig-mcp

# Health check for SSE mode
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3001/health || exit 1

# Default command (stdio mode)
# Override with: docker run ... jig-mcp -transport sse -port 3001
ENTRYPOINT ["jig-mcp"]
