# Pre-warmed Session Pool

Pre-create sandbox sessions so that `POST /v1/sessions` can return in **sub-100ms** when a session is available in the pool (target ~50ms), instead of ~200–450ms for a cold create.

## How It Works

```
Daemon Startup
├─ Create N sandboxes per configured image (no workspace)
├─ Store sessions with status "pool_idle" (not subject to TTL expiry)
└─ Pool ready

User Creates Session (without workspace_id)
├─ pool.Get(image) → session available? → acquire from pool (~50ms) ✅
├─ Update status to "running", set TTL, return to user
├─ Refill pool in background (replenish 1 session)
└─ If pool empty → normal create (~200–450ms), then refill in background
```

**Scope:** The pool is used only for sessions **without** `workspace_id`. Sessions with a workspace always use the normal create path, because workspace sessions require a bind-mount at create time.

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

- **Pool lifecycle:** At daemon startup, `RefillAll` creates the configured number of sandboxes per image. They are stored with status `pool_idle` and a far-future expiry so the reaper does not destroy them.
- **Acquire:** `pool.Get` returns a session ID, which is removed from the pool. The session status is updated to `running` and `expires_at` is set to the user’s TTL.
- **Refill:** After each create (pool hit or normal), a background goroutine calls `Refill` to add one session back for that image.
- **Release:** When a session is destroyed, it is not returned to the pool. The pool is replenished on demand by `Refill` after future creates.

## Future Work (Phase 1.5)

- Support `workspace_id` on acquire via bind-mount at runtime (documented as future extension).
