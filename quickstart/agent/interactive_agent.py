#!/usr/bin/env python3
"""
Interactive Sandkasten Agent

Chat with the agent and watch it use the sandbox in real-time.

Usage:
    export OPENAI_API_KEY="sk-..."
    python interactive_agent.py
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


# Tools

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox."""
    print(f"  → exec: {cmd}")
    result = await sandbox_session.exec(cmd, timeout_ms=timeout_ms)

    parts = [f"exit={result.exit_code}", f"cwd={result.cwd}"]
    if result.truncated:
        parts.append("(truncated)")

    return "\n".join(parts) + "\n---\n" + result.output


@function_tool
async def write_file(path: str, content: str) -> str:
    """Write a file in the sandbox."""
    print(f"  → write: {path}")
    await sandbox_session.write(path, content)
    return f"wrote {path}"


@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox."""
    print(f"  → read: {path}")
    content = await sandbox_session.read(path)
    return content.decode()


# Agent

agent = Agent(
    name="assistant",
    instructions="""You are a helpful assistant with a Linux sandbox.

You can:
- Run commands with exec(cmd)
- Write files with write_file(path, content)
- Read files with read_file(path)

The sandbox has Python 3, bash, and common utilities.
Be helpful and thorough.""",
    tools=[exec, write_file, read_file],
)


async def main():
    global sandbox_session

    print("\n" + "="*60)
    print("Interactive Sandkasten Agent")
    print("="*60)
    print("\nType your requests. Type 'quit' to exit.\n")

    client = SandboxClient(
        base_url=SANDKASTEN_URL,
        api_key=SANDKASTEN_API_KEY,
    )
    sandbox_session = await client.create_session(image="sandbox-runtime:python")
    print(f"✓ Session {sandbox_session.id} ready\n")

    try:
        while True:
            try:
                user_input = input("You: ").strip()
            except EOFError:
                break

            if not user_input:
                continue

            if user_input.lower() in ("quit", "exit", "q"):
                break

            print()

            result = await Runner.run(agent, user_input)

            print(f"\nAgent: {result.final_output}\n")

    except KeyboardInterrupt:
        print("\n\nInterrupted.")
    finally:
        await sandbox_session.destroy()
        await client.close()
        print(f"\n✓ Session {sandbox_session.id} destroyed\n")


if __name__ == "__main__":
    asyncio.run(main())
