# 🏖️ Sandkasten

**Self-hosted sandbox runtime for AI agents.** Stateful Linux containers with persistent shell, file operations, and workspace management.

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
- ✅ **File Operations** - Read/write files in `/workspace`
- ✅ **Multiple Runtimes** - Python, Node.js, or custom images
- ✅ **Persistent Workspaces** - Volumes that survive session destruction
- ✅ **Pre-warmed Pool** - Instant session creation (~50ms vs 2-3s)
- ✅ **Web Dashboard** - Monitor sessions, edit config
- ✅ **Python + TypeScript SDKs** - Clean async APIs
- ✅ **Agent-Ready** - Works with OpenAI Agents SDK, LangChain, etc.

## Quick Start

### 1. Start Daemon

```bash
# Docker Compose (easiest)
cd quickstart/daemon && docker-compose up -d

# Or build from source
make build && ./sandkasten --config sandkasten.yaml
```

### 2. Run Example Agent

```bash
cd quickstart/agent
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

### 3. Open Dashboard

```
http://localhost:8080
```

## Documentation

- [Quickstart Guide](./docs/quickstart.md) - Get running in 5 minutes
- [API Reference](./docs/api.md) - Complete HTTP API docs
- [Configuration](./docs/configuration.md) - Config options and security
- [Features](./docs/features/) - Workspaces, pooling, streaming
- [Agent Examples](./quickstart/agent/) - OpenAI Agents SDK examples

## Architecture

```
┌─────────────────────────────────────────────┐
│              Sandkasten Daemon              │
├─────────────────────────────────────────────┤
│  HTTP API → Session Manager → Docker Engine │
│     ↓            ↓               ↓          │
│  Sessions    Workspace        Container     │
│  (SQLite)    Volumes          Pool          │
└─────────────────────────────────────────────┘
         │
         ↓
┌─────────────────────┐   ┌─────────────────────┐
│  Container (Python) │   │  Container (Node)   │
│  ┌───────────────┐  │   │  ┌───────────────┐  │
│  │ Runner (PID1) │  │   │  │ Runner (PID1) │  │
│  │ ↓             │  │   │  │ ↓             │  │
│  │ bash -l (PTY) │  │   │  │ bash -l (PTY) │  │
│  └───────────────┘  │   │  └───────────────┘  │
│  /workspace/        │   │  /workspace/        │
└─────────────────────┘   └─────────────────────┘
```

## Use Cases

### AI Coding Agents

```python
agent = Agent(
    name="coding-assistant",
    instructions="You have a Linux sandbox...",
    tools=[exec, write_file, read_file],
)
```

### Data Analysis

```python
await session.exec("pip install pandas matplotlib")
await session.write("analysis.py", script)
result = await session.exec("python3 analysis.py")
```

### Package Testing

```python
await session.exec("pip install my-package")
await session.exec("pytest tests/")
```

### Education & Tutorials

```python
# Safe execution environment for user code
await session.write("user_code.py", user_submitted_code)
result = await session.exec("python3 user_code.py", timeout_ms=5000)
```

## Installation

### Requirements

- Docker Engine
- Go 1.24+ (for building from source)

### Build

```bash
git clone https://github.com/yourusername/sandkasten
cd sandkasten
make build
```

This builds:
- `sandkasten` daemon binary
- `runner` binary (embedded in images)
- Docker images: `sandbox-runtime:base`, `:python`, `:node`

## Configuration

Minimal `sandkasten.yaml`:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-your-secret-key"
```

Full example with all features:

```yaml
# See quickstart/daemon/sandkasten-full.yaml
workspace:
  enabled: true
pool:
  enabled: true
  images:
    sandbox-runtime:python: 3
```

See [Configuration Guide](./docs/configuration.md) for details.

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

[Python SDK Docs](./sdk/python/README.md)

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

[TypeScript SDK Docs](./sdk/README.md)

## Examples

See [quickstart/agent/](./quickstart/agent/) for complete examples:

- **enhanced_agent.py** - Full-featured with Rich UI, streaming, history
- **coding_agent.py** - Simple task-based agent
- **interactive_agent.py** - Interactive REPL

## Security

Sandboxes are isolated with:
- ✅ Read-only root filesystem
- ✅ No capabilities (CAP_DROP=ALL)
- ✅ PID/CPU/memory limits
- ✅ No new privileges
- ✅ Optional network isolation

For production:
- Use strong API keys
- Bind to localhost (use reverse proxy)
- Enable network isolation
- Set resource limits
- Regular security updates

See [Configuration Guide](./docs/configuration.md) for details.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## License

MIT - See [LICENSE](./LICENSE) for details.

## Credits

Built with ❤️ using:
- [Docker Engine API](https://docs.docker.com/engine/api/)
- [creack/pty](https://github.com/creack/pty) for PTY management
- [modernc.org/sqlite](https://modernc.org/sqlite) for pure-Go SQLite

## Links

- **Documentation**: [docs/](./docs/)
- **Examples**: [quickstart/agent/](./quickstart/agent/)
- **Issues**: [GitHub Issues](https://github.com/yourusername/sandkasten/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/sandkasten/discussions)
