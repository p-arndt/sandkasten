# Using Sandkasten with the OpenAI Agents SDK

This guide shows how to give an [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/) agent access to a Sandkasten sandbox via **tools**: run shell commands, read files, and write files inside the sandbox.

## Prerequisites

> [!NOTE]
> The Sandkasten daemon must be running (see [Quickstart](./quickstart.md)). Use the same `api_key` and `default_image` (e.g. `python`) as in your config.

- Python 3.10+ with `openai-agents` and `sandkasten` installed

```bash
pip install openai-agents sandkasten
# or in your project
uv add openai-agents sandkasten
```

Set your OpenAI API key:

```bash
export OPENAI_API_KEY="sk-..."
```

## 1. Create a sandbox session

Use the Sandkasten Python client to create a session. The session is stateful: `cd`, environment variables, and background processes persist across tool calls.

```python
from sandkasten import SandboxClient

client = SandboxClient(
    base_url="http://localhost:8080",
    api_key="sk-test",
)

# One session per “conversation” or task
session = await client.create_session()
# Optional: use a persistent workspace so files survive session destruction
# session = await client.create_session(workspace_id="my-project")
```

## 2. Define tools that use the sandbox

Use `@function_tool` from the OpenAI Agents SDK. Each tool calls the Sandkasten session (e.g. `exec`, `read`, `write`). The agent will see these as callable tools.

```python
from agents import Agent, Runner, function_tool

# Session is set before running the agent (see below)
sandbox_session = None

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox.
    The shell is stateful: cd, env vars, and background processes persist."""
    result = await sandbox_session.exec(cmd, timeout_ms=timeout_ms)
    return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"


@function_tool
async def write_file(path: str, content: str) -> str:
    """Write text content to a file in the sandbox (path relative to /workspace or absolute)."""
    await sandbox_session.write(path, content)
    return f"wrote {path}"


@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox workspace."""
    content = await sandbox_session.read(path)
    return content.decode()
```

## 3. Create the agent and run

Create an `Agent` with these tools and run it with `Runner.run()`. Make sure the sandbox session is created and assigned to `sandbox_session` before the run.

```python
agent = Agent(
    name="coding-assistant",
    instructions="""You are a helpful coding assistant with access to a Linux sandbox.
    Use exec() to run shell commands (bash, python3, etc.).
    Use write_file(path, content) to create or overwrite files.
    Use read_file(path) to read file contents.
    The sandbox has Python 3, pip, and common tools. Be concise and run code to verify it works.""",
    tools=[exec, write_file, read_file],
)

async def main():
    global sandbox_session
    sandbox_session = await client.create_session()
    try:
        result = await Runner.run(
            agent,
            "Write a Python script that prints the first 10 Fibonacci numbers and run it.",
        )
        print(result.final_output)
    finally:
        await sandbox_session.destroy()
        await client.close()
```

## Full example (copy-paste)

```python
import asyncio
from agents import Agent, Runner, function_tool
from sandkasten import SandboxClient

SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-test"

client = SandboxClient(base_url=SANDKASTEN_URL, api_key=SANDKASTEN_API_KEY)
sandbox_session = None

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox. Stateful: cd, env, background processes persist."""
    result = await sandbox_session.exec(cmd, timeout_ms=timeout_ms)
    return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

@function_tool
async def write_file(path: str, content: str) -> str:
    """Write text to a file in the sandbox (path relative to /workspace or absolute)."""
    await sandbox_session.write(path, content)
    return f"wrote {path}"

@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox workspace."""
    content = await sandbox_session.read(path)
    return content.decode()

agent = Agent(
    name="coding-assistant",
    instructions="You have a Linux sandbox. Use exec(), write_file(), and read_file(). Be concise and run code to verify.",
    tools=[exec, write_file, read_file],
)

async def main():
    global sandbox_session
    sandbox_session = await client.create_session()
    try:
        result = await Runner.run(
            agent,
            "Create a small Python script that fetches https://httpbin.org/get and prints the JSON, then run it.",
        )
        print(result.final_output)
    finally:
        await sandbox_session.destroy()
        await client.close()

if __name__ == "__main__":
    asyncio.run(main())
```

Save as `agent_demo.py`, then:

```bash
export OPENAI_API_KEY="sk-..."
python agent_demo.py
```

## Streaming and conversation history

> [!TIP]
> - **Streaming:** Use `Runner.run_streamed()` for token-by-token output. See the [OpenAI Agents streaming guide](https://openai.github.io/openai-agents-python/streaming/).
> - **Sessions:** Use `SQLiteSession` so the agent keeps conversation context across turns. See the [Sessions guide](https://openai.github.io/openai-agents-python/sessions/).

## Example agents in this repo

The [quickstart/agent](../../quickstart/agent/) directory contains full examples:

- **enhanced_agent.py** — Interactive agent with Rich UI, streaming, SQLite history, and persistent workspaces
- **coding_agent.py** — Single-task run (create session → run task → destroy)
- **interactive_agent.py** — Simple interactive loop with tool feedback

Run the enhanced agent:

```bash
cd quickstart/agent
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

## Learn more

- [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/)
- [Sandkasten API reference](./api.md)
- [Sandkasten Python SDK](https://github.com/p-arndt/sandkasten/tree/main/sdk/python)
