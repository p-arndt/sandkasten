"""Multi-user workspace example — per-user isolation with Sandkasten.

Each user gets an isolated workspace. Files persist across sessions for the same
workspace_id, so user A never sees user B's files.

Requires: pip install sandkasten[agents]

Usage (from sdk/python):
  pip install sandkasten[agents]
  python examples/openai_agents/workspace_example.py
"""

import asyncio
import os

from agents import Agent, Runner
from sandkasten import SandboxClient, sandbox_tools_for_workspace


async def run_user_request(
    client: SandboxClient,
    user_id: str,
    request: str,
) -> str:
    """Run an agent request in a user-specific workspace."""
    async with sandbox_tools_for_workspace(
        client,
        workspace_id=user_id,
        image="python",
    ) as (session, tools):
        agent = Agent(
            name="coding-assistant",
            instructions="You have a Linux sandbox. Use the sandbox tools to execute commands and manage files.",
            tools=tools,
        )
        result = await Runner.run(agent, request)
        return result.final_output or ""


async def main() -> None:
    base_url = os.environ.get("SANDKASTEN_BASE_URL", "http://localhost:8080")
    api_key = os.environ.get("SANDKASTEN_API_KEY", "sk-sandbox-quickstart")

    client = SandboxClient(base_url=base_url, api_key=api_key)

    try:
        # User A: create a file in their workspace
        print("=== User alice: create greeting.py ===\n")
        out_a = await run_user_request(
            client,
            user_id="user-alice",
            request="Create a file greeting.py that prints 'Hello from Alice'. Run it and show the output.",
        )
        print(out_a)

        # User B: completely isolated workspace — does not see Alice's file
        print("\n=== User bob: list files (Alice's file not visible) ===\n")
        out_b = await run_user_request(
            client,
            user_id="user-bob",
            request="List files in the current directory with sandbox_list_files.",
        )
        print(out_b)

        # User A again: file still there (persistent workspace)
        print("\n=== User alice: file persists ===\n")
        out_a2 = await run_user_request(
            client,
            user_id="user-alice",
            request="Read greeting.py and run it.",
        )
        print(out_a2)

    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())
