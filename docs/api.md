# API Reference

Complete HTTP API documentation for Sandkasten.

**CLI:** List sessions with `./bin/sandkasten ps`. Run the daemon in the background with `./bin/sandkasten daemon -d`; stop it with `sudo ./bin/sandkasten stop` (or **Ctrl+C** if in foreground). Validate security baseline with `./bin/sandkasten security --config sandkasten.yaml`.

## Authentication

All API endpoints (except `/healthz` and web UI) require authentication:

```http
Authorization: Bearer <api_key>
```

## Base URL

Default: `http://localhost:8080`

## Sessions

### Create Session

```http
POST /v1/sessions
```

**Request:**
```json
{
  "image": "python",
  "ttl_seconds": 3600,
  "workspace_id": "user123-project"
}
```

**Response:**
```json
{
  "id": "abc123def456",
  "image": "python",
  "status": "running",
  "cwd": "/workspace",
  "workspace_id": "user123-project",
  "created_at": "2026-02-08T10:00:00Z",
  "expires_at": "2026-02-08T11:00:00Z"
}
```

### Get Session

```http
GET /v1/sessions/{id}
```

**Response:** Same as create session

### List Sessions

```http
GET /v1/sessions
```

**Response:**
```json
[
  {
    "id": "abc123",
    "image": "python",
    "status": "running",
    ...
  }
]
```

### Destroy Session

```http
DELETE /v1/sessions/{id}
```

**Response:**
```json
{"ok": true}
```

### Session Stats

```http
GET /v1/sessions/{id}/stats
```

**Response:**
```json
{
  "memory_bytes": 1239040,
  "memory_limit": 536870912,
  "cpu_usage_usec": 10442
}
```

## Execution

### Execute Command (Blocking)

```http
POST /v1/sessions/{id}/exec
```

**Request:**
```json
{
  "cmd": "python3 -c 'print(42)'",
  "timeout_ms": 30000
}
```

**Response:**
```json
{
  "exit_code": 0,
  "cwd": "/workspace",
  "output": "42\n",
  "truncated": false,
  "duration_ms": 42
}
```

**Notes:**
- Shell is persistent (cd, env vars, background processes persist)
- Output is combined stdout+stderr
- Truncated after 1MB
- Returns when command completes
- Large commands are supported: commands over 16 KiB are staged as a temporary script in `/workspace/.sandkasten/` and then executed via a short command
- Maximum `cmd` size is 1 MiB; larger payloads return `400 INVALID_REQUEST` with guidance to use `/fs/write`

### Execute Command (Streaming)

```http
POST /v1/sessions/{id}/exec/stream
```

**Request:** Same as blocking exec

**Response:** Server-Sent Events (SSE)

```
event: chunk
data: {"chunk":"Hello\n","timestamp":1707390000000}

event: chunk
data: {"chunk":"World\n","timestamp":1707390001000}

event: done
data: {"exit_code":0,"cwd":"/workspace","duration_ms":1234}
```

**Events:**
- `chunk` - Output chunk with timestamp
- `done` - Command completed
- `error` - Error occurred

**Notes:**
- Real-time output for long commands
- Same persistent shell semantics
- Client must support SSE
- Large commands are supported with the same staging behavior as blocking exec (inline threshold 16 KiB, API limit 1 MiB)

See [Streaming Guide](./features/streaming.md) for details.

## Filesystem

### Write File

```http
POST /v1/sessions/{id}/fs/write
```

**Request:**
```json
{
  "path": "/workspace/hello.py",
  "content_base64": "cHJpbnQoJ2hlbGxvJyk="
}
```

Or with text:
```json
{
  "path": "/workspace/hello.py",
  "text": "print('hello')"
}
```

**Response:**
```json
{"ok": true}
```

### Read File

```http
GET /v1/sessions/{id}/fs/read?path=/workspace/hello.py
```

**Query Parameters:**
- `path` (required) - File path
- `max_bytes` (optional) - Max bytes to read

**Response:**
```json
{
  "path": "/workspace/hello.py",
  "content_base64": "cHJpbnQoJ2hlbGxvJyk=",
  "truncated": false
}
```

## Workspaces

### List Workspaces

```http
GET /v1/workspaces
```

**Response:**
```json
{
  "workspaces": [
    {
      "id": "user123-project",
      "created_at": "2026-02-08T10:00:00Z",
      "labels": {
        "sandkasten.workspace_id": "user123-project"
      }
    }
  ]
}
```

### Delete Workspace

```http
DELETE /v1/workspaces/{id}
```

**Response:**
```json
{"ok": true}
```

**Note:** Destroys all data in the workspace permanently.

## Health Check

### Health Check

```http
GET /healthz
```

**Response:**
```json
{"status": "ok"}
```

**Note:** No authentication required.

## Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created (session) |
| 400 | Bad request (invalid JSON, missing params) |
| 401 | Unauthorized (invalid API key) |
| 404 | Not found (session doesn't exist) |
| 500 | Internal server error |

## Error Format

```json
{
  "error": "session not found: abc123"
}
```

## Rate Limits

No rate limits by default. Implement in reverse proxy if needed.

## Examples

### cURL

```bash
API_KEY="sk-sandbox-quickstart"
BASE_URL="http://localhost:8080"

# Create session
SESSION=$(curl -s -X POST $BASE_URL/v1/sessions \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"image":"python"}' | jq -r .id)

# Execute
curl -X POST $BASE_URL/v1/sessions/$SESSION/exec \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"cmd":"echo hello"}'

# Write file
echo -n "print('hello')" | base64 | \
  jq -R '{path:"/workspace/test.py",content_base64:.}' | \
  curl -X POST $BASE_URL/v1/sessions/$SESSION/fs/write \
    -H "Authorization: Bearer $API_KEY" \
    -d @-

# Read file
curl "$BASE_URL/v1/sessions/$SESSION/fs/read?path=/workspace/test.py" \
  -H "Authorization: Bearer $API_KEY" | \
  jq -r .content_base64 | base64 -d

# Destroy
curl -X DELETE $BASE_URL/v1/sessions/$SESSION \
  -H "Authorization: Bearer $API_KEY"
```

### Python

See [SDK documentation](../sdk/python/README.md)

### TypeScript

See [SDK documentation](../sdk/README.md)
