# CLAUDE.md

This file provides guidance to Claude (claude.ai/code) when working with code in this repository.

## Project Overview

Sandkasten is a self-hosted sandbox runtime for AI agents. It provides stateful Linux sandboxes with persistent shell sessions, file operations, and workspace management via HTTP API. Written in Go with TypeScript and Python SDKs.

**Key characteristics:**
- No Docker required - uses native Linux sandboxing
- Single API key auth (no multi-tenancy in MVP)
- Stateful bash sessions that persist across requests
- Isolation via Linux namespaces, cgroups v2, and overlayfs

## Requirements

- **Linux** only (kernel 5.11+) or WSL2
- cgroups v2 mounted at `/sys/fs/cgroup`
- overlayfs support
- Root or CAP_SYS_ADMIN for daemon

## Build Commands

```bash
# Build everything
task build

# Build individual components
task daemon      # Build daemon only
task runner      # Build runner binary (static Linux binary)

# Run daemon locally
task run

# Clean build artifacts
task clean
```

## Running the Daemon

```bash
# Build and run with Task
task run

# With config file (requires root for namespace operations)
sudo ./bin/sandkasten --config sandkasten.yaml

# Environment variables override config
export SANDKASTEN_API_KEY="sk-test"
sudo ./bin/sandkasten
```

## Image Management

Images are rootfs directories stored in `/var/lib/sandkasten/images/<name>/rootfs`.

```bash
# Import from tarball
sudo ./bin/imgbuilder import --name python --tar python.tar.gz

# List images
./bin/imgbuilder list

# Validate image
./bin/imgbuilder validate python
```

## Architecture

### Three-Layer Design

1. **Runner** (`cmd/runner/main.go`)
   - Runs as PID 1 inside sandboxes
   - Manages persistent PTY + bash shell
   - Listens on Unix socket `/run/sandkasten/runner.sock`
   - Handles exec/read/write requests via JSON-line protocol

2. **Daemon** (`cmd/sandkasten/main.go`)
   - HTTP API server
   - Orchestrates sandbox creation via Linux runtime driver
   - SQLite state persistence
   - Background reaper for expired sessions

3. **Runtime Driver** (`internal/runtime/linux/`)
   - Creates overlayfs mounts
   - Sets up namespaces (mount, pid, uts, ipc, net)
   - Configures cgroups v2 limits
   - Performs pivot_root into sandbox

### Communication Flow

```
HTTP Request → API Handler → Session Manager → Runtime Driver
                                                   ↓
                                    Unix socket: /var/lib/sandkasten/sessions/<id>/run/runner.sock
                                                   ↓
                                    Runner (PID 1) ← PTY → bash (persistent)
```

### Core Packages

- `internal/api/` - HTTP routes, middleware, handlers
- `internal/session/` - Session orchestration, per-session exec mutexes
- `internal/runtime/` - Runtime driver interface
- `internal/runtime/linux/` - Linux sandbox implementation
- `internal/store/` - SQLite persistence
- `internal/reaper/` - TTL cleanup + reconciliation
- `internal/config/` - YAML + env var config
- `internal/web/` - Web dashboard handlers
- `protocol/` - Shared request/response types

### Sandbox Isolation

Each sandbox uses:
- **Namespaces**: mount, pid, uts, ipc, network (optional)
- **cgroups v2**: cpu.max, memory.max, pids.max
- **overlayfs**: lower=image rootfs, upper=session writes
- **Capabilities**: All dropped via prctl
- **no_new_privs**: Prevents privilege escalation

### Session Lifecycle

1. Create: Validate image → create overlay → setup namespaces → start runner
2. Exec: Load session → acquire mutex → connect to socket → send request
3. Destroy: Kill process → remove cgroup → unmount overlay → cleanup dirs
4. Expiry: Reaper finds expired → destroys → marks status in DB

## Configuration

- Primary: `sandkasten.yaml` (pass with `--config` flag)
- All options have env var overrides (`SANDKASTEN_*`)

**Key settings:**
```yaml
listen: "127.0.0.1:8080"
api_key: "sk-test"
data_dir: "/var/lib/sandkasten"
default_image: "python"
defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  network_mode: "none"
```

## File Organization

```
cmd/
  sandkasten/     # Daemon entrypoint
  runner/         # Runner binary (PID 1 in sandboxes)
  imgbuilder/     # Image management tool

internal/
  api/            # HTTP server, routes, handlers
  session/        # Session manager with exec serialization
  runtime/        # Runtime driver interface
    linux/        # Linux implementation (namespaces, cgroups, overlayfs)
  store/          # SQLite store
  reaper/         # TTL cleanup
  config/         # Config loading
  web/            # Dashboard UI

protocol/         # Shared types (runner ↔ daemon)
```

## Common Development Patterns

### Adding a New API Endpoint

1. Define handler in `internal/api/handlers.go`
2. Add route in `internal/api/router.go`
3. Update SDK clients (`sdk/` and `sdk/python/`)
4. Document in `docs/api.md`

### Debugging Sandbox Issues

```bash
# Check cgroup status
cat /sys/fs/cgroup/sandkasten/<session_id>/cgroup.procs

# Check running sandboxes
ls /var/lib/sandkasten/sessions/

# Check runner socket
ls -la /var/lib/sandkasten/sessions/<id>/run/

# Manual cleanup
sudo umount -R /var/lib/sandkasten/sessions/<id>/mnt
sudo rm -rf /var/lib/sandkasten/sessions/<id>
```

## Important Constraints

1. **Linux only**: No macOS support. Use WSL2 on Windows.

2. **Data directory**: Must be on a filesystem supporting overlayfs (ext4, xfs). NOT NTFS in WSL2.

3. **Exec serialization**: Per-session mutex ensures exec commands run sequentially.

4. **Runner is restart-safe**: Daemon can restart without losing shell sessions because state lives in runner (PID 1).

5. **Output limits**: Exec output capped at 5MB (`protocol.MaxOutputBytes`), file reads at 10MB.

## Quick Reference

**Start daemon:**
```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

**Import image:**
```bash
docker create --name temp python:3.12-slim
docker export temp | gzip > /tmp/python.tar.gz
docker rm temp
sudo ./bin/imgbuilder import --name python --tar /tmp/python.tar.gz
```

**API endpoints:**
- `POST /v1/sessions` - Create session
- `POST /v1/sessions/{id}/exec` - Execute command
- `POST /v1/sessions/{id}/fs/write` - Write file
- `GET /v1/sessions/{id}/fs/read` - Read file
- `DELETE /v1/sessions/{id}` - Destroy session

## Documentation

- `README.md` - Project overview and quick start
- `docs/quickstart.md` - 5-minute getting started guide
- `docs/api.md` - Complete HTTP API reference
- `docs/configuration.md` - Config options and security
- `PLAN.md` - Implementation plan (historical reference)
