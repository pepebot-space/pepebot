# Multi-stage build for minimal image size
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o pepebot \
    ./cmd/pepebot

# Final image with Ubuntu LTS
FROM ubuntu:24.04

# Set environment to prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install required utilities
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    python3 \
    python3-pip \
    cron \
    tmux \
    bash \
    curl \
    vim \
    nano \
    htop \
    net-tools \
    iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Install docker-systemctl-replacement
RUN curl -L https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl3.py \
    -o /usr/local/bin/systemctl \
    && chmod +x /usr/local/bin/systemctl

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy binary
COPY --from=builder /build/pepebot /usr/local/bin/pepebot

# Create cron directories and log files
RUN mkdir -p /var/log /var/spool/cron/crontabs \
    && touch /var/log/cron.log \
    && touch /var/log/pepebot-cron.log

# Create entrypoint script for Ubuntu
RUN cat > /entrypoint.sh << 'EOF'
#!/bin/bash
set -e

echo "🐸 Starting Pepebot with cron support..."

# Create cron log file if it doesn't exist
touch /var/log/cron.log

# Start cron daemon in background (Ubuntu uses 'cron' not 'crond')
echo "Starting cron daemon..."
cron
echo "Cron daemon started (PID: $(pgrep cron))"

# Trap SIGTERM and SIGINT to gracefully stop services
trap 'echo "Shutting down..."; service cron stop; exit 0' SIGTERM SIGINT

# Log cron output
tail -f /var/log/cron.log /var/log/pepebot-cron.log 2>/dev/null &

# Start pepebot gateway
echo "Starting pepebot gateway..."
exec /usr/local/bin/pepebot "$@"
EOF

RUN chmod +x /entrypoint.sh

# Note: Built-in skills are now fetched from GitHub during onboarding
# Users can install them with: pepebot skills install-builtin

# Set up workspace directory
VOLUME ["/root/.pepebot"]

# Expose default gateway port
EXPOSE 18790

# Use entrypoint script
ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]

# Metadata
LABEL org.opencontainers.image.title="Pepebot"
LABEL org.opencontainers.image.description="Ultra-lightweight Personal AI Assistant"
LABEL org.opencontainers.image.authors="Pepebot Contributors"
LABEL org.opencontainers.image.url="https://github.com/pepebot-space/pepebot"
LABEL org.opencontainers.image.source="https://github.com/pepebot-space/pepebot"
LABEL org.opencontainers.image.licenses="MIT"
