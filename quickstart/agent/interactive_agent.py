#!/usr/bin/env python3
"""
Interactive Sandkasten Agent

Chat with the agent and watch it use the sandbox in real-time.

Usage:
    export OPENAI_API_KEY="sk-..."
    python interactive_agent.py
"""

import asyncio
import base64
import sys
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
    return data["id"]


async def destroy_session(sid: str) -> None:
    """Destroy a sandbox session."""
    await http_client.delete(f"/v1/sessions/{sid}")


# Tools

@function_tool
async def exec(cmd: str, timeout_ms: int = 30000) -> str:
    """Execute a shell command in the sandbox."""
    print(f"  → exec: {cmd}")
    resp = await http_client.post(
        f"/v1/sessions/{session_id}/exec",
        json={"cmd": cmd, "timeout_ms": timeout_ms},
    )
    resp.raise_for_status()
    data = resp.json()

    parts = [f"exit={data['exit_code']}", f"cwd={data['cwd']}"]
    if data.get("truncated"):
        parts.append("(truncated)")

    return "\n".join(parts) + "\n---\n" + data["output"]


@function_tool
async def write_file(path: str, content: str) -> str:
    """Write a file in the sandbox."""
    print(f"  → write: {path}")
    resp = await http_client.post(
        f"/v1/sessions/{session_id}/fs/write",
        json={"path": path, "text": content},
    )
    resp.raise_for_status()
    return f"wrote {path}"


@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox."""
    print(f"  → read: {path}")
    resp = await http_client.get(
        f"/v1/sessions/{session_id}/fs/read",
        params={"path": path},
    )
    resp.raise_for_status()
    data = resp.json()
    return base64.b64decode(data["content_base64"]).decode()


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
    global session_id

    print("\n" + "="*60)
    print("Interactive Sandkasten Agent")
    print("="*60)
    print("\nType your requests. Type 'quit' to exit.\n")

    session_id = await create_session()
    print(f"✓ Session {session_id} ready\n")

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
        await destroy_session(session_id)
        await http_client.aclose()
        print(f"\n✓ Session {session_id} destroyed\n")


if __name__ == "__main__":
    asyncio.run(main())
