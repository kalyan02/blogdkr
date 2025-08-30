FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY oddity/go.mod oddity/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source code and build
COPY oddity/ ./

# Build with verbose output to debug any issues
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -ldflags '-w -s' -o oddity main.go

# Verify the binary was created
# RUN ls -la oddity && file oddity

# Final stage - use minimal alpine image
FROM alpine:3.18

# Install only runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    tzdata && \
    rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 -S backup && \
    adduser -S -D -H -u 1001 -h /var/backups -s /sbin/nologin -G backup -g backup backup

# Create directories
RUN mkdir -p /var/backups /var/log && \
    chown -R backup:backup /var/backups /var/log

# Copy binary from builder stage
COPY --from=builder /app/oddity /usr/local/bin/oddity

# Copy and set up entrypoint
COPY oddity/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set timezone
ENV TZ=UTC

# Switch to non-root user
USER backup

# ENTRYPOINT ["/entrypoint.sh"]
