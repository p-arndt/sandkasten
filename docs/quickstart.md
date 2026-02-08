# Quickstart Guide

Get Sandkasten running in 5 minutes.

## Prerequisites

- Docker Engine installed and running
- Go 1.24+ (for building from source)
- OR: Use pre-built Docker images

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
cd quickstart/daemon
docker-compose up -d
```

This starts the daemon with:
- Python, Node, and Base runtime images
- Port 8080 exposed
- Default API key: `sk-sandbox-quickstart`

### Option 2: Build from Source

```bash
# Build
task build

# Start daemon
./sandkasten --config quickstart/daemon/sandkasten.yaml
```

## Verify It's Running

```bash
# Check health
curl http://localhost:8080/healthz

# Open dashboard
open http://localhost:8080
```

## Your First Session

### Using Python SDK

```bash
# Install
cd quickstart/agent
uv sync

# Set API key
export OPENAI_API_KEY="sk-..."

# Run enhanced agent
uv run enhanced_agent.py
```

### Using cURL

```bash
# Create session
SESSION_ID=$(curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-sandbox-quickstart" \
  -d '{"image":"sandbox-runtime:python"}' | jq -r .id)

# Execute command
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-quickstart" \
  -d '{"cmd":"python3 -c \"print(42)\""}'

# Clean up
curl -X DELETE http://localhost:8080/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer sk-sandbox-quickstart"
```

## Next Steps

- [Configuration Guide](./configuration.md) - Customize settings
- [API Reference](./api.md) - Complete API documentation
- [Features](./features/) - Workspaces, pooling, streaming
- [Examples](../quickstart/agent/) - Agent examples

## Troubleshooting

### "Connection refused"
Daemon not running. Check:
```bash
docker ps  # Should see sandkasten-daemon
docker logs sandkasten-daemon
```

### "Invalid API key"
Check config matches:
```bash
# In sandkasten.yaml
api_key: "sk-sandbox-quickstart"

# In your code
Authorization: Bearer sk-sandbox-quickstart
```

### "Docker not found"
Install Docker:
- [Docker Desktop](https://docs.docker.com/get-docker/)
- [Docker Engine](https://docs.docker.com/engine/install/)
