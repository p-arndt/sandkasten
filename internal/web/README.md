# Sandkasten Web Dashboard

Simple web UI for monitoring and managing sandbox sessions.

## Features

### Status Page (`/`)
- **Live session monitoring** - Auto-refreshes every 5 seconds
- **Stats dashboard** - Total sessions, active, expired, uptime
- **Session table** - View all sessions with:
  - Session ID
  - Docker image
  - Status (running/expired/destroyed)
  - Created time
  - Expiry time
  - Last activity
  - Destroy button for manual cleanup

### Settings Page (`/settings`)
- **Config editor** - Edit `sandkasten.yaml` directly in browser
- **YAML validation** - Validate before saving
- **Live reload** - Reload current config from file
- **Configuration reference** - Inline documentation for all config options

## Usage

Start the daemon with a config file:

```bash
./sandkasten --config sandkasten.yaml
```

Then open in your browser:

```
http://localhost:8080/          # Status page
http://localhost:8080/settings  # Settings page
```

## API Endpoints

The dashboard uses these API endpoints:

- `GET /api/status` - Get session stats and list
  ```json
  {
    "total_sessions": 5,
    "active_sessions": 3,
    "expired_sessions": 2,
    "uptime_seconds": 3600,
    "sessions": [...]
  }
  ```

- `GET /api/config` - Get current YAML config
  ```json
  {
    "content": "listen: 0.0.0.0:8080\n...",
    "path": "/path/to/sandkasten.yaml"
  }
  ```

- `PUT /api/config` - Save YAML config
  ```json
  {
    "content": "listen: 0.0.0.0:8080\n..."
  }
  ```

- `POST /api/config/validate` - Validate YAML without saving
  ```json
  {
    "content": "listen: 0.0.0.0:8080\n..."
  }
  ```
  Returns: `{"valid": true}` or `{"valid": false, "error": "..."}`

## Security

**Important**: The web UI does NOT require authentication by default. In production:

1. **Add authentication** - Modify `internal/api/middleware.go` to require auth for web routes
2. **Use HTTPS** - Put behind a reverse proxy (nginx, Caddy) with TLS
3. **Restrict access** - Bind to `127.0.0.1` or use firewall rules

Example nginx config for auth:

```nginx
location / {
    auth_basic "Sandkasten Dashboard";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```

## Implementation

### Embedded Templates

HTML templates are embedded in the binary using `//go:embed`:

```go
//go:embed templates/status.html
var statusHTML string
```

This means no external files are needed at runtime.

### Styling

- Pure CSS, no frameworks
- Dark theme optimized for monitoring
- Responsive design
- Monospace fonts for IDs and technical data

### JavaScript

- Vanilla JS, no dependencies
- Auto-refresh with `setInterval`
- Fetch API for all requests
- Simple error handling with alerts

## Development

To modify the UI:

1. Edit HTML in `internal/web/templates/`
2. Rebuild daemon: `make build`
3. Restart daemon
4. Refresh browser (hard refresh: Cmd+Shift+R / Ctrl+F5)

## Future Enhancements

Potential improvements:

- [ ] Session logs viewer
- [ ] Metrics graphs (session count over time, exec latency)
- [ ] WebSocket for real-time updates instead of polling
- [ ] Session filtering and search
- [ ] Exec command history per session
- [ ] Container resource usage (CPU, memory)
- [ ] Export session data (CSV, JSON)
- [ ] Dark/light theme toggle
- [ ] Authentication system built-in
