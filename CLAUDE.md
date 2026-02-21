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
# Build and run with Task (foreground)
task run

# Foreground (requires root for namespace operations)
sudo ./bin/sandkasten --config sandkasten.yaml

# Background / detached (like Docker daemon)
sudo ./bin/sandkasten daemon -d --config sandkasten.yaml

# List sessions (like docker ps)
./bin/sandkasten ps

# Environment variables override config
export SANDKASTEN_API_KEY="sk-test"
sudo ./bin/sandkasten
```

When running with `-d`/`--detach`, the daemon writes its PID to `<data_dir>/run/sandkasten.pid`.

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
- `internal/pool/` - Pre-warmed session pool (optional, when `pool.enabled`)
- `internal/reaper/` - TTL cleanup + reconciliation (skips `pool_idle` sessions)
- `internal/config/` - YAML + env var config
- `internal/web/` - Web dashboard handlers
- `protocol/` - Shared request/response types

