#!/usr/bin/env python3
"""
Sandkasten Quickstart Example: Coding Agent

This agent has access to a stateful Linux sandbox with Python.
It can execute commands, write files, and read files.

Usage:
    export OPENAI_API_KEY="sk-..."
    python coding_agent.py
"""

import asyncio
import base64
import httpx
from agents import Agent, Runner, function_tool


# Configuration
SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-sandbox-quickstart"

# Global state
http_client = httpx.AsyncClient(
    base_url=SANDKASTEN_URL,
    headers={"Authorization": f"Bearer {SANDKASTEN_API_KEY}"},
    timeout=120.0,
)
session_id: str | None = None


async def create_session(image: str = "sandbox-runtime:python") -> str:
    """Create a new sandbox session."""
    resp = await http_client.post("/v1/sessions", json={"image": image})
    resp.raise_for_status()
    data = resp.json()
    print(f"✓ Created session {data['id']} (expires: {data['expires_at']})")
    return data["id"]


async def destroy_session(sid: str) -> None:
    """Destroy a sandbox session."""
    await http_client.delete(f"/v1/sessions/{sid}")
    print(f"✓ Destroyed session {sid}")


# Tools for the agent

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox.

    The sandbox is stateful — cd, environment variables, and background
    processes persist between calls.

    Args:
        cmd: Shell command to execute
        timeout_ms: Timeout in milliseconds (default 30000)

    Returns:
        Command output with exit code and current directory
    """
    resp = await http_client.post(
        f"/v1/sessions/{session_id}/exec",
        json={"cmd": cmd, "timeout_ms": timeout_ms},
    )
    resp.raise_for_status()
    data = resp.json()

    parts = [
        f"exit_code={data['exit_code']}",
        f"cwd={data['cwd']}",
    ]
    if data.get("truncated"):
        parts.append("(output truncated)")

    output = "\n".join(parts) + "\n---\n" + data["output"]

    # Log for visibility
    print(f"  exec: {cmd[:60]}{'...' if len(cmd) > 60 else ''}")

    return output


@function_tool
async def write_file(path: str, content: str) -> str:
    """Write text content to a file in the sandbox workspace.

    Args:
        path: File path (relative to /workspace or absolute)
        content: File content

    Returns:
        Confirmation message
    """
    resp = await http_client.post(
        f"/v1/sessions/{session_id}/fs/write",
        json={"path": path, "text": content},
    )
    resp.raise_for_status()

    print(f"  write: {path}")
    return f"wrote {path}"


@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox workspace.

    Args:
        path: File path (relative to /workspace or absolute)

    Returns:
        File contents
    """
    resp = await http_client.get(
        f"/v1/sessions/{session_id}/fs/read",
        params={"path": path},
    )
    resp.raise_for_status()
    data = resp.json()

    content = base64.b64decode(data["content_base64"]).decode()

    print(f"  read: {path} ({len(content)} bytes)")
    return content


# Define the agent

agent = Agent(
    name="coding-assistant",
    instructions="""You are a helpful coding assistant with access to a Linux sandbox.

Available tools:
- exec(cmd): Run shell commands (bash, python3, etc.)
- write_file(path, content): Write files
- read_file(path): Read files

The sandbox has:
- Python 3 with pip
- Common utilities (bash, coreutils)
- Persistent /workspace directory
- Stateful shell (cd, env vars, background processes persist)

Always:
1. Use exec to check what's available before writing code
2. Write clean, working code
3. Test your code by running it
4. Be concise but complete
""",
    tools=[exec, write_file, read_file],
)


async def main():
    global session_id

    print("\n" + "="*60)
    print("Sandkasten Quickstart: Coding Agent")
    print("="*60 + "\n")

    # Create sandbox session
    session_id = await create_session()

    try:
        # Run the agent
        task = (
            "Write a Python script that generates the first 20 Fibonacci numbers, "
            "save it to fib.py, and run it to show the output."
        )

        print(f"\nTask: {task}\n")
        print("-" * 60)

        result = await Runner.run(agent, task)

        print("-" * 60)
        print("\n✓ Agent completed the task\n")
        print("Final output:")
        print(result.final_output)

    finally:
        # Clean up
        await destroy_session(session_id)
        await http_client.aclose()

    print("\n" + "="*60)
    print("Done!")
    print("="*60 + "\n")


if __name__ == "__main__":
    asyncio.run(main())
