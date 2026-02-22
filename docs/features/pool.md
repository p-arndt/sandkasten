# Pre-warmed Session Pool

Pre-create sandbox sessions so that `POST /v1/sessions` can return in **sub-100ms** (often sub-1ms in local benchmarks) when a matching session is available in the pool, instead of cold create latency.

## How It Works

```
Daemon Startup
├─ Create N sandboxes per configured image (no workspace)
├─ Store sessions with status "pool_idle" (not subject to TTL expiry)
└─ Pool ready

User Creates Session
├─ pool.Get(image, workspace_id) → matching session available? → acquire from pool ✅
├─ Update status to "running", set TTL, return to user
├─ Refill pool in background for that key
└─ If pool empty → normal create, then refill in background
```

**Scope:** The pool supports both sessions **with** and **without** `workspace_id` using workspace-aware keys.

## Configuration

```yaml
pool:
  enabled: true
  images:
    python: 3   # keep 3 Python sessions ready
    node: 2     # keep 2 Node sessions ready
```

- **`enabled`** (default: `false`): Turn pooling on. Keep disabled if you don't need sub-100ms create latency.
- **`images`**: Map of image name → number of pre-warmed sessions to maintain.

Pool images are filtered by `allowed_images`. If `allowed_images` is set and an image is not in the list, it is not pooled.

Environment override: `SANDKASTEN_POOL_ENABLED=true`

## When to Use

Pooling helps when:

- Session creation is a hot path (many sessions per second)
- Sub-100ms latency is important
- You want to hide cold-start overhead

For typical usage (~100–200ms cold create), pooling is optional.

## Implementation Details

- **Pool lifecycle:** At daemon startup, `RefillAll` creates the configured number of sandboxes per image (workspace_id = empty). They are stored with status `pool_idle` and far-future expiry so the reaper does not destroy them.
- **Acquire:** `pool.Get` returns a session ID, which is removed from the pool. The session status is updated to `running` and `expires_at` is set to the user’s TTL.
- **Refill:** After each create (pool hit or normal), a background goroutine calls `Refill` to replenish the specific key (`image` or `image+workspace_id`).
- **Release:** When a session is destroyed, it is not returned to the pool. The pool is replenished on demand by `Refill` after future creates.

## Workspace Support

Sessions created with `workspace_id` can also use the pool via workspace-aware entries. The flow:

1. Check pool key `(image, workspace_id)`
2. If hit: acquire directly and return
3. If miss: normal create with `workspace_id`, then refill that key in background

This avoids the previous late bind-mount-on-acquire path on readonly roots and gives consistent warm behavior for shared workspaces.
