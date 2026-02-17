# Quickstart Guide

Get Sandkasten running in 5 minutes.

## Prerequisites

- Linux (kernel 5.11+) or WSL2 with Ubuntu 22.04+
- cgroups v2 (default on modern systems)
- Go 1.24+ (for building)

> **Note:** macOS is not supported. Use a Linux VM or WSL2.

## Quick Start

### 1. Build

```bash
git clone https://github.com/yourusername/sandkasten
cd sandkasten
task build
```

This builds:
- `bin/sandkasten` - The daemon
- `bin/runner` - Runner binary (embedded in sandboxes)
- `bin/imgbuilder` - Image management tool

### 2. Prepare a Rootfs Image

You need at least one rootfs image. Choose one method:

**Option A: Export from Docker (easiest)**

```bash
# Create and export minimal Alpine rootfs
docker create --name temp alpine:latest
docker export temp | gzip > /tmp/base.tar.gz
docker rm temp

# Import into sandkasten
sudo ./bin/imgbuilder import --name base --tar /tmp/base.tar.gz
```

**Option B: Using debootstrap (Debian/Ubuntu)**

```bash
# Create minimal Debian rootfs
sudo debootstrap --variant=minbase bookworm /tmp/rootfs
tar -czf /tmp/base.tar.gz -C /tmp/rootfs .
sudo ./bin/imgbuilder import --name base --tar /tmp/base.tar.gz
```

**Option C: Create Python image**

```bash
# Start from a Python container
docker create --name temp python:3.12-slim
docker export temp | gzip > /tmp/python.tar.gz
docker rm temp

sudo ./bin/imgbuilder import --name python --tar /tmp/python.tar.gz
```

### 3. Verify Images

```bash
./bin/imgbuilder list

# Validate
./bin/imgbuilder validate base
```

### 4. Create Configuration

Create `sandkasten.yaml`:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-quickstart"
data_dir: "/var/lib/sandkasten"
default_image: "base"
```

### 5. Start Daemon

```bash
# Create data directory
sudo mkdir -p /var/lib/sandkasten/images
sudo mkdir -p /var/lib/sandkasten/sessions
sudo mkdir -p /var/lib/sandkasten/workspaces

# Start daemon (requires root for namespace operations)
sudo ./bin/sandkasten --config sandkasten.yaml
```

### 6. Verify It's Running

```bash
# Check health
curl http://localhost:8080/healthz

# Open dashboard
open http://localhost:8080
```

## Your First Session

### Using cURL

```bash
# Create session
SESSION_ID=$(curl -s -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-sandbox-quickstart" \
  -d '{"image":"base"}' | jq -r .id)

echo "Session ID: $SESSION_ID"

# Execute command
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-quickstart" \
  -d '{"cmd":"echo hello world"}'

# Write a file
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/fs/write \
  -H "Authorization: Bearer sk-sandbox-quickstart" \
  -d '{"path":"/workspace/test.txt","text":"Hello from sandbox!"}'

# Read it back
curl "http://localhost:8080/v1/sessions/$SESSION_ID/fs/read?path=/workspace/test.txt" \
  -H "Authorization: Bearer sk-sandbox-quickstart"

# Clean up
curl -X DELETE http://localhost:8080/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer sk-sandbox-quickstart"
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
        api_key="sk-sandbox-quickstart"
    )
    
    async with await client.create_session(image="base") as session:
        # Execute command
        result = await session.exec("echo hello")
        print(result.output)
        
        # Write file
        await session.write("test.py", "print('Hello!')")
        
        # Run Python
        result = await session.exec("python3 test.py")
        print(result.output)

asyncio.run(main())
```

## Next Steps

- [Configuration Guide](./configuration.md) - Customize settings
- [API Reference](./api.md) - Complete API documentation
- [Build custom images](#building-custom-images)

## Building Custom Images

### Python Image

```dockerfile
# Dockerfile for Python rootfs
FROM python:3.12-slim

# Install common packages
RUN pip install numpy pandas requests

# Create sandbox user
RUN useradd -m -u 1000 sandbox
USER sandbox
WORKDIR /workspace
```

Build and export:

```bash
docker build -t my-python .
docker create --name temp my-python
docker export temp | gzip > /tmp/my-python.tar.gz
docker rm temp

sudo ./bin/imgbuilder import --name my-python --tar /tmp/my-python.tar.gz
```

### Node.js Image

```bash
docker create --name temp node:22-slim
docker export temp | gzip > /tmp/node.tar.gz
docker rm temp

sudo ./bin/imgbuilder import --name node --tar /tmp/node.tar.gz
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

Check available images:

```bash
./bin/imgbuilder list
```

Import one if needed:

```bash
sudo ./bin/imgbuilder import --name base --tar /tmp/base.tar.gz
```

### "Invalid API key"

Check your config matches your requests:

```yaml
# sandkasten.yaml
api_key: "sk-sandbox-quickstart"
```

```bash
# Request header
Authorization: Bearer sk-sandbox-quickstart
```

## WSL2 Tips

1. **Use WSL2, not WSL1** - WSL1 doesn't support cgroups v2
2. **Store data in Linux filesystem** - Use `/var/lib/sandkasten`, not `/mnt/c/...`
3. **Run daemon in WSL** - Access from Windows via `localhost:8080`

```powershell
# From PowerShell, access the daemon running in WSL
curl http://localhost:8080/healthz
```
