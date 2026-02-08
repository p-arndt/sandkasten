# Persistent Workspaces

Docker volume-backed storage that survives session destruction.

## Overview

By default, sessions are **ephemeral** - files are lost when the session is destroyed. With persistent workspaces, files are stored in named Docker volumes that survive session destruction.

## Use Cases

- **Resume work** - Pick up where you left off
- **Long-running projects** - Work across multiple sessions
- **Collaboration** - Share workspace ID across users/agents
- **Reliability** - Survive crashes and restarts

## How It Works

```
Session 1 (workspace: "user123-project")
├─ write file.py
├─ execute code
└─ destroy → files persist in volume

Session 2 (workspace: "user123-project")
├─ read file.py → ✅ still there!
└─ continue work
```

## Configuration

Enable in `sandkasten.yaml`:

```yaml
workspace:
  enabled: true
  persist_by_default: false  # require explicit workspace_id
```

## API Usage

### Create Session with Workspace

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-..." \
  -d '{
    "image": "sandbox-runtime:python",
    "workspace_id": "user123-project"
  }'
```

**Without workspace_id:** Session uses ephemeral storage (default)

### List Workspaces

```bash
curl http://localhost:8080/v1/workspaces \
  -H "Authorization: Bearer sk-..."
```

### Delete Workspace

```bash
curl -X DELETE http://localhost:8080/v1/workspaces/user123-project \
  -H "Authorization: Bearer sk-..."
```

⚠️ **Warning:** This permanently deletes all files.

## SDK Usage

### Python

```python
from sandkasten import SandboxClient

client = SandboxClient(base_url="...", api_key="...")

# Create with persistent workspace
session = await client.create_session(workspace_id="user123-project")
await session.write("code.py", "print('hello')")
await session.destroy()

# Later - reconnect
session2 = await client.create_session(workspace_id="user123-project")
content = await session2.read("code.py")
print(content)  # ✅ print('hello')

# List workspaces
workspaces = await client.list_workspaces()

# Delete workspace
await client.delete_workspace("user123-project")
```

## Workspace ID Patterns

### Per-User Workspaces

```python
workspace_id = f"user-{user_id}"
```

### Per-Project Workspaces

```python
workspace_id = f"{user_id}-{project_id}"
```

### Temporary Workspaces

```python
workspace_id = f"temp-{uuid.uuid4()}"
```

## Volume Management

Workspaces are stored as Docker volumes:

```bash
# List volumes
docker volume ls --filter label=sandkasten.workspace=true

# Inspect volume
docker volume inspect sandkasten-ws-user123-project

# Manual cleanup
docker volume rm sandkasten-ws-user123-project
```

## Limitations

- **Size:** No built-in size limits (use Docker storage drivers)
- **Sharing:** No access control (any session with workspace_id can access)
- **Backup:** Use Docker volume backup tools
- **Migration:** Requires Docker volume export/import

## Best Practices

1. **Use descriptive IDs**
   ```python
   workspace_id = f"{user_id}-{project_name}-{timestamp}"
   ```

2. **Clean up unused workspaces**
   ```python
   # Implement workspace cleanup policy
   old_workspaces = await client.list_workspaces()
   for ws in old_workspaces:
       if is_too_old(ws):
           await client.delete_workspace(ws['id'])
   ```

3. **Handle conflicts**
   ```python
   # Multiple sessions can use same workspace
   # Implement locking if needed
   ```

4. **Monitor size**
   ```bash
   # Check workspace sizes
   docker system df -v | grep sandkasten-ws
   ```

## Troubleshooting

### "Workspace not enabled"

Enable in config:
```yaml
workspace:
  enabled: true
```

### Volume not found

Workspace is auto-created on first session use. If deleted manually via Docker, it will be recreated.

### Volume too large

```bash
# Check size
docker volume inspect sandkasten-ws-myworkspace | jq '.[0].Options.size'

# Clean up
docker exec -it <container> du -sh /workspace/*
```
