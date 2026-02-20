# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binaries directly
# Build runner as a static Linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/runner ./cmd/runner

# Build daemon and imgbuilder
RUN go build -o bin/sandkasten ./cmd/sandkasten
RUN go build -o bin/imgbuilder ./cmd/imgbuilder

# Runtime Stage
FROM debian:bookworm-slim

# Install runtime dependencies
# - iproute2 + iptables + procps: Required for sandbox bridge networking
# - ca-certificates: Required for HTTPS requests (if any)
RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iptables \
    procps \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy binaries from builder to /bin
COPY --from=builder /app/bin/sandkasten /bin/sandkasten
COPY --from=builder /app/bin/runner /bin/runner
COPY --from=builder /app/bin/imgbuilder /bin/imgbuilder

# Create data directory and config directory
RUN mkdir -p /var/lib/sandkasten /etc/sandkasten

# Copy default config
COPY sandkasten.yaml /etc/sandkasten/sandkasten.yaml

# Expose API port
EXPOSE 8080

# Default entrypoint
CMD ["/bin/sandkasten", "--config", "/etc/sandkasten/sandkasten.yaml"]
