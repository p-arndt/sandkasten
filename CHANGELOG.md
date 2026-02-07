# Changelog

## [Unreleased] - 2026-02-08

### Added
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
- Makefile for building all components
