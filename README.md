<div align="center">
  <img src="logo.png" alt="Sandkasten" width="400">
</div>

<h1 align="center">Sandkasten</h1>

<p align="center">
  <strong>Self-hosted sandbox runtime for AI agents.</strong><br>
  Stateful Linux sandboxes with persistent shell, file operations, and workspace management — no Docker required.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
  <img src="https://img.shields.io/badge/platform-Linux%20%7C%20WSL2-green.svg" alt="Platform">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go">
</p>

---

## Try it

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

---

## Quick Start

Complete setup from zero to a running agent.

### 1. Build

```bash
git clone https://github.com/p-arndt/sandkasten
cd sandkasten
task build
```

This produces `bin/sandkasten`, `bin/runner`, and `bin/imgbuilder`.

### 2. Preflight & initialize

```bash
# Check kernel, cgroups, overlayfs
./bin/sandkasten doctor

# Security check (api key, seccomp, limits)
./bin/sandkasten security --config sandkasten.yaml

# Create sandkasten.yaml, data dirs, and pull the default image (name: base)
sudo ./bin/sandkasten init --config sandkasten.yaml
```

After this you have a minimal config and one image (`base`). For the example agent you need a **Python** image.

### 3. Create images

Sessions run from an **image** (rootfs). Pull at least one image you’ll use (e.g. Python for the agent examples):

```bash
# Pull a Python image (no Docker daemon required)
sudo ./bin/sandkasten image pull --name python python:3.12-slim

# Optional: Node.js
sudo ./bin/sandkasten image pull --name node node:22-slim

# List and validate
./bin/sandkasten image list
./bin/sandkasten image validate python
```

### 4. Configure

Edit `sandkasten.yaml` (in the repo root or where you run the daemon). Set `default_image` to an image you created and ensure `api_key` is set:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-test"
data_dir: "/var/lib/sandkasten"
default_image: "python"   # use the image you pulled
```

For more options (limits, workspaces, security) see [Configuration](./docs/configuration.md).

### 5. Start the daemon

```bash
# Foreground (logs in terminal)
sudo ./bin/sandkasten --config sandkasten.yaml

# Or background (like Docker daemon)
sudo ./bin/sandkasten daemon -d --config sandkasten.yaml
```

Useful commands:

```bash
./bin/sandkasten ps          # list sessions (like docker ps)
sudo ./bin/sandkasten stop   # stop daemon when run with daemon -d
```

When running in foreground, stop with **Ctrl+C**.  
**Production:** set a strong `api_key` (or `SANDKASTEN_API_KEY`). The daemon refuses to bind to a non-loopback address without an API key.

### 6. Verify

```bash
curl http://localhost:8080/healthz
# Open dashboard: http://localhost:8080
```

### 7. Run the example agent

```bash
cd quickstart/agent
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

The agent uses the daemon’s `default_image` (e.g. `python`) and connects to `http://localhost:8080` with the API key from your config.

### Run with Docker

You can also host Sandkasten with Docker:

```bash
# From the repo (sandkasten.yaml and docker-compose.yml in place)
mkdir -p /var/lib/sandkasten
docker compose up -d
```

The stack uses the repo’s `Dockerfile` and `docker-compose.yml` (mounts `./sandkasten.yaml` and `/var/lib/sandkasten`). The container runs privileged so the daemon can create sandboxes. API and dashboard on port 8080.

## Documentation

| Guide | Description |
|-------|-------------|
| [Quickstart](./docs/quickstart.md) | Get running in 5 minutes |
| [OpenAI Agents SDK](./docs/openai-agents.md) | Use Sandkasten as tools (exec, read, write) with the OpenAI Agents SDK |
| [Windows / WSL2](./docs/windows.md) | Detailed Windows instructions |
| [API Reference](./docs/api.md) | Complete HTTP API docs |
| [Configuration](./docs/configuration.md) | Config options and security |
| [Security Guide](./docs/security.md) | Hardened config and checklist |

---

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

Pull from a registry (recommended) or build custom images; see [Configuration](./docs/configuration.md) and the image tool help for details.

## Configuration

After `sandkasten init`, edit `sandkasten.yaml`. Minimal config:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-your-secret-key"
data_dir: "/var/lib/sandkasten"
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

---

## License

**MIT** — See [LICENSE](./LICENSE) for details.

## Credits

Built with:
- Linux namespaces, cgroups, and overlayfs
- [creack/pty](https://github.com/creack/pty) for PTY management
- [modernc.org/sqlite](https://modernc.org/sqlite) for pure-Go SQLite
