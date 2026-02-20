# Quickstart Guide

Get Sandkasten running in 5 minutes. This guide walks through **build → images → config → daemon → first session** so nothing is skipped.

## Prerequisites

- Linux (kernel 5.11+) or WSL2 with Ubuntu 22.04+
- cgroups v2 (default on modern systems)
- Go 1.24+ (for building)

> **Note:** macOS is not supported. Use a Linux VM or WSL2.

## Complete setup (step by step)

### 1. Build

```bash
git clone https://github.com/p-arndt/sandkasten
cd sandkasten
task build
```

This produces:
- `bin/sandkasten` — daemon and CLI
- `bin/runner` — runs inside sandboxes (embedded when creating images)
- `bin/imgbuilder` — legacy image import tool

### 2. Preflight and bootstrap

```bash
# Check kernel, cgroups, overlayfs, data-dir
./bin/sandkasten doctor

# Security baseline (api key, seccomp, limits)
./bin/sandkasten security --config sandkasten.yaml

# Create sandkasten.yaml + data dirs + pull first image (name: base)
sudo ./bin/sandkasten init --config sandkasten.yaml
```

By default, `init` pulls **alpine:latest** as image name **base**. For Python/Node sessions or the example agents you need to pull more images (next step).

### 3. Create images

Sessions run from an **image** (a rootfs). You must have at least one image. Pull from OCI registries without a Docker daemon:

```bash
# Python (required for quickstart/agent examples)
sudo ./bin/sandkasten image pull --name python python:3.12-slim

# Optional: Node.js
sudo ./bin/sandkasten image pull --name node node:22-slim

# List and validate
./bin/sandkasten image list
./bin/sandkasten image validate base
./bin/sandkasten image validate python
```

Image names (e.g. `python`, `node`, `base`) are what you use in config and API.

### 4. Configuration

If you used `sandkasten init`, you already have `sandkasten.yaml`. Edit it and set **default_image** to an image you created (e.g. `python`), and set **api_key**:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-test"
data_dir: "/var/lib/sandkasten"
default_image: "python"
```

If you did **not** run `init`, create the data dirs and config manually:

```bash
sudo mkdir -p /var/lib/sandkasten/images
sudo mkdir -p /var/lib/sandkasten/sessions
sudo mkdir -p /var/lib/sandkasten/workspaces
```

Create `sandkasten.yaml` with the same content as above. See [Configuration](./configuration.md) for all options.

### 5. Start the daemon

```bash
# Foreground (logs in terminal; Ctrl+C to stop)
sudo ./bin/sandkasten --config sandkasten.yaml

# Or background (like Docker)
sudo ./bin/sandkasten daemon -d --config sandkasten.yaml
```

Stop when running in background:

```bash
sudo ./bin/sandkasten stop
```

### 6. Verify

```bash
curl http://localhost:8080/healthz
./bin/sandkasten ps
# Dashboard: http://localhost:8080
```

Once you see a healthy response and `ps` works, you can create sessions and run the example agent.

## Your First Session

Use the same **api_key** as in your `sandkasten.yaml` (e.g. `sk-test`). Use an **image** you created (e.g. `python` or `base`).

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

This happens when using NTFS (e.g., `/mnt/c` in WSL2).

**Solution:** Store data inside WSL's Linux filesystem:

```yaml
# Correct
data_dir: "/var/lib/sandkasten"

# Wrong (NTFS doesn't support overlayfs properly)
data_dir: "/mnt/c/sandkasten"
```

### "permission denied" / namespace errors

The daemon needs root (or CAP_SYS_ADMIN) to create namespaces:

```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

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

1. **Use WSL2, not WSL1** - WSL1 doesn't support cgroups v2
2. **Store data in Linux filesystem** - Use `/var/lib/sandkasten`, not `/mnt/c/...`
3. **Run daemon in WSL** - Access from Windows via `localhost:8080`

```powershell
# From PowerShell, access the daemon running in WSL
curl http://localhost:8080/healthz
```
