# Sandkasten

**Stateful sandbox runtime for AI agents** — exec, read, write tools with persistent shell sessions inside Docker containers.

---

## What is this?

Sandkasten gives your AI agent a **real Linux environment** where:
- Shell state persists (`cd`, env vars, background processes)
- Files persist within a session
- Three simple tools: `exec`, `read`, `write`
- Safe by default: no network, read-only rootfs, resource limits

Perfect for coding assistants, data analysis agents, CI/CD automation, or any LLM that needs to run commands and edit files.

---

## Quick Start

### Prerequisites
- Docker (20.10+) and Docker Compose (optional)
- Go 1.24+ (for building from source)
- Node.js 18+ (optional, for TS SDK)

### Option 1: Docker Compose (Recommended)

One-command setup:

```bash
./quickstart.sh
```

Or manually:

```bash
# Build sandbox images
make images

# Start daemon
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop
docker-compose down
```

The daemon runs at `http://localhost:8080` with API key `sk-sandbox-dev` (change in `.env`).

### Option 2: Build from Source

```bash
# Build runner binary, daemon, and Docker images
make build

# Start the daemon
./bin/sandkasten
```

By default, the daemon listens on `127.0.0.1:8080` with **no authentication** (dev mode).

### Create a session

```bash
# Create a Python sandbox
curl -X POST http://localhost:8080/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"image": "sandbox-runtime:python"}'

# Response:
# {"id":"a1b2c3d4","image":"sandbox-runtime:python","status":"running","cwd":"/workspace",...}
```

### Run commands

```bash
SESSION_ID="a1b2c3d4"

# Execute a command
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd":"python3 --version"}'

# Write a file
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/fs/write \
  -H 'Content-Type: application/json' \
  -d '{"path":"hello.py","text":"print(\"hello world\")"}'

# Execute it
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd":"python3 hello.py"}'

# Read a file
curl "http://localhost:8080/v1/sessions/$SESSION_ID/fs/read?path=hello.py"
```

### Clean up

```bash
# Destroy the session
curl -X DELETE http://localhost:8080/v1/sessions/$SESSION_ID
```

---

## Configuration

Create `sandkasten.yaml`:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-your-secret-key"  # Require auth
db_path: "./sandkasten.db"
session_ttl_seconds: 1800  # 30 minutes

allowed_images:
  - "sandbox-runtime:base"
  - "sandbox-runtime:python"
  - "sandbox-runtime:node"

defaults:
  cpu_limit: 1.0          # CPU cores
  mem_limit_mb: 512       # RAM
  pids_limit: 256
  max_exec_timeout_ms: 120000
  network_mode: "none"    # or "full"
  readonly_rootfs: true
```

Run with config:

```bash
./bin/sandkasten --config sandkasten.yaml
```

Environment variables override YAML:

```bash
export SANDKASTEN_API_KEY="sk-sandbox-your-secret-key"
export SANDKASTEN_LISTEN="0.0.0.0:8080"
./bin/sandkasten
```

---

## API Reference

### Authentication

If `api_key` is set, include it in requests:

```bash
curl -H "Authorization: Bearer sk-sandbox-your-secret-key" ...
```

### Endpoints

#### `POST /v1/sessions`
Create a new sandbox session.

**Request:**
```json
{
  "image": "sandbox-runtime:python",
  "ttl_seconds": 1800
}
```

**Response:**
```json
{
  "id": "a1b2c3d4",
  "image": "sandbox-runtime:python",
  "status": "running",
  "cwd": "/workspace",
  "created_at": "2026-02-07T22:00:00Z",
  "expires_at": "2026-02-07T22:30:00Z"
}
```

#### `POST /v1/sessions/{id}/exec`
Execute a shell command.

**Request:**
```json
{
  "cmd": "ls -la",
  "timeout_ms": 30000
}
```

**Response:**
```json
{
  "exit_code": 0,
  "cwd": "/workspace",
  "output": "total 8\ndrwxr-xr-x ...",
  "truncated": false,
  "duration_ms": 42
}
```

#### `POST /v1/sessions/{id}/fs/write`
Write a file.

**Request (text):**
```json
{
  "path": "/workspace/hello.py",
  "text": "print('hello')"
}
```

**Request (binary):**
```json
{
  "path": "/workspace/data.bin",
  "content_base64": "SGVsbG8gd29ybGQ="
}
```

#### `GET /v1/sessions/{id}/fs/read?path=...`
Read a file (returns base64-encoded content).

**Response:**
```json
{
  "path": "/workspace/hello.py",
  "content_base64": "cHJpbnQoJ2hlbGxvJyk=",
  "truncated": false
}
```

#### `DELETE /v1/sessions/{id}`
Destroy a session and clean up resources.

#### `GET /v1/sessions`
List all sessions (admin endpoint).

---

## TypeScript SDK

Install:

```bash
cd sdk
npm install
npm run build
```

Use in your project:

```typescript
import { SandboxClient } from "@sandkasten/sdk";

const client = new SandboxClient({
  baseUrl: "http://localhost:8080",
  apiKey: "sk-sandbox-your-secret-key", // optional
});

const session = await client.createSession({
  image: "sandbox-runtime:python",
});

const result = await session.exec("python3 --version");
console.log(result.output);

await session.write("hello.py", "print('hello world')");
const content = await session.readText("hello.py");

await session.destroy();
```

---

## Integration with AI Frameworks

### OpenAI Agents SDK

See `examples/openai-agents/main.py`:

```python
from agents import Agent, Runner, function_tool
import httpx

# Create sandkasten client
http = httpx.AsyncClient(base_url="http://localhost:8080")

@function_tool
async def exec(cmd: str) -> str:
    """Execute a shell command in the sandbox."""
    resp = await http.post(f"/v1/sessions/{session_id}/exec",
                           json={"cmd": cmd})
    data = resp.json()
    return f"exit={data['exit_code']}\n{data['output']}"

@function_tool
async def write_file(path: str, content: str) -> str:
    """Write a file in the sandbox."""
    await http.post(f"/v1/sessions/{session_id}/fs/write",
                    json={"path": path, "text": content})
    return f"wrote {path}"

agent = Agent(
    name="coding-assistant",
    instructions="You have a Linux sandbox. Use exec and write_file.",
    tools=[exec, write_file],
)

result = await Runner.run(agent, "Write and run a Python script...")
```

### Vercel AI SDK / LangChain / Claude

The pattern is identical:
1. Create a session once per conversation
2. Define `exec`, `write`, `read` as tools that call the HTTP API
3. Pass those tools to your agent
4. Destroy the session when done

The session persists shell state and files across all tool calls.

---

## Architecture

```
┌─────────────────────────────────────────┐
│  Your Agent Platform (OpenAI, Claude)  │
│         ┌──────────────────┐            │
│         │  exec/read/write │ ◄───── LLM calls tools
│         └────────┬─────────┘            │
└──────────────────┼──────────────────────┘
                   │ HTTP
          ┌────────▼─────────┐
          │  Sandkasten      │
          │  Daemon (Go)     │
          │  ┌─────────────┐ │
          │  │ Session Mgr │ │
          │  │ SQLite DB   │ │
          │  └──────┬──────┘ │
          └─────────┼────────┘
                    │ Docker API
       ┌────────────▼──────────────┐
       │  Docker Container         │
       │  ┌─────────────────────┐  │
       │  │ Runner (PID 1)      │  │
       │  │  ├─ bash PTY        │  │
       │  │  └─ unix socket     │  │
       │  └─────────────────────┘  │
       │  /workspace (volume)      │
       └───────────────────────────┘
```

### Key Components

- **Runner (`cmd/runner`)**: Tiny Go binary inside containers. Owns the bash PTY, exposes a Unix socket, handles exec/read/write requests.
- **Daemon (`cmd/sandkasten`)**: HTTP API, session management, Docker orchestration, SQLite persistence.
- **Reaper**: Background goroutine that garbage-collects expired sessions.
- **SDK**: TypeScript client library (zero dependencies except `fetch`).

### Why is exec stateful?

Each container runs **one persistent bash shell**. All `exec` calls are serialized and written to that shell's PTY. This means:
- `cd /foo` persists
- `export VAR=x` persists
- `python -m http.server &` runs in the background
- Output is captured via sentinel markers

---

## Sandbox Images

### Base
```dockerfile
FROM ubuntu:24.04
# bash, coreutils, runner binary
```

### Python
```dockerfile
FROM sandbox-runtime:base
# + python3, pip
```

### Node
```dockerfile
FROM sandbox-runtime:base
# + node 22, npm
```

Build your own:
```dockerfile
FROM sandbox-runtime:base
RUN apt-get update && apt-get install -y gcc make
```

Rebuild:
```bash
make images
```

---

## Security

### Default hardening (all containers):
- Read-only root filesystem
- No network (`--network none`)
- All Linux capabilities dropped
- `no-new-privileges`
- CPU, memory, PID limits enforced
- Non-root user (`sandbox`)

### Isolation model:
- **Sandkasten is single-tenant** (one daemon = one trust boundary)
- Sessions are isolated from each other (separate containers/volumes)
- The daemon host never exposes Docker socket to containers

### Deployment recommendations:
- Run the daemon behind a reverse proxy (Nginx, Traefik)
- Use TLS
- Set `api_key` and rotate regularly
- For production multi-tenant: add JWT auth + per-tenant quotas (Phase 2)

---

## Operational Notes

### Lifecycle
- Sessions auto-expire after `session_ttl_seconds` (default 30 min)
- Each `exec`/`read`/`write` extends the TTL
- Expired sessions are garbage-collected every 30s
- On daemon restart, reconciliation syncs DB ↔ Docker state

### Persistence
- SQLite DB (`sandkasten.db`) holds session metadata
- Docker volumes hold workspace files
- Destroying a session removes the container + volume

### Logs
- Daemon logs to stdout (structured slog)
- Use `docker logs` to inspect individual containers

### Monitoring
- Health check: `GET /healthz`
- List sessions: `GET /v1/sessions` (requires auth)

---

## Roadmap

### Phase 1 (MVP) — **Done**
- [x] Stateful exec via persistent PTY
- [x] Read/write file operations
- [x] Single API key auth
- [x] TTL + garbage collection
- [x] TypeScript SDK
- [x] OpenAI Agents SDK example

### Phase 2
- [ ] JWT authentication
- [ ] Streaming exec output (SSE)
- [ ] Persistent workspaces (cross-session)
- [ ] Metrics (Prometheus)
- [ ] Multi-tenancy (tenant_id, quotas)

### Phase 3
- [ ] Restricted egress networking
- [ ] Seccomp profiles
- [ ] Warm session pools

---

## Troubleshooting

### `docker: command not found`
Install Docker and ensure the daemon is running:
```bash
sudo systemctl start docker
docker ps
```

### `go build` fails with "redeclared" errors
Your Go toolchain is broken. Install Go 1.24.x:
```bash
wget https://go.dev/dl/go1.24.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.5.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH
```

### Session creation hangs
Check runner logs:
```bash
docker logs sandkasten-<session_id>
```

Ensure the base image built correctly:
```bash
docker images | grep sandbox-runtime
```

### "container not found" errors
The container was removed outside of sandkasten. The reconciliation loop (runs every 30s) will mark the session as `crashed`.

---

## Contributing

This is a personal/experimental project. Issues and PRs welcome, but no guarantees on response time.

---

## License

MIT
