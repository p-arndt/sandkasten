# Sandkasten

**Self-hosted sandbox runtime for AI agents.** Stateful Linux sandboxes with persistent shell, file operations, and workspace management — no Docker required.

```python
from sandkasten import SandboxClient

async with SandboxClient(base_url="...", api_key="...") as client:
    async with await client.create_session() as session:
        await session.write("hello.py", "print('Hello from sandbox!')")
        result = await session.exec("python3 hello.py")
        print(result.output)  # Hello from sandbox!
```

## Features

- ✅ **Stateful Sessions** - Persistent bash shell (cd, env vars, background processes)
- ✅ **No Docker** - Native Linux sandboxing with namespaces, cgroups, overlayfs
- ✅ **File Operations** - Read/write files in `/workspace`
- ✅ **Multiple Runtimes** - Python, Node.js, or custom images
- ✅ **Persistent Workspaces** - Directories that survive session destruction
- ✅ **Web Dashboard** - Monitor sessions, edit config
- ✅ **Python + TypeScript SDKs** - Clean async APIs
- ✅ **Agent-Ready** - Works with OpenAI Agents SDK, LangChain, etc.
- ✅ **WSL2 Support** - Run on Windows via WSL2

## Requirements

- **Linux** with kernel 5.11+ (or WSL2 with Ubuntu 22.04+)
- cgroups v2 mounted at `/sys/fs/cgroup`
- overlayfs support
- Go 1.24+ (for building)

> **Note:** macOS is not supported. Use a Linux VM or WSL2.

## Quick Start

### 1. Build

```bash
git clone https://github.com/yourusername/sandkasten
cd sandkasten
task build
```

### 2. Run Preflight + Initialize

```bash
# Check Linux requirements and hints
./bin/sandkasten doctor

# Create config + data dirs + pull default image
sudo ./bin/sandkasten init --config sandkasten.yaml
```

### 3. Start Daemon

```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

### 4. Run Example Agent

```bash
cd quickstart/agent
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

### 5. Open Dashboard

```
http://localhost:8080
```

## Documentation

- [Quickstart Guide](./docs/quickstart.md) - Get running in 5 minutes
- [Windows/WSL2 Setup](./docs/windows.md) - Detailed Windows instructions
- [API Reference](./docs/api.md) - Complete HTTP API docs
- [Configuration](./docs/configuration.md) - Config options and security
- [Architecture](#architecture) - How it works

## Architecture

Sandkasten uses native Linux sandboxing instead of containers:

```
┌──────────────────────────────────────────────────────┐
│                  Sandkasten Daemon                   │
├──────────────────────────────────────────────────────┤
│  HTTP API → Session Manager → Linux Runtime Driver   │
│     ↓            ↓                  ↓                │
│  Sessions    Workspaces         OverlayFS            │
│  (SQLite)    (bind mount)       + Namespaces         │
│                                  + Cgroups            │
└──────────────────────────────────────────────────────┘
          │
          ↓ (Unix socket)
┌──────────────────────────────────────────────────────┐
│                  Sandbox Process                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ Runner (PID 1)                                 │  │
│  │ ↓                                              │  │
│  │ bash -l (PTY) ← persistent shell state        │  │
│  └────────────────────────────────────────────────┘  │
│  /workspace/ ← bind mount from host                  │
│  (overlayfs: lower=image, upper=session)             │
└──────────────────────────────────────────────────────┘
```

### Isolation Mechanisms

Each sandbox is isolated using:

- **Mount namespace** - Private filesystem via overlayfs
- **PID namespace** - Runner becomes PID 1
- **UTS namespace** - Private hostname (`sk-<session_id>`)
- **IPC namespace** - Isolated IPC resources
- **Network namespace** (optional) - No network access
- **cgroups v2** - CPU, memory, and PID limits
- **Capabilities** - All capabilities dropped
- **no_new_privs** - Cannot gain new privileges

## Directory Layout

```
/var/lib/sandkasten/
├── images/
│   ├── base/
│   │   ├── rootfs/          # Read-only base filesystem
│   │   └── meta.json
│   ├── python/
│   │   ├── rootfs/
│   │   └── meta.json
│   └── node/
│       ├── rootfs/
│       └── meta.json
├── sessions/
│   └── <session_id>/
│       ├── upper/           # Overlay upper (writable layer)
│       ├── work/            # Overlay workdir
│       ├── mnt/             # Overlay mountpoint
│       ├── run/             # Bind mount for runner socket
│       │   └── runner.sock
│       └── state.json       # Runtime state
└── workspaces/
    └── <workspace_id>/      # Persistent workspace data
```

## Image Management

### Pull/Manage Images

```bash
# Pull directly from OCI registries (no Docker daemon)
sudo ./bin/sandkasten image pull --name python python:3.12-slim

# List available images
./bin/sandkasten image list

# Validate an image
./bin/sandkasten image validate python

# Delete an image
sudo ./bin/sandkasten image delete python
```

### Building Images

**Method 1: Pull from registry directly (recommended)**

```bash
sudo ./bin/sandkasten image pull --name python python:3.12-slim
```

**Method 2: Build rootfs manually + import tarball**

```bash
sudo debootstrap --variant=minbase bookworm /tmp/python-rootfs
sudo chroot /tmp/python-rootfs apt-get install -y python3 python3-pip
tar -czf python.tar.gz -C /tmp/python-rootfs .
sudo ./bin/imgbuilder import --name python --tar python.tar.gz
```

### Image Requirements

Each image must contain:
- `/bin/sh` - Basic shell
- `/usr/local/bin/runner` - Runner binary (auto-copied on import)

## Configuration

Bootstrap quickly:

```bash
# Creates config/data dirs and pulls a default base image
sudo ./bin/sandkasten init --config sandkasten.yaml
```

Minimal `sandkasten.yaml`:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-your-secret-key"
data_dir: "/var/lib/sandkasten"
```

Full example:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-your-secret-key"
data_dir: "/var/lib/sandkasten"
default_image: "python"
allowed_images: ["base", "python", "node"]
session_ttl_seconds: 1800

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  max_exec_timeout_ms: 120000
  network_mode: "none"

workspace:
  enabled: true
  persist_by_default: false

security:
  seccomp: "off"  # off | mvp
```

See [Configuration Guide](./docs/configuration.md) for all options.

## WSL2 Support

Sandkasten runs on Windows via WSL2:

```powershell
# In PowerShell
wsl --install -d Ubuntu-22.04

# In WSL
cd /mnt/c/path/to/sandkasten
task build
sudo ./bin/sandkasten --config sandkasten.yaml
```

**Important:** Store data inside WSL's filesystem (not `/mnt/c`):
```yaml
data_dir: "/var/lib/sandkasten"  # Correct - uses ext4
# data_dir: "/mnt/c/sandkasten"  # Wrong - NTFS doesn't support overlayfs
```

## SDKs

### Python

```bash
pip install sandkasten
```

```python
from sandkasten import SandboxClient

client = SandboxClient(base_url="...", api_key="...")
session = await client.create_session()
result = await session.exec("echo hello")
```

### TypeScript

```bash
npm install @sandkasten/client
```

```typescript
import { SandboxClient } from '@sandkasten/client';

const client = new SandboxClient({baseUrl: '...', apiKey: '...'});
const session = await client.createSession();
const result = await session.exec('echo hello');
```

## Security

Sandboxes are isolated with:
- ✅ Mount/PID/UTS/IPC namespaces
- ✅ Optional network namespace (no network by default)
- ✅ cgroups v2 resource limits
- ✅ All capabilities dropped
- ✅ no_new_privs flag
- ✅ Read-only base rootfs (overlayfs lower)

For production:
- Use strong API keys
- Bind to localhost (use reverse proxy)
- Keep network disabled
- Set conservative resource limits
- Run as non-root when possible (requires user namespace setup)

## Development

### Build Commands

```bash
task build      # Build everything
task daemon     # Build daemon only
task runner     # Build runner binary
task imgbuilder # Build legacy image import tool
task run        # Run daemon locally
```

### Project Structure

```
cmd/
├── sandkasten/      # Daemon entrypoint
├── runner/          # Runner binary (PID 1 in sandbox)
└── imgbuilder/      # Image management tool

internal/
├── api/             # HTTP handlers
├── session/         # Session orchestration
├── runtime/
│   ├── driver.go    # Runtime interface
│   └── linux/       # Linux implementation
├── store/           # SQLite persistence
├── reaper/          # TTL cleanup
├── config/          # Configuration
└── web/             # Dashboard

protocol/            # Runner ↔ Daemon protocol
```

## API Reference

See [API Documentation](./docs/api.md) for complete reference.

Quick reference:

| Endpoint | Description |
|----------|-------------|
| `POST /v1/sessions` | Create session |
| `GET /v1/sessions` | List sessions |
| `GET /v1/sessions/{id}` | Get session |
| `POST /v1/sessions/{id}/exec` | Execute command |
| `POST /v1/sessions/{id}/fs/write` | Write file |
| `GET /v1/sessions/{id}/fs/read` | Read file |
| `DELETE /v1/sessions/{id}` | Destroy session |
| `GET /v1/workspaces` | List workspaces |

## License

MIT - See [LICENSE](./LICENSE) for details.

## Credits

Built with:
- Linux namespaces, cgroups, and overlayfs
- [creack/pty](https://github.com/creack/pty) for PTY management
- [modernc.org/sqlite](https://modernc.org/sqlite) for pure-Go SQLite
