# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sandkasten is a self-hosted sandbox runtime for AI agents. It provides stateful Linux containers with persistent shell sessions, file operations, and workspace management via HTTP API. Written in Go with TypeScript and Python SDKs.

**Key characteristics:**
- Single API key auth (no multi-tenancy in MVP)
- Stateful bash sessions that persist across requests
- Runner binary (PID 1) communicates via Unix socket
- Container lifecycle managed by daemon with SQLite store

## Build Commands

We use [Task](https://taskfile.dev/) instead of Make for build automation.

```bash
# Build everything (runner binary, daemon, Docker images)
task build

# Build individual components
task runner      # Build runner binary (static Linux binary)
task daemon      # Build sandkasten daemon
task images      # Build all Docker images (base, python, node)
task image-base  # Build just base image (faster iteration)

# Run daemon locally
task run

# Clean build artifacts
task clean

# List all available tasks
task --list
```

**Important:** Go 1.25.7 on this system has broken runtime. Use `go vet ./...` for checking, but `task` commands handle builds correctly with proper flags.

## Running the Daemon

```bash
# Build and run with Task
task run

# With config file
./bin/sandkasten --config sandkasten.yaml

# Quick start with Docker Compose
cd quickstart/daemon && docker-compose up -d

# Environment variables override config
export SANDKASTEN_API_KEY="sk-test"
./bin/sandkasten
```

## Testing

```bash
# No Go test files exist yet
# Current validation is via end-to-end quickstart examples

# Test with Python SDK
cd quickstart/agent
uv run enhanced_agent.py
```

## Architecture

### Three-Layer Design

1. **Runner** (`cmd/runner/main.go`)
   - Runs as PID 1 inside containers
   - Manages persistent PTY + bash shell
   - Listens on Unix socket `/run/runner.sock`
   - Handles exec/read/write requests via JSON-line protocol
   - Two modes: server (PID 1) and client (`--client` flag for one-shot requests)

2. **Daemon** (`cmd/sandkasten/main.go`)
   - HTTP API server
   - Orchestrates Docker containers
   - SQLite state persistence
   - Background reaper for expired sessions
   - Manages workspace volumes and container pool

3. **SDK** (`sdk/`)
   - TypeScript SDK (zero dependencies): `SandboxClient` + `Session` classes
   - Python SDK: `sandkasten` package with async API

### Communication Flow

```
HTTP Request → API Handler → Session Manager → Docker Client
                                                   ↓
                                    docker exec runner --client '{json}'
                                                   ↓
                                    Runner (PID 1) ← Unix Socket → Runner Client
                                                   ↓
                                    PTY/bash (persistent shell state)
```

**Key design:** Each daemon request does `docker exec runner --client '{...}'` which connects to the long-running runner's Unix socket. This makes the daemon restart-safe—shell state lives in the container, not in daemon connections.

### Core Packages

- `internal/api/` - HTTP routes, middleware (auth, logging), handlers
- `internal/session/` - Session orchestration, per-session exec mutexes
- `internal/docker/` - Docker Engine API wrapper (create, exec, remove)
- `internal/store/` - SQLite persistence (sessions table)
- `internal/reaper/` - Background TTL/GC goroutine
- `internal/config/` - YAML + env var config loading
- `internal/pool/` - Pre-warmed container pool for instant session creation
- `internal/workspace/` - Persistent workspace volume management
- `internal/web/` - Web dashboard handlers
- `protocol/` - Shared request/response types between runner ↔ daemon

### Sentinel-Based Command Execution

Commands are wrapped with markers for reliable output capture:

```
__SANDKASTEN_BEGIN__:<request_id>
<user command>
printf '\n__SANDKASTEN_END__:<request_id>:%d:%s\n' "$?" "$PWD"
```

Runner reads PTY output between sentinels, extracts exit code and cwd from end marker.

### Session Lifecycle

1. Create: Validate image → acquire from pool OR create container → insert DB row
2. Exec: Load session → acquire per-session mutex → docker exec runner client → update last_activity
3. Read/Write: Direct filesystem operations via runner
4. Destroy: Remove container → update DB status → remove volume (if ephemeral)
5. Expiry: Reaper finds expired sessions → destroys container → marks DB row

## Configuration

- Primary: `sandkasten.yaml` (pass with `--config` flag)
- All options have env var overrides (`SANDKASTEN_*`)
- See `quickstart/daemon/sandkasten-full.yaml` for complete example
- See `docs/configuration.md` for full reference

**Key settings:**
- `listen`: Host:port binding (default `127.0.0.1:8080`)
- `api_key`: Single auth key (empty = open access)
- `default_image`: Default runtime (`sandbox-runtime:python`, `:node`, `:base`)
- `workspace.enabled`: Enable persistent workspaces
- `pool.enabled`: Enable pre-warmed container pool

## File Organization

```
cmd/
  sandkasten/     # Daemon entrypoint
  runner/         # Runner binary (PID 1 in containers)

internal/
  api/            # HTTP server, routes, handlers
  docker/         # Docker client wrapper
  session/        # Session manager with exec serialization
  store/          # SQLite store
  reaper/         # TTL/GC background task
  config/         # Config loading
  pool/           # Container pool manager
  workspace/      # Workspace volume manager
  web/            # Dashboard UI

protocol/         # Shared types (runner ↔ daemon)

images/
  base/           # Base Dockerfile + runner binary
  python/         # Python runtime extension
  node/           # Node.js runtime extension

sdk/              # TypeScript SDK
  python/         # Python SDK

quickstart/       # End-user examples
  agent/          # OpenAI Agents SDK examples
  daemon/         # Docker Compose + config examples

docs/             # Documentation
```

## Common Development Patterns

### Adding a New API Endpoint

1. Define handler in `internal/api/handlers.go`
2. Add route in `internal/api/router.go`
3. Update SDK clients (`sdk/` and `sdk/python/`)
4. Document in `docs/api.md`

### Adding Runner Request Type

1. Add request/response types in `protocol/protocol.go`
2. Implement handler in `cmd/runner/main.go` (`handleConn` switch)
3. Add corresponding method in `internal/docker/client.go` (`ExecRunner`)
4. Wire through `internal/session/manager.go`

### Container Security

All containers are hardened with:
- Read-only root filesystem (`readonly_rootfs: true`)
- All capabilities dropped (`CapDrop: ["ALL"]`)
- No new privileges (`SecurityOpt: ["no-new-privileges"]`)
- PID/CPU/memory limits
- Optional network isolation (`network_mode: "none"`)

Do not weaken these defaults without explicit justification.

## Important Constraints

1. **No tests yet**: Validation is manual via quickstart examples. Be careful with changes.

2. **Single API key**: No per-tenant auth. Sessions are not isolated by ownership.

3. **Exec serialization**: Per-session mutex ensures exec commands run sequentially. This is critical for shell statefulness (cd, env vars, etc.).

4. **Runner is restart-safe**: Daemon can restart without losing shell sessions because state lives in runner (PID 1) inside containers.

5. **Output limits**: Exec output capped at 5MB (`protocol.MaxOutputBytes`), file reads at 10MB (`protocol.DefaultMaxReadBytes`). Prevents memory issues.

6. **Workspace volumes**:
   - Ephemeral (default): `sandkasten-ws-<session_id>`, deleted with session
   - Persistent: `sandkasten-ws-<workspace_id>`, survives session destruction

## Quick Reference

**Start daemon in dev:**
```bash
task build && ./bin/sandkasten --config quickstart/daemon/sandkasten.yaml
# Or simply:
task run
```

**View dashboard:**
```
http://localhost:8080
```

**API endpoints:**
- `POST /v1/sessions` - Create session
- `POST /v1/sessions/{id}/exec` - Execute command
- `GET /v1/sessions/{id}/exec/stream` - Streaming exec (SSE)
- `POST /v1/sessions/{id}/fs/write` - Write file
- `GET /v1/sessions/{id}/fs/read` - Read file
- `DELETE /v1/sessions/{id}` - Destroy session
- `GET /v1/workspaces` - List workspaces
- `GET /v1/pool/status` - Pool status

**Docker image tags:**
- `sandbox-runtime:base` - Minimal (bash, coreutils)
- `sandbox-runtime:python` - Python 3 + pip + uv
- `sandbox-runtime:node` - Node.js 22 + npm

## Documentation

- `README.md` - Project overview and quick start
- `docs/quickstart.md` - 5-minute getting started guide
- `docs/api.md` - Complete HTTP API reference
- `docs/configuration.md` - Config options and security
- `docs/features/` - Workspaces, pooling, streaming
- `PLAN.md` - Original implementation plan (historical reference)
