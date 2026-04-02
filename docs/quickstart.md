# Quickstart Guide

Get Sandkasten running in under a minute. Pick whichever path fits your setup.

## Prerequisites

- Linux (kernel 5.11+) or WSL2 with Ubuntu 22.04+
- cgroups v2 (default on modern systems)

> [!NOTE]
> macOS is not supported. Use a Linux VM or WSL2.

## Setup paths

### Option A: Docker (fastest, no build needed)

```bash
docker run -d --privileged --name sandkasten \
  -p 8080:8080 \
  -v sandkasten-data:/var/lib/sandkasten \
  ghcr.io/p-arndt/sandkasten:latest \
  /bin/sandkasten up
```

Or with docker compose (no repo clone needed):

```bash
curl -O https://raw.githubusercontent.com/p-arndt/sandkasten/main/docker-compose.standalone.yml
docker compose -f docker-compose.standalone.yml up -d
# Default API key: sk-sandkasten
```

Check status:

```bash
docker logs sandkasten 2>&1 | grep "API key"
curl http://localhost:8080/healthz
```

### Option B: Install binary + `sandkasten up` (recommended for native Linux/WSL2)

```bash
# Install the latest release binary
curl -fsSL https://raw.githubusercontent.com/p-arndt/sandkasten/main/scripts/install.sh | sudo bash

# Start with zero config
sudo sandkasten up
```

`sandkasten up` automatically:
1. Checks your environment (kernel, cgroups, overlayfs)
2. Creates data directories in `/var/lib/sandkasten`
3. Pulls a Python sandbox image from the registry
4. Generates an API key and prints it to the terminal
5. Starts the daemon on `localhost:8080`

You can customize the defaults:

```bash
sudo sandkasten up --image node           # Use Node.js instead of Python
sudo sandkasten up -d                      # Run in background
sudo sandkasten up --listen 0.0.0.0:8080   # Bind to all interfaces
```

### Option C: Build from source + `sandkasten up`

```bash
git clone https://github.com/p-arndt/sandkasten
cd sandkasten
task build
sudo ./bin/sandkasten up
```

### Option D: Step-by-step (full control)

For production or custom setups where you want explicit control over each step:

```bash
# 1. Build (or install binary)
task build

# 2. Check environment
./bin/sandkasten doctor

# 3. Bootstrap config, data dirs, and first image
sudo ./bin/sandkasten init --config sandkasten.yaml

# 4. Pull additional images
sudo ./bin/sandkasten image pull --name python python:3.12-slim
sudo ./bin/sandkasten image pull --name node node:22-slim

# 5. Edit sandkasten.yaml (set default_image, api_key, etc.)

# 6. Start daemon
sudo ./bin/sandkasten --config sandkasten.yaml
```

See [Configuration](./configuration.md) for all options.

### Verify

```bash
curl http://localhost:8080/healthz
./bin/sandkasten ps
```

## Auto-pull images

Sandkasten automatically pulls images from OCI registries when they are requested but not available locally. This means you don't need to manually pull images before creating sessions.

Well-known image mappings:

| Name | Pulls | Description |
|------|-------|-------------|
| `python` | `python:3.12-slim` | Python 3.12 with pip |
| `node` | `node:22-slim` | Node.js 22 with npm |
| `base` | `alpine:latest` | Minimal Alpine Linux |
| `ubuntu` | `ubuntu:24.04` | Ubuntu 24.04 |
| `golang` | `golang:1.25-alpine` | Go 1.25 |
| `ruby` | `ruby:3.3-slim` | Ruby 3.3 |
| `rust` | `rust:1-slim` | Rust stable |

Unknown image names are pulled as `<name>:latest` from Docker Hub.

Auto-pull is enabled by default. Disable with `auto_pull: { enabled: false }` in config or `SANDKASTEN_AUTO_PULL=false`.

## Your First Session

> [!IMPORTANT]
> Use the same **api_key** as in your `sandkasten.yaml` (e.g. `sk-test`) and an **image** you pulled (e.g. `python` or `base`).

### Using cURL

```bash
# Create session (use your api_key and an image you pulled)
SESSION_ID=$(curl -s -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-test" \
  -d '{"image":"python"}' | jq -r .id)

echo "Session ID: $SESSION_ID"

# Execute command
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-test" \
  -d '{"cmd":"echo hello world"}'

# Write a file
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/fs/write \
  -H "Authorization: Bearer sk-test" \
  -d '{"path":"/workspace/test.txt","text":"Hello from sandbox!"}'

# Read it back
curl "http://localhost:8080/v1/sessions/$SESSION_ID/fs/read?path=/workspace/test.txt" \
  -H "Authorization: Bearer sk-test"

# Clean up
curl -X DELETE http://localhost:8080/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer sk-test"
```

### Using Python SDK

```bash
pip install sandkasten
```

```python
import asyncio
from sandkasten import SandboxClient

async def main():
    client = SandboxClient(
        base_url="http://localhost:8080",
        api_key="sk-test"
    )
    # Use an image you pulled (e.g. "python"); or omit for default_image from config
    async with await client.create_session(image="python") as session:
        result = await session.exec("echo hello")
        print(result.output)
        await session.write("test.py", "print('Hello!')")
        result = await session.exec("python3 test.py")
        print(result.output)

asyncio.run(main())
```

### Run the example agent

```bash
cd quickstart/agent
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

The agent uses the daemon’s **default_image** and API key from config. See [OpenAI Agents SDK](./openai-agents.md) for wiring Sandkasten as tools.

## Next Steps

- [Configuration Guide](./configuration.md) — Customize settings
- [API Reference](./api.md) — Full API docs
- [OpenAI Agents SDK](./openai-agents.md) — Use Sandkasten as tools in an agent
- [Build custom images](#building-custom-images) — Pull more images or build from rootfs

## Building Custom Images

### Python Image (registry pull)

```bash
sudo ./bin/sandkasten image pull --name my-python python:3.12-slim
```

### Node.js Image (registry pull)

```bash
sudo ./bin/sandkasten image pull --name node node:22-slim
```

### Manual Rootfs Build (no registry)

```bash
sudo debootstrap --variant=minbase bookworm /tmp/custom-rootfs
sudo chroot /tmp/custom-rootfs apt-get update
sudo chroot /tmp/custom-rootfs apt-get install -y python3 python3-pip
tar -czf /tmp/custom-rootfs.tar.gz -C /tmp/custom-rootfs .
sudo ./bin/imgbuilder import --name custom --tar /tmp/custom-rootfs.tar.gz
```

## Troubleshooting

### "cgroup v2 not mounted"

Ensure cgroups v2 is enabled:

```bash
mount | grep cgroup2
# Should show: cgroup2 on /sys/fs/cgroup type cgroup2
```

If not, check your kernel boot parameters or use a newer distro.

### "overlayfs: upper fs does not support xattrs"

> [!WARNING]
> This happens when `data_dir` is on NTFS (e.g. `/mnt/c` in WSL2). Store data inside WSL's Linux filesystem: `data_dir: "/var/lib/sandkasten"`. Do not use `/mnt/c/...`.

### "permission denied" / namespace errors

The daemon needs root (or CAP_SYS_ADMIN) to create namespaces:

```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

### cgroup "not delegated" warnings in Docker

Example warnings:
- `cannot set memory limit (cgroup not delegated)`
- `cannot set pids limit (cgroup not delegated)`
- `cannot set cpu limit (cgroup not delegated)`

In Docker Desktop/WSL2 this can happen because nested cgroup delegation is restricted. Sandkasten still runs, but strict per-session cgroup limits may not be fully enforceable.

Options:
- Keep current setup (warnings are emitted once per daemon process)
- For fully enforced limits, run Sandkasten directly on Linux/WSL2 host (not nested in Docker)

### "image not found"

List images and pull the one your config or request uses:

```bash
./bin/sandkasten image list
sudo ./bin/sandkasten image pull --name python python:3.12-slim
```

Ensure `default_image` in `sandkasten.yaml` matches a name you pulled (e.g. `python` or `base`).

### "Invalid API key"

Use the same API key in `sandkasten.yaml` and in your requests (e.g. `Authorization: Bearer sk-test`).

## WSL2 Tips

> [!TIP]
> - Use **WSL2**, not WSL1 (WSL1 doesn't support cgroups v2).
> - Store data in the Linux filesystem: `/var/lib/sandkasten`, not `/mnt/c/...`.
> - Run the daemon in WSL; access from Windows at `http://localhost:8080`.
