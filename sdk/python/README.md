# Sandkasten Python SDK

Python client for [Sandkasten](https://github.com/yourusername/sandkasten) — a self-hosted sandbox runtime for AI agents.

## Installation

```bash
pip install sandkasten
```

Or with uv:

```bash
uv add sandkasten
```

## Quick Start

```python
import asyncio
from sandkasten import SandboxClient

async def main():
    # Create client
    client = SandboxClient(
        base_url="http://localhost:8080",
        api_key="sk-sandbox-quickstart"
    )

    # Create a session
    session = await client.create_session(image="python")

    try:
        # Execute commands
        result = await session.exec("echo 'Hello from sandbox'")
        print(result.output)  # Hello from sandbox

        # Write a file
        await session.write("hello.py", "print('Hello, World!')")

        # Run it
        result = await session.exec("python3 hello.py")
        print(result.output)  # Hello, World!

        # Read files
        result = await session.read("hello.py")
        print(result.content.decode())

    finally:
        # Clean up
        await session.destroy()
        await client.close()

asyncio.run(main())
```

## Usage with Context Managers

```python
async with SandboxClient(base_url="...", api_key="...") as client:
    async with await client.create_session() as session:
        result = await session.exec("pip install requests")
        # Session automatically destroyed on exit
```

## API Reference

### `SandboxClient`

Main client for managing sandbox sessions.

#### `__init__(*, base_url: str, api_key: str, timeout: float = 120.0)`

Create a new client.

- **base_url**: URL of Sandkasten daemon (e.g., `http://localhost:8080`)
- **api_key**: API key for authentication
- **timeout**: HTTP timeout in seconds

#### `async create_session(*, image: str = "python", ttl_seconds: int | None = None, workspace_id: str | None = None) -> Session`

Create a new sandbox session.

- **image**: Image name (`base`, `python`, `node`)
- **ttl_seconds**: Session lifetime (None = daemon default)
- **workspace_id**: Persistent workspace ID (None = ephemeral)

#### `async get_session(session_id: str) -> Session`

Get an existing session by ID.

#### `async list_sessions() -> list[SessionInfo]`

List all active sessions.

#### `async close()`

Close the HTTP client.

---

### `Session`

A stateful sandbox session with persistent shell and filesystem.

#### `async exec(cmd: str, *, timeout_ms: int = 30000) -> ExecResult`

Execute a shell command.

**Stateful**: Directory changes, environment variables, and background processes persist.

```python
await session.exec("cd /tmp")
result = await session.exec("pwd")
print(result.cwd)  # /tmp
```

Returns `ExecResult`:
- `exit_code: int` — Exit code (0 = success)
- `cwd: str` — Current working directory
- `output: str` — Combined stdout/stderr
- `truncated: bool` — Whether output was truncated
- `duration_ms: int` — Execution time in milliseconds

#### `async write(path: str, content: str | bytes)`

Write content to a file.

```python
await session.write("script.py", "print('hello')")
await session.write("data.bin", b"\x00\x01\x02")
```

#### `async read(path: str, *, max_bytes: int | None = None) -> ReadResult`

Read a file. Returns `ReadResult` with `content`, `path`, and `truncated` fields.

```python
result = await session.read("output.txt")
print(result.content.decode())
if result.truncated:
    print("(output was truncated)")
```

#### `async upload(file: str | Path | BinaryIO, *, dest_path: str = "/workspace", filename: str | None = None) -> list[str]`

Upload a file via multipart form. Returns list of uploaded paths.

#### `async stats() -> SessionStats`

Get resource usage (memory_bytes, memory_limit, cpu_usage_usec).

#### `async info() -> SessionInfo`

Get session metadata (status, expiry, etc.).

#### `async destroy()`

Destroy the session and clean up resources.

---

## Using with AI Agent Frameworks

### OpenAI Agents SDK

```python
from agents import Agent, Runner, function_tool
from sandkasten import SandboxClient

client = SandboxClient(base_url="...", api_key="...")
session = None

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox."""
    result = await session.exec(cmd, timeout_ms=timeout_ms)
    return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

@function_tool
async def write_file(path: str, content: str) -> str:
    """Write content to a file."""
    await session.write(path, content)
    return f"wrote {path}"

@function_tool
async def read_file(path: str) -> str:
    """Read a file."""
    result = await session.read(path)
    return result.content.decode()

agent = Agent(
    name="coding-assistant",
    instructions="You have a Linux sandbox with exec, write_file, read_file tools.",
    tools=[exec, write_file, read_file],
)

async def main():
    global session
    session = await client.create_session(image="python")
    try:
        result = await Runner.run(
            agent,
            "Write a Python script that prints fibonacci numbers"
        )
        print(result.final_output)
    finally:
        await session.destroy()
        await client.close()
```

### LangChain

```python
from langchain.tools import tool
from sandkasten import SandboxClient

client = SandboxClient(base_url="...", api_key="...")
session = None

@tool
async def sandbox_exec(cmd: str) -> str:
    """Execute a command in the sandbox."""
    result = await session.exec(cmd)
    return result.output

# Use with LangChain agents...
```

## Available Images

- `base` — Minimal Ubuntu with bash, coreutils
- `python` — Python 3 with pip, uv, common packages (requests, httpx, pandas, numpy, matplotlib, beautifulsoup4, etc.)
- `node` — Node.js 22 with npm

## Error Handling

All methods raise `httpx.HTTPError` on failure. Stream errors raise `SandkastenStreamError`:

```python
from sandkasten import SandboxClient, SandkastenStreamError

try:
    result = await session.exec("invalid-command")
except httpx.HTTPStatusError as e:
    print(f"HTTP {e.response.status_code}: {e.response.text}")

try:
    async for chunk in session.exec_stream("fail-cmd"):
        ...
except SandkastenStreamError as e:
    print(f"Stream error: {e}")
```

## Development

```bash
# Clone repo
git clone https://github.com/yourusername/sandkasten
cd sandkasten/sdk/python

# Install with uv
uv sync

# Install dev dependencies and run tests
uv sync --extra dev
pytest
```

## License

MIT
