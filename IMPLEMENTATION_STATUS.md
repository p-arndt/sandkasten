# Persisted Workspaces & Pre-warmed Pool Implementation Status

## ✅ Completed

### 1. Workspace Manager (`internal/workspace/`)
- [x] Create/delete/list workspace volumes
- [x] Volume existence checking
- [x] Workspace metadata tracking

### 2. Pool Manager (`internal/pool/`)
- [x] Pre-warm container pool
- [x] Configurable per-image pool sizes
- [x] Auto-refill background workers
- [x] Get/Return pool operations

### 3. Config Updates
- [x] Added `PoolConfig` struct
- [x] Added `WorkspaceConfig` struct
- [x] YAML configuration support

### 4. Store Updates
- [x] Added `workspace_id` field to Session
- [x] Updated all SQL queries
- [x] Database migration for workspace_id column

## ✅ Completed (Full Implementation)

### 5. Docker Client Updates
- [x] Mount workspace volumes to containers
- [x] Update CreateContainer to accept workspace_id
- [x] Handle both ephemeral and persistent modes
- [x] Support custom labels for pool containers

### 6. Session Manager Updates
- [x] Integrate workspace manager
- [x] Integrate pool manager
- [x] Update Create to use pool when available
- [x] Handle workspace volume mounting
- [x] Auto-create workspace volumes on demand
- [x] Add workspace management methods (List, Delete)

### 7. API Updates
- [x] Add `workspace_id` param to POST /v1/sessions
- [x] Add GET /v1/workspaces
- [x] Add DELETE /v1/workspaces/{id}
- [x] Update session responses to include workspace_id

### 8. Daemon Integration
- [x] Wire up workspace manager
- [x] Wire up pool manager
- [x] Start pool on daemon startup
- [x] Stop pool on daemon shutdown
- [x] Expose Docker client for workspace manager

### 9. SDK Updates
- [x] Update Python SDK for workspace_id
- [x] Add workspace management methods to Python SDK
- [ ] Update TypeScript SDK for workspace_id (TODO)

### 10. Documentation
- [x] Update CHANGELOG.md
- [x] Document workspace persistence feature
- [x] Document pool configuration
- [x] Example config files (sandkasten-full.yaml)

## Configuration Example

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-quickstart"

# Persistent workspaces
workspace:
  enabled: true
  persist_by_default: false  # require explicit workspace_id

# Pre-warmed container pool
pool:
  enabled: true
  images:
    sandbox-runtime:python: 3  # keep 3 ready
    sandbox-runtime:node: 2

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  network_mode: "full"
```

## API Usage Examples

### Create session with persistent workspace

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-..." \
  -d '{
    "image": "sandbox-runtime:python",
    "workspace_id": "user123-project456"
  }'
```

### List workspaces

```bash
curl http://localhost:8080/v1/workspaces \
  -H "Authorization: Bearer sk-..."
```

### Delete workspace

```bash
curl -X DELETE http://localhost:8080/v1/workspaces/user123-project456 \
  -H "Authorization: Bearer sk-..."
```

## Next Steps

1. Update Docker client to mount volumes
2. Update session manager to integrate pool + workspaces
3. Add API endpoints
4. Update SDKs
5. Test end-to-end
6. Update documentation
