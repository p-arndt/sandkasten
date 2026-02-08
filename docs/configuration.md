# Configuration Guide

Complete reference for configuring Sandkasten.

## Configuration File

Location: `sandkasten.yaml` (specify with `--config` flag)

### Minimal Configuration

```yaml
listen: "127.0.0.1:8080"
api_key: "your-secret-key-here"
```

### Full Configuration

See [quickstart/daemon/sandkasten-full.yaml](../quickstart/daemon/sandkasten-full.yaml) for complete example.

## Configuration Options

### Server

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen` | string | `127.0.0.1:8080` | Host and port to bind |
| `api_key` | string | `""` | API key (empty = open access) |

### Images

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `default_image` | string | `sandbox-runtime:base` | Default image for sessions |
| `allowed_images` | []string | `[]` | Allowed images (empty = all) |

### Database

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `db_path` | string | `./sandkasten.db` | SQLite database path |
| `session_ttl_seconds` | int | `1800` | Default session lifetime (30 min) |

### Container Defaults

```yaml
defaults:
  cpu_limit: 1.0              # CPU cores (1.0 = 1 core)
  mem_limit_mb: 512           # Memory limit in MB
  pids_limit: 256             # Process limit
  max_exec_timeout_ms: 120000 # Max exec timeout (2 min)
  network_mode: "none"        # "none", "bridge", "host"
  readonly_rootfs: true       # Read-only root filesystem
```

### Workspace Persistence

```yaml
workspace:
  enabled: true                # Enable persistent workspaces
  persist_by_default: false    # Require explicit workspace_id
```

See [features/workspaces.md](./features/workspaces.md) for details.

### Container Pool

```yaml
pool:
  enabled: true
  images:
    sandbox-runtime:python: 3  # Keep 3 Python containers ready
    sandbox-runtime:node: 2    # Keep 2 Node containers ready
```

See [features/pool.md](./features/pool.md) for details.

## Environment Variables

All config options can be overridden with environment variables:

| Variable | Config Option |
|----------|---------------|
| `SANDKASTEN_LISTEN` | `listen` |
| `SANDKASTEN_API_KEY` | `api_key` |
| `SANDKASTEN_DEFAULT_IMAGE` | `default_image` |
| `SANDKASTEN_ALLOWED_IMAGES` | `allowed_images` (comma-separated) |
| `SANDKASTEN_DB_PATH` | `db_path` |
| `SANDKASTEN_SESSION_TTL_SECONDS` | `session_ttl_seconds` |
| `SANDKASTEN_CPU_LIMIT` | `defaults.cpu_limit` |
| `SANDKASTEN_MEM_LIMIT_MB` | `defaults.mem_limit_mb` |
| `SANDKASTEN_NETWORK_MODE` | `defaults.network_mode` |

Example:
```bash
export SANDKASTEN_API_KEY="sk-prod-secret"
export SANDKASTEN_NETWORK_MODE="none"
./sandkasten --config sandkasten.yaml
```

## Security Recommendations

### Production Deployment

1. **Use strong API key**
   ```yaml
   api_key: "sk-prod-$(openssl rand -hex 32)"
   ```

2. **Bind to localhost only** (use reverse proxy)
   ```yaml
   listen: "127.0.0.1:8080"
   ```

3. **Disable network** (unless needed)
   ```yaml
   defaults:
     network_mode: "none"
   ```

4. **Enable read-only root**
   ```yaml
   defaults:
     readonly_rootfs: true
   ```

5. **Set resource limits**
   ```yaml
   defaults:
     cpu_limit: 0.5
     mem_limit_mb: 256
     pids_limit: 64
   ```

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

- `sandbox-runtime:base` - Minimal (bash, coreutils)
- `sandbox-runtime:python` - Python 3 + pip + uv + common packages
- `sandbox-runtime:node` - Node.js 22 + npm

### Allow Specific Images Only

```yaml
allowed_images:
  - "sandbox-runtime:python"
  - "sandbox-runtime:node"
```

### Custom Images

Build custom images extending the base:

```dockerfile
FROM sandbox-runtime:base

USER root
RUN apt-get update && apt-get install -y my-tool
USER sandbox
```

Then allow it:
```yaml
allowed_images:
  - "my-custom-image:latest"
```
