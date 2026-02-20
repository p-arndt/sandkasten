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
import sys
from pathlib import Path

# Add SDK to path for development
sdk_path = Path(__file__).parent.parent.parent / "sdk" / "python"
sys.path.insert(0, str(sdk_path))

from agents import Agent, Runner, function_tool
from sandkasten import SandboxClient, Session

# Configuration
SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-sandbox-quickstart"

# Global state
sandbox_session: Session | None = None


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
    result = await sandbox_session.exec(cmd, timeout_ms=timeout_ms)

    parts = [
        f"exit_code={result.exit_code}",
        f"cwd={result.cwd}",
    ]
    if result.truncated:
        parts.append("(output truncated)")

    output = "\n".join(parts) + "\n---\n" + result.output

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
    await sandbox_session.write(path, content)

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
    content = await sandbox_session.read(path)

    print(f"  read: {path} ({len(content)} bytes)")
    return content.decode()


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
    global sandbox_session

    print("\n" + "="*60)
    print("Sandkasten Quickstart: Coding Agent")
    print("="*60 + "\n")

    # Create client and sandbox session
    client = SandboxClient(
        base_url=SANDKASTEN_URL,
        api_key=SANDKASTEN_API_KEY,
    )
    sandbox_session = await client.create_session(image="sandbox-runtime:python")

    # Print session info
    info = await sandbox_session.info()
    print(f"✓ Created session {info.id} (expires: {info.expires_at})")

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
        print(f"✓ Destroyed session {sandbox_session.id}")
        await sandbox_session.destroy()
        await client.close()

    print("\n" + "="*60)
    print("Done!")
    print("="*60 + "\n")


if __name__ == "__main__":
    asyncio.run(main())
