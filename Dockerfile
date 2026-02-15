# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

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

# Final minimal image
FROM scratch

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy binary
COPY --from=builder /build/pepebot /usr/local/bin/pepebot

# Copy built-in skills
COPY skills /pepebot/skills

# Set up workspace directory
VOLUME ["/root/.pepebot"]

# Expose default gateway port
EXPOSE 18790

# Default command
ENTRYPOINT ["/usr/local/bin/pepebot"]
CMD ["gateway"]

# Metadata
LABEL org.opencontainers.image.title="Pepebot"
LABEL org.opencontainers.image.description="Ultra-lightweight Personal AI Assistant"
LABEL org.opencontainers.image.authors="Pepebot Contributors"
LABEL org.opencontainers.image.url="https://github.com/pepebot-space/pepebot"
LABEL org.opencontainers.image.source="https://github.com/pepebot-space/pepebot"
LABEL org.opencontainers.image.licenses="MIT"
