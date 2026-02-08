# Changelog

## [Unreleased] - 2026-02-08

### Security
- **CRITICAL:** Fixed path traversal vulnerability that allowed reading files outside `/workspace`
- **CRITICAL:** Fixed race condition in container pool cleanup that could cause crashes
- Added authentication requirement for config modification endpoints (`PUT /api/config`, `POST /api/config/validate`)
- Reduced Unix socket permissions from `0666` to `0600` for better security
- Enhanced command execution safety by using `printf` instead of `echo` for sentinel markers
- Fixed silent JSON marshaling errors that could hide failures

### Added
- **TypeScript SDK** enhancements:
  - Full workspace support (create sessions with `workspaceId`, list/delete workspaces)
  - Streaming exec support via `execStream()` async iterator
  - Convenience methods `writeText()` and `info()`
  - Now at feature parity with Python SDK
- **Structured Error Responses**:
  - Error codes: `SESSION_NOT_FOUND`, `SESSION_EXPIRED`, `INVALID_IMAGE`, `COMMAND_TIMEOUT`, etc.
  - Proper HTTP status codes (404, 410, 504, etc.) based on error type
  - Detailed validation error messages
- **Request Validation** for all API endpoints:
  - Session creation: TTL bounds (0-86400s), workspace ID format validation
  - Command execution: Timeout bounds (0-600000ms)
  - File operations: Path validation, content requirements
- New sentinel error types in session package for better error handling
- **Python SDK** (`sdk/python/sandkasten`):
  - Clean async API with `SandboxClient` and `Session` classes
  - Type-safe with dataclasses (`ExecResult`, `SessionInfo`)
  - Context manager support for automatic cleanup
  - Mirrors TypeScript SDK API for consistency
  - Comprehensive docstrings and examples
- **Web Dashboard** (`internal/web`):
  - Status page showing active sessions, stats, and real-time updates
  - Settings page for editing `sandkasten.yaml` config in browser
  - YAML validation before saving
  - Auto-refresh every 5 seconds
  - Session management (view details, destroy sessions)
  - Dark theme UI with clean design
  - Accessible at `/` (status) and `/settings`
- **Persistent Workspaces** (`internal/workspace`):
  - Docker volume-backed persistent storage
  - Optional `workspace_id` parameter on session creation
  - Workspace survives session destruction
  - Users can reconnect to same files later
  - Workspace management API (`GET /v1/workspaces`, `DELETE /v1/workspaces/{id}`)
  - Enable with `workspace.enabled: true` in config
- **Pre-warmed Container Pool** (`internal/pool`):
  - Background pool of ready-to-use containers
  - Instant session creation (no startup delay)
  - Configurable pool size per image
  - Auto-refill in background
  - Enable with `pool.enabled: true` and configure pool sizes
  - Graceful shutdown drains pool
- **Streaming Exec Output** (`/v1/sessions/{id}/exec/stream`):
  - Server-Sent Events (SSE) streaming for real-time output
  - Separate endpoint preserves backward compatibility
  - Python SDK `exec_stream()` async iterator
  - Useful for long-running commands (pip install, downloads)
  - Forward-compatible API (true streaming in v0.2)
- Enhanced interactive agent (`quickstart/agent/enhanced_agent.py`) with:
  - Rich terminal UI with boxes, colors, and panels
  - Streaming responses (token-by-token) using `Runner.run_streamed()`
  - SQLite-backed conversation history with `/history` and `/clear` commands
  - Visual tool execution feedback
  - Commands: `/history`, `/clear`, `/help`, `/quit`
- Enhanced Python sandbox image with pre-installed packages:
  - Common libraries: requests, httpx, pandas, numpy, matplotlib, beautifulsoup4, lxml, pyyaml, python-dotenv
  - Development tools: git, curl, wget, jq
  - uv package manager for fast installs
  - Python alias (`python` → `python3`)

### Changed
- All API handlers now return structured errors with error codes instead of generic messages
- Config endpoints now require authentication when API key is configured (read-only access still public)
- Error responses now include proper HTTP status codes based on error type

### Fixed
- **Security:** Path sanitization now correctly prevents absolute paths from escaping `/workspace`
- **Security:** Container pool cleanup no longer has race condition with refill workers
- JSON marshaling errors are now logged and handled gracefully
- Invalid request parameters now return 400 Bad Request instead of 500 Internal Server Error
- **Streaming Exec**: Fixed critical bug in SSE handler where `done` events were sent with zero-valued metadata (exit_code=0, cwd="", duration_ms=0) instead of actual command results. Handler now correctly checks `chunk.Done` flag instead of channel closure to emit proper completion data.
- Complete quickstart directory with:
  - Pre-configured daemon (Docker Compose)
  - Three agent examples (enhanced, simple, basic)
  - Comprehensive README with visual examples
  - Example tasks and prompts
- Agent documentation (`quickstart/agent/README.md`)
- Docker Compose setup for one-command deployment
- Example `.env` file
- Automated quickstart script (`quickstart/run.sh`)

### Fixed
- Tool name display in streaming events (now correctly accesses `event.item.raw_item.name`)
- Agent instructions updated to mention pre-installed packages and uv

### Changed
- Python sandbox image now includes uv and common packages
- Quickstart daemon configured with `network_mode: full` by default
- Agent instructions enhanced with package management guidance
- Updated all quickstart documentation to reference uv instead of pip/requirements.txt
- **Enhanced agent now uses Python SDK** instead of raw httpx calls (cleaner code)
- All quickstart examples updated to use Python SDK
- Session manager now accepts pool and workspace managers as dependencies
- Docker client supports workspace volume mounting
- Session create endpoint accepts optional `workspace_id` parameter
- SessionInfo includes `workspace_id` field
- **Documentation reorganized** into `docs/` folder with separate feature guides
- Main README streamlined with links to detailed docs

## [0.1.0] - 2026-02-07

### Added
- Initial implementation of Sandkasten sandbox runtime
- Go daemon with HTTP API
- Runner binary (PID 1 inside containers)
- Protocol types for runner ↔ daemon communication
- SQLite session store
- Docker wrapper for container management
- Session manager with per-session exec serialization
- Reaper for TTL-based garbage collection
- Reconciliation on daemon startup
- Base, Python, and Node sandbox images
- TypeScript SDK
- OpenAI Agents SDK example
- Comprehensive README with API documentation
- Taskfile for building all components
