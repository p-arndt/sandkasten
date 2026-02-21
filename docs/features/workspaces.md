# Persistent Workspaces

Directory-backed storage that survives session destruction.

## Overview

By default, sessions are **ephemeral** - files are lost when the session is destroyed. With persistent workspaces, files are stored in directories on the host that survive session destruction.

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
└─ destroy → files persist in /var/lib/sandkasten/workspaces/user123-project/

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
    "image": "python",
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

### Browse and Write Workspace Files

```bash
# List files
curl http://localhost:8080/v1/workspaces/user123-project/fs \
  -H "Authorization: Bearer sk-..."

# Write file (creates workspace if needed)
curl -X POST http://localhost:8080/v1/workspaces/user123-project/fs/write \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"path":"code.py","text":"print(\"hello\")"}'

# Upload file (multipart, for binary or large files)
curl -X POST http://localhost:8080/v1/workspaces/user123-project/fs/upload \
  -H "Authorization: Bearer sk-..." \
  -F "file=@./data.csv"

# Read file
curl "http://localhost:8080/v1/workspaces/user123-project/fs/read?path=/code.py" \
  -H "Authorization: Bearer sk-..."
```

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

## Filesystem Layout

Workspaces are stored as directories:

```
/var/lib/sandkasten/workspaces/
├── user123-project/
│   ├── code.py
│   └── data.csv
├── user456-analysis/
│   └── report.pdf
└── temp-abc123/
    └── ...
```

### Manual Access

```bash
# List workspaces
ls /var/lib/sandkasten/workspaces/

# Inspect workspace
ls -la /var/lib/sandkasten/workspaces/user123-project/

# Backup workspace
tar -czf backup.tar.gz -C /var/lib/sandkasten/workspaces user123-project/

# Copy to/from workspace
cp local_file.txt /var/lib/sandkasten/workspaces/user123-project/
```

## Limitations

- **Size:** No built-in size limits (use filesystem quotas)
- **Sharing:** No access control (any session with workspace_id can access)
- **Backup:** Use standard filesystem backup tools
- **Concurrency:** Multiple sessions using same workspace may conflict

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
   # Implement application-level locking if needed
   ```

4. **Monitor size**
   ```bash
   # Check workspace sizes
   du -sh /var/lib/sandkasten/workspaces/*
   ```

## Troubleshooting

### "Workspace not enabled"

Enable in config:
```yaml
workspace:
  enabled: true
```

### Workspace not found

Workspace is auto-created on first session use. Check the data directory:
```bash
ls /var/lib/sandkasten/workspaces/
```

### Directory too large

```bash
# Check size
du -sh /var/lib/sandkasten/workspaces/myworkspace

# Clean up large files
du -sh /var/lib/sandkasten/workspaces/myworkspace/*
```

### Permission denied

Ensure daemon has write access:
```bash
sudo chown -R root:root /var/lib/sandkasten/workspaces/
sudo chmod -R 755 /var/lib/sandkasten/workspaces/
```
