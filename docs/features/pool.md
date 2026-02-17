# Pre-warmed Session Pool

> **Note:** The container pool feature is currently unavailable in v2. Session creation using native Linux sandboxing is already fast (~100-200ms), reducing the need for pre-warming.

## Overview

In v1 (Docker-based), creating a session required:
1. Pull image (if needed)
2. Create container (~500ms)
3. Start container (~1-2s)
4. Wait for runner (~500ms)

**Total: 2-3 seconds**

With v2's native Linux sandboxing:
- No container runtime overhead
- Direct namespace/cgroup creation
- overlayfs mounting is fast

**Total: ~100-200ms** ⚡

## Future Improvements

The following are planned for future releases:

### Warm Session Pools

Pre-create sandbox processes ready for immediate use:

```
Daemon Startup
├─ Create 3 Python sandboxes (namespaces + cgroups ready)
├─ Create 2 Node sandboxes
└─ Ready for instant assignment

User Creates Session
├─ Get sandbox from pool → instant! ✅
└─ Refill pool in background
```

### Configuration (Planned)

```yaml
pool:
  enabled: true
  images:
    python: 3
    node: 2
```

## Current Performance

| Operation | Time |
|-----------|------|
| Create sandbox | ~100-200ms |
| Exec command | ~10-50ms |
| Destroy sandbox | ~50-100ms |

For most use cases, this is fast enough that pooling is not needed.

## When Pooling Helps

Pooling becomes valuable when:
- Session creation is a hot path (many per second)
- Sub-100ms latency is critical
- You want to hide image I/O latency

If you need pooling now, consider:
- Running multiple daemon instances
- Load balancing across instances
- Caching sessions instead of destroying them
