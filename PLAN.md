# Sandkasten — Implementation Plan (Phase 1 MVP)

## Scope

Stripped multi-tenancy. Single API key auth. Focus: working stateful sandbox
runtime that an agent platform can integrate via TS SDK or raw HTTP.

---

## Project Structure

```
sandkasten/
├── cmd/
│   ├── sandkasten/        # daemon entrypoint
│   │   └── main.go
│   └── runner/            # runner binary (lives inside containers)
│       └── main.go
├── internal/
│   ├── api/               # HTTP handlers + router + middleware
│   ├── docker/            # Docker Engine API wrapper
│   ├── store/             # SQLite session store + migrations
│   ├── session/           # Session manager (orchestration layer)
│   ├── reaper/            # TTL/GC goroutine
│   └── config/            # Config loading (env + yaml)
├── protocol/              # Runner ↔ daemon protocol types (shared)
├── images/
│   ├── base/Dockerfile
│   ├── python/Dockerfile
│   └── node/Dockerfile
├── sdk/                   # TypeScript SDK (separate npm package)
│   └── src/
├── examples/
│   └── openai-agents/     # End-user integration example
└── PLAN.md
```

---

## Step 1 — Runner binary (`cmd/runner`)

The runner is a small static Go binary that runs as PID 1 inside every
sandbox container. It is the only thing the daemon talks to.

### What it does
- Allocates a PTY, starts `bash -l` (fallback `sh`)
- Reads JSON-line requests from stdin
- For `exec`: writes command to PTY wrapped in sentinel markers,
  captures output until end sentinel, returns JSON-line response
- For `read`/`write`: operates directly on the filesystem
- Signals readiness on stdout with a `{"ready":true}` line

### Protocol (JSON lines over stdin/stdout)

**Request types:**
```jsonc
// exec
{"id":"abc","type":"exec","cmd":"ls -la","timeout_ms":30000}
// write
{"id":"def","type":"write","path":"/workspace/foo.py","content_base64":"..."}
// read
{"id":"ghi","type":"read","path":"/workspace/foo.py","max_bytes":1048576}
```

**Response:**
```jsonc
// exec response
{"id":"abc","type":"exec","exit_code":0,"cwd":"/workspace","output":"...","truncated":false,"duration_ms":42}
// write response
{"id":"def","type":"write","ok":true}
// read response
{"id":"ghi","type":"read","content_base64":"...","truncated":false}
// error
{"id":"xxx","type":"error","message":"timeout exceeded"}
```

### Sentinel mechanism (exec)
For each exec request `id`:
1. Write to PTY: `__SANDKASTEN_BEGIN__:<id>\n`
2. Write the user command + `\n`
3. Write: `printf '\n__SANDKASTEN_END__:%s:%d:%s\n' '<id>' "$?" "$PWD"\n`
4. Read PTY output, buffer between BEGIN and END markers
5. Parse exit code + cwd from END marker

### Build
- `CGO_ENABLED=0 GOOS=linux go build -o runner ./cmd/runner`
- Static binary, copied into images

---

## Step 2 — Sandbox base image (`images/base`)

```dockerfile
FROM ubuntu:24.04
RUN useradd -m -s /bin/bash sandbox && \
    apt-get update && apt-get install -y --no-install-recommends \
    bash coreutils procps ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY runner /usr/local/bin/runner
RUN chmod +x /usr/local/bin/runner && mkdir /workspace && chown sandbox:sandbox /workspace
USER sandbox
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/runner"]
```

Python and Node images extend base with their respective runtimes.

---

## Step 3 — Daemon core (`cmd/sandkasten` + `internal/`)

### Config (`internal/config`)
Loaded from env vars + optional `sandkasten.yaml`:
```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-..."          # single key for MVP
default_image: "sandbox-runtime:base"
allowed_images:
  - "sandbox-runtime:base"
  - "sandbox-runtime:python"
  - "sandbox-runtime:node"
db_path: "./sandkasten.db"
session_ttl_seconds: 1800
defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  max_exec_timeout_ms: 120000
  network_mode: "none"
  readonly_rootfs: true
```

### SQLite store (`internal/store`)
Single table for MVP:
```sql
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,
  image         TEXT NOT NULL,
  container_id  TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'running', -- running | expired | destroyed
  cwd           TEXT NOT NULL DEFAULT '/workspace',
  created_at    TEXT NOT NULL,
  expires_at    TEXT NOT NULL,
  last_activity TEXT NOT NULL
);
```

### Docker wrapper (`internal/docker`)
Thin wrapper around `github.com/docker/docker/client`:
- `CreateContainer(sessionID, image, limits)` — creates + starts container
  with hardening defaults (read-only rootfs, drop all caps, no-new-privileges,
  pids/mem/cpu limits, tmpfs /tmp, network none, labels)
- `ExecRunner(containerID, request) -> response` — runs
  `docker exec -i <container> /usr/local/bin/runner` and sends one JSON-line
  request, reads one JSON-line response
- `RemoveContainer(containerID)` — force remove
- `ListSandboxContainers()` — list by label for reconciliation

**Key design decision**: each `exec` call does a fresh `docker exec` into the
runner. The runner process is long-lived (PID 1), but the daemon connects to it
per-request. This makes the daemon restart-safe — no persistent connections.

Wait — this conflicts with the "persistent shell" requirement. If runner is
PID 1 and we docker-exec into it per request, we need the runner to maintain
the PTY/shell across requests. The approach:

- Runner starts bash PTY on startup and keeps it alive
- Runner listens on a Unix socket (`/tmp/runner.sock`) inside the container
- Daemon does `docker exec` to run a tiny **client** that connects to the
  socket, sends one request, gets one response, exits
- OR (simpler): runner listens on stdin but we use `docker attach` (messy)
- OR (simplest for MVP): runner accepts connections on a TCP port inside the
  container (localhost only, no network exposure since network=none)

**Chosen approach for MVP:**
Runner listens on a Unix socket `/run/runner.sock`. Daemon interaction:
```
docker exec -i <container> /usr/local/bin/runner-client <json-request>
```
Where `runner-client` is a second tiny binary (or runner itself with a flag:
`runner --client '{"id":"...","type":"exec",...}'`) that connects to the socket,
sends the request, reads the response, prints it to stdout.

This way:
- Runner (PID 1) owns the persistent bash PTY
- Each daemon request is a clean docker exec
- Daemon restarts don't lose shell state
- No need for persistent connections

### Session manager (`internal/session`)
Orchestration layer:
- `Create(image) -> Session` — validate image, create container, insert DB row
- `Exec(sessionID, cmd, timeoutMs) -> ExecResult` — load session, check not
  expired, acquire per-session mutex, docker exec runner-client, update
  last_activity + extend lease, return result
- `Read(sessionID, path, maxBytes) -> bytes`
- `Write(sessionID, path, content)`
- `Destroy(sessionID)` — remove container, update DB
- `Get(sessionID) -> Session`
- Per-session mutex map to serialize exec calls

### HTTP API (`internal/api`)
Router: `net/http` + `chi` (lightweight)

Middleware:
- API key check: `Authorization: Bearer <key>` must match configured key
- Request ID injection
- Logging

Endpoints:
```
POST   /v1/sessions                    -> create session
GET    /v1/sessions/{id}               -> get session info
POST   /v1/sessions/{id}/exec          -> execute command
POST   /v1/sessions/{id}/fs/write      -> write file
GET    /v1/sessions/{id}/fs/read       -> read file
DELETE /v1/sessions/{id}               -> destroy session
GET    /v1/sessions                    -> list sessions (admin/debug)
```

---

## Step 4 — Reaper & reconciliation (`internal/reaper`)

### Reaper goroutine
- Ticks every 30s
- Finds sessions where `expires_at < now` and `status = running`
- Destroys container, marks session `expired`

### Reconciliation (on startup)
- List all containers with label `sandkasten.session_id`
- For each: if no matching DB row or DB row is destroyed/expired → remove container
- For each DB row marked `running`: if no container exists → mark `crashed`
- Extend lease on activity: each exec/read/write bumps `expires_at`

---

## Step 5 — TS SDK (`sdk/`)

Minimal, zero-dependency (aside from fetch) TypeScript package.

```ts
class SandboxClient {
  constructor(opts: { baseUrl: string; apiKey: string })
  createSession(opts?: { image?: string; ttlSeconds?: number }): Promise<Session>
  getSession(id: string): Promise<Session>
  listSessions(): Promise<SessionInfo[]>
}

class Session {
  readonly id: string
  exec(cmd: string, opts?: { timeoutMs?: number }): Promise<ExecResult>
  write(path: string, content: string | Uint8Array): Promise<void>
  read(path: string, opts?: { maxBytes?: number }): Promise<Uint8Array>
  destroy(): Promise<void>
}

interface ExecResult {
  exitCode: number
  cwd: string
  output: string
  truncated: boolean
  durationMs: number
}
```

---

## Step 6 — End-user integration example (`examples/openai-agents/`)

Shows how someone self-hosting sandkasten wires it into the OpenAI Agents SDK.

```python
from agents import Agent, Runner, function_tool
from sandkasten import SandboxClient  # thin Python wrapper or raw HTTP

client = SandboxClient(base_url="http://localhost:8080", api_key="sk-sandbox-...")

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox. Stateful: cd, env, bg jobs persist."""
    result = await session.exec(cmd, timeout_ms=timeout_ms)
    return f"exit={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

@function_tool
async def write_file(path: str, content: str) -> str:
    """Write content to a file in the sandbox workspace."""
    await session.write(path, content)
    return f"wrote {path}"

@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox workspace."""
    data = await session.read(path)
    return data.decode()

agent = Agent(
    name="coding-assistant",
    instructions="You have a Linux sandbox. Use exec, write_file, read_file.",
    tools=[exec, write_file, read_file],
)

async def main():
    global session
    session = await client.create_session(image="sandbox-runtime:python")
    try:
        result = await Runner.run(agent, "Write a Python script that prints fibonacci numbers and run it")
        print(result.final_output)
    finally:
        await session.destroy()
```

This pattern works identically for:
- **Vercel AI SDK** (define tools as `tool()` functions)
- **LangChain** (define as `@tool` decorated functions)
- **Claude tool_use** (define in the tools array, call via TS SDK)

The key insight: sandkasten is **agent-framework-agnostic**. The SDK gives you
`exec/read/write` — you wrap them as tools in whatever framework you use.

---

## Implementation Order

| # | What | Deliverable | Depends on |
|---|------|-------------|------------|
| 1 | Runner binary | `cmd/runner/main.go` — PTY, sentinel, unix socket, request handling | — |
| 2 | Runner client mode | `runner --client '{...}'` flag | 1 |
| 3 | Protocol types | `protocol/` package (shared between runner and daemon) | — |
| 4 | Base Dockerfile | `images/base/Dockerfile` + build script | 1 |
| 5 | Config + store | `internal/config`, `internal/store` (SQLite) | — |
| 6 | Docker wrapper | `internal/docker` (create, exec, remove, list) | 3 |
| 7 | Session manager | `internal/session` (orchestration, locking) | 5, 6 |
| 8 | HTTP API | `internal/api` (routes, middleware, handlers) | 7 |
| 9 | Reaper | `internal/reaper` (GC + reconciliation) | 5, 6 |
| 10 | Daemon main | `cmd/sandkasten/main.go` (wires everything) | 5–9 |
| 11 | Integration test | End-to-end: create session, exec, read, write, destroy | 1–10 |
| 12 | TS SDK | `sdk/` package | 8 (just needs API spec) |
| 13 | Example | `examples/openai-agents/` | 12 |

Steps 1–3 can be parallel. Steps 5–6 can be parallel. Step 11 is the validation gate.

---

## What's deferred (not in this plan)

- Multi-tenancy (tenant_id, subject_id, per-tenant policies)
- JWT auth
- Streaming/SSE exec output
- Persisted workspaces
- Restricted egress networking
- Warm session pools
- Seccomp profiles
- Metrics/observability beyond logs
