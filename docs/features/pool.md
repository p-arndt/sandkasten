# Pre-warmed Session Pool

Pre-create sandbox sessions so that `POST /v1/sessions` can return in **sub-100ms** when a session is available in the pool (target ~50ms), instead of ~200–450ms for a cold create.

## How It Works

```
Daemon Startup
├─ Create N sandboxes per configured image (no workspace)
├─ Store sessions with status "pool_idle" (not subject to TTL expiry)
└─ Pool ready

User Creates Session (with or without workspace_id)
├─ pool.Get(image) → session available? → acquire from pool (~50–80ms) ✅
├─ If workspace_id: nsenter bind-mount workspace into /workspace
├─ Update status to "running", set TTL, return to user
├─ Refill pool in background (replenish 1 session)
└─ If pool empty → normal create (~200–450ms), then refill in background
```

**Scope:** The pool supports both sessions **with** and **without** `workspace_id`. Pooled sessions start without a workspace; when a request includes `workspace_id`, the workspace directory is bind-mounted into `/workspace` at acquire time via `nsenter`.

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

## Workspace Support

Sessions created with `workspace_id` can also use the pool. The flow:

1. Take an idle session from the pool (created without workspace)
2. Use `nsenter` to enter the sandbox's mount namespace
3. Bind-mount the workspace directory into `/workspace`
4. Update the store and return the session to the user

Adds ~10–30ms to pool acquire for the bind-mount step, still much faster than cold create. Requires `workspace.enabled: true` and the workspace to exist (created via `ensureWorkspace` before acquire).
