# Pre-warmed Container Pool

Background pool of ready containers for instant session creation.

## Overview

By default, creating a session requires:
1. Pull image (if needed)
2. Create container (~500ms)
3. Start container (~1-2s)
4. Wait for runner (~500ms)

**Total: 2-3 seconds**

With pooling: **~50ms** ⚡

## How It Works

```
Daemon Startup
├─ Create 3 Python containers
├─ Create 2 Node containers
└─ Start background refill workers

User Creates Session
├─ Get container from pool → instant! ✅
└─ Refill pool in background
```

## Configuration

Enable in `sandkasten.yaml`:

```yaml
pool:
  enabled: true
  images:
    sandbox-runtime:python: 3  # Keep 3 Python containers ready
    sandbox-runtime:node: 2    # Keep 2 Node containers ready
```

## Performance

| Metric | Without Pool | With Pool |
|--------|-------------|-----------|
| Session creation | 2-3s | ~50ms |
| Resource overhead | None | Minimal (idle containers) |
| Memory per pooled | ~50MB | ~50MB |
| CPU per pooled | ~0% | ~0% |

## Pool Behavior

### Startup

On daemon start:
1. Read pool config
2. Pre-create configured number of containers
3. Start auto-refill workers

### Runtime

- **Get:** Non-blocking, returns container from pool or empty
- **Return:** Return unused containers to pool (if space)
- **Refill:** Background workers maintain target pool size
- **Overflow:** Extra containers are destroyed

### Shutdown

On daemon stop:
1. Stop accepting new gets
2. Drain pool
3. Destroy all pooled containers

## Monitoring

Check daemon logs:

```
INFO container pool started images=map[sandbox-runtime:python:3 ...]
INFO refilling pool image=sandbox-runtime:python current=2 target=3 creating=1
INFO created pooled container image=sandbox-runtime:python container=abc123
INFO using pooled container image=sandbox-runtime:python container=abc123
```

## Best Practices

### 1. Size Based on Load

```yaml
pool:
  enabled: true
  images:
    # High traffic image
    sandbox-runtime:python: 10

    # Low traffic image
    sandbox-runtime:node: 2
```

### 2. Monitor Pool Hit Rate

Track how often pool is used:

```bash
# Check logs
docker logs sandkasten-daemon | grep "using pooled"
docker logs sandkasten-daemon | grep "create container"
```

### 3. Start Small

```yaml
# Start conservative
pool:
  images:
    sandbox-runtime:python: 2
```

Then increase based on usage.

### 4. Match Peak Hours

```yaml
# High pool during business hours
# Low pool overnight
# (Requires dynamic config reload - future feature)
```

## Limitations

- **No per-workspace pooling** - Pool containers are ephemeral
- **Fixed pool size** - No auto-scaling (yet)
- **Memory overhead** - Each pooled container uses ~50MB idle
- **Startup time** - Takes time to initially fill pool

## Advanced

### Pool Metrics (Manual)

```bash
# Count pooled containers
docker ps --filter label=sandkasten.pool=true --format "{{.Image}}" | sort | uniq -c

# Check pool labels
docker inspect <container> | jq '.[0].Config.Labels'
```

### Disable Pool for Image

Remove from config or set to 0:

```yaml
pool:
  images:
    sandbox-runtime:python: 0  # Disabled
```

### Pool Warmup Strategy

Pool warms up in background. For instant availability:

```bash
# Start daemon
./sandkasten &

# Wait for pool to fill
sleep 10

# Now pool is ready
```

## Troubleshooting

### "Pool not starting"

Check config:
```yaml
pool:
  enabled: true  # ← Must be true
  images:
    sandbox-runtime:python: 3  # ← At least one image
```

### "Pool empty immediately"

Pool may be undersized. Increase pool size or check container creation errors in logs.

### High memory usage

Reduce pool sizes:
```yaml
pool:
  images:
    sandbox-runtime:python: 1  # Reduced
```

Or disable pooling:
```yaml
pool:
  enabled: false
```
