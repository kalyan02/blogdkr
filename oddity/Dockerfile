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
    tzdata \
    sqlite \
    imagemagick && \
    rm -rf /var/cache/apk/*

# Create non-root user
ARG USER_UID=1000
ARG USER_GID=1000
RUN addgroup -g ${USER_GID} -S oddity && \
    adduser -S -D -H -u ${USER_UID} -h /var/oddity -s /sbin/nologin -G oddity -g oddity oddity

# Create directories
RUN mkdir -p /var/oddity /var/log /var/oddity/data && \
    chown -R oddity:oddity /var/oddity /var/log

# Copy binary from builder stage
COPY --from=builder /app/oddity /usr/local/bin/oddity

# Copy and set up entrypoint
COPY oddity/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set timezone
ENV TZ=UTC

# Switch to non-root user
USER oddity

# ENTRYPOINT ["/entrypoint.sh"]
