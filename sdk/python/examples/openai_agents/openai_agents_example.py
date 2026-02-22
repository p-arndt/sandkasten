"""OpenAI Agents SDK + Sandkasten — minimal example.

Requires: pip install sandkasten[agents]

Set SANDKASTEN_BASE_URL and SANDKASTEN_API_KEY, or use defaults (localhost:8080).
"""

import asyncio
import os

from agents import Agent, Runner
from sandkasten import SandboxClient, create_sandkasten_tools


async def main() -> None:
    base_url = os.environ.get("SANDKASTEN_BASE_URL", "http://localhost:8080")
    api_key = os.environ.get("SANDKASTEN_API_KEY", "sk-sandbox-quickstart")

    client = SandboxClient(base_url=base_url, api_key=api_key)

    session = await client.create_session(image="python")
    tools = create_sandkasten_tools(session)

    agent = Agent(
        name="coding-assistant",
        instructions="You have a Linux sandbox. Use sandbox_exec, sandbox_write_file, sandbox_read_file to run commands and manage files.",
        tools=tools,
    )

    result = await Runner.run(
        agent,
        "Write a Python script that prints the first 5 Fibonacci numbers, then run it.",
    )

    print(result.final_output)

    await session.destroy()
    await client.close()


if __name__ == "__main__":
    asyncio.run(main())
