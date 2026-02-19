# Configuration Guide

Complete reference for configuring Sandkasten.

## Configuration File

Location: `sandkasten.yaml` (specify with `--config` flag)

### Minimal Configuration

```yaml
listen: "127.0.0.1:8080"
api_key: "your-secret-key-here"
data_dir: "/var/lib/sandkasten"
```

### Full Configuration

```yaml
# Server settings
listen: "127.0.0.1:8080"
api_key: "sk-your-secret-key"

# Data storage
data_dir: "/var/lib/sandkasten"
db_path: "/var/lib/sandkasten/sandkasten.db"

# Image settings
default_image: "python"
allowed_images:
  - "base"
  - "python"
  - "node"

# Session settings
session_ttl_seconds: 1800  # 30 minutes

# Resource limits
defaults:
  cpu_limit: 1.0              # CPU cores (1.0 = 1 core)
  mem_limit_mb: 512           # Memory limit in MB
  pids_limit: 256             # Process limit
  max_exec_timeout_ms: 120000 # Max exec timeout (2 min)
  network_mode: "none"        # "none" or "bridge" (requires setup)

# Workspace persistence
workspace:
  enabled: true
  persist_by_default: false

# Security
security:
  seccomp: "off"  # "off" or "mvp" (experimental)
```

## Configuration Options

### Server Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen` | string | `127.0.0.1:8080` | Host and port to bind. For production, use `127.0.0.1` and put a reverse proxy (with TLS) in front, or ensure `api_key` is set if binding to `0.0.0.0`. |
| `api_key` | string | `""` | API key. **Empty = open access (dev only).** Never leave empty in production or when binding to a non-loopback address (e.g. `0.0.0.0`); the daemon will refuse to start. |

### Data Storage

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `data_dir` | string | `/var/lib/sandkasten` | Base directory for all data |
| `db_path` | string | `<data_dir>/sandkasten.db` | SQLite database path |

> **Important on WSL2:** Store `data_dir` inside the Linux filesystem (e.g., `/var/lib/sandkasten`), not on NTFS (`/mnt/c/...`). NTFS doesn't support overlayfs properly.

### Images

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `default_image` | string | `base` | Default image for new sessions |
| `allowed_images` | []string | `[]` | Allowed images (empty = all) |

### Sessions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `session_ttl_seconds` | int | `1800` | Session lifetime in seconds (30 min) |

### Resource Limits

```yaml
defaults:
  cpu_limit: 1.0              # CPU cores
  mem_limit_mb: 512           # Memory in MB
  pids_limit: 256             # Max processes
  max_exec_timeout_ms: 120000 # Max command timeout
  network_mode: "none"        # Network isolation
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `cpu_limit` | float | `1.0` | CPU cores (uses cgroup cpu.max) |
| `mem_limit_mb` | int | `512` | Memory limit in MB |
| `pids_limit` | int | `256` | Maximum number of processes |
| `max_exec_timeout_ms` | int | `120000` | Maximum command execution time |
| `network_mode` | string | `none` | Network mode (`none` = no network) |

### Workspace Persistence

```yaml
workspace:
  enabled: true
  persist_by_default: false
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable persistent workspaces |
| `persist_by_default` | bool | `false` | Create persistent workspace by default |

When enabled, sessions can specify a `workspace_id` to persist files across session destruction:

```bash
# Create session with persistent workspace
curl -X POST http://localhost:8080/v1/sessions \
  -d '{"workspace_id": "my-project"}'
```

### Security

```yaml
security:
  seccomp: "off"
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `seccomp` | string | `off` | Seccomp profile (`off` or `mvp`) |

## Environment Variables

All config options can be overridden with environment variables (prefix: `SANDKASTEN_`):

| Variable | Config Option |
|----------|---------------|
| `SANDKASTEN_LISTEN` | `listen` |
| `SANDKASTEN_API_KEY` | `api_key` |
| `SANDKASTEN_DATA_DIR` | `data_dir` |
| `SANDKASTEN_DEFAULT_IMAGE` | `default_image` |
| `SANDKASTEN_ALLOWED_IMAGES` | `allowed_images` (comma-separated) |
| `SANDKASTEN_DB_PATH` | `db_path` |
| `SANDKASTEN_SESSION_TTL_SECONDS` | `session_ttl_seconds` |
| `SANDKASTEN_CPU_LIMIT` | `defaults.cpu_limit` |
| `SANDKASTEN_MEM_LIMIT_MB` | `defaults.mem_limit_mb` |
| `SANDKASTEN_PIDS_LIMIT` | `defaults.pids_limit` |
| `SANDKASTEN_MAX_EXEC_TIMEOUT_MS` | `defaults.max_exec_timeout_ms` |
| `SANDKASTEN_NETWORK_MODE` | `defaults.network_mode` |
| `SANDKASTEN_SECCOMP` | `security.seccomp` |

Example:

```bash
export SANDKASTEN_API_KEY="sk-prod-secret"
export SANDKASTEN_DATA_DIR="/var/lib/sandkasten"
export SANDKASTEN_NETWORK_MODE="none"
# Foreground
sudo ./bin/sandkasten --config sandkasten.yaml
# Or background (daemon mode, like Docker)
sudo ./bin/sandkasten daemon -d --config sandkasten.yaml
```

When running detached, the PID is written to `<data_dir>/run/sandkasten.pid`. Use `./bin/sandkasten ps` to list sessions (similar to `docker ps`).

## Security Recommendations

### Production Deployment

1. **Use strong API key**
   ```bash
   api_key: "sk-prod-$(openssl rand -hex 32)"
   ```

2. **Bind to localhost only** (use reverse proxy for external access)
   ```yaml
   listen: "127.0.0.1:8080"
   ```

3. **Keep network disabled** (default)
   ```yaml
   defaults:
     network_mode: "none"
   ```

4. **Set conservative resource limits**
   ```yaml
   defaults:
     cpu_limit: 0.5
     mem_limit_mb: 256
     pids_limit: 64
   ```

5. **Restrict allowed images**
   ```yaml
   allowed_images:
     - "python"
     - "node"
   ```

6. **Rate limiting**: The daemon does not rate-limit requests. Put a reverse proxy (e.g. nginx, Caddy) in front and configure rate limits per IP or per API key to reduce DoS risk.

### Isolation Guarantees

Each sandbox is isolated with:

- **Namespaces**: mount, pid, uts, ipc, network (optional)
- **cgroups v2**: cpu, memory, pids limits
- **Capabilities**: All capabilities dropped
- **no_new_privs**: Cannot gain new privileges
- **Filesystem**: Read-only base via overlayfs

### Reverse Proxy Example

#### Nginx

```nginx
server {
    listen 443 ssl;
    server_name sandkasten.example.com;

    ssl_certificate /etc/ssl/cert.pem;
    ssl_certificate_key /etc/ssl/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 300s;  # Long-running commands
    }
}
```

#### Caddy

```
sandkasten.example.com {
    reverse_proxy localhost:8080
}
```

## Image Management

### Available Images

Images are stored in `<data_dir>/images/<name>/rootfs/`. Each image must contain:
- `/bin/sh` - Basic shell
- `/usr/local/bin/runner` - Runner binary (auto-copied on import)

### Import Images

```bash
# Import from tarball
sudo ./bin/imgbuilder import --name python --tar python.tar.gz

# List images
./bin/imgbuilder list

# Validate image
./bin/imgbuilder validate python

# Delete image
sudo ./bin/imgbuilder delete python
```

### Building Custom Images

**From Docker (build-time only):**

```bash
docker build -t my-image .
docker create --name temp my-image
docker export temp | gzip > my-image.tar.gz
docker rm temp
sudo ./bin/imgbuilder import --name my-image --tar my-image.tar.gz
```

**From scratch (debootstrap):**

```bash
sudo debootstrap --variant=minbase bookworm /tmp/rootfs
sudo chroot /tmp/rootfs apt-get install -y python3
tar -czf python.tar.gz -C /tmp/rootfs .
sudo ./bin/imgbuilder import --name python --tar python.tar.gz
```

## System Requirements

### Linux

- Kernel 5.11+ (for overlayfs in user namespaces)
- cgroups v2 mounted at `/sys/fs/cgroup`
- Root or CAP_SYS_ADMIN capability

### WSL2

- Windows 10/11 with WSL2
- Ubuntu 22.04+ (or any distro with cgroups v2)
- Store data in Linux filesystem (not `/mnt/c`)

Verify cgroups v2:

```bash
mount | grep cgroup2
# Should show: cgroup2 on /sys/fs/cgroup type cgroup2
```

Verify overlayfs:

```bash
cat /proc/filesystems | grep overlay
# Should show: overlay
```
