#!/usr/bin/env python3
"""
Enhanced Interactive Sandkasten Agent

Features:
- Rich terminal UI with boxes and colors
- Streaming responses (token-by-token)
- Conversation history with SQLite
- Visual tool execution feedback
- Message history display

Usage:
    export OPENAI_API_KEY="sk-..."
    uv run enhanced_agent.py
"""

import asyncio
import os
import sys
from datetime import datetime
from pathlib import Path

# Add SDK to path for development
sdk_path = Path(__file__).parent.parent.parent / "sdk" / "python"
sys.path.insert(0, str(sdk_path))

from agents import Agent, Runner, SQLiteSession, function_tool
from openai.types.responses import ResponseTextDeltaEvent
from rich.console import Console
from rich.panel import Panel
from rich.markdown import Markdown
from rich.table import Table
from rich.live import Live
from rich import box

from sandkasten import SandboxClient, Session

# Configuration
SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-sandbox-quickstart"
SESSION_DB = "conversation_history.db"

# Global state
console = Console()
client: SandboxClient | None = None
sandbox_session: Session | None = None
conversation_session: SQLiteSession | None = None


# Tools with visual feedback

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
    console.print(f"[dim]  → exec:[/dim] [cyan]{cmd}[/cyan]")

    result = await sandbox_session.exec(cmd, timeout_ms=timeout_ms)

    parts = [
        f"exit_code={result.exit_code}",
        f"cwd={result.cwd}",
    ]
    if result.truncated:
        parts.append("(output truncated)")

    return "\n".join(parts) + "\n---\n" + result.output


@function_tool
async def write_file(path: str, content: str) -> str:
    """Write text content to a file in the sandbox workspace.

    Args:
        path: File path (relative to /workspace or absolute)
        content: File content

    Returns:
        Confirmation message
    """
    console.print(f"[dim]  → write:[/dim] [green]{path}[/green] [dim]({len(content)} bytes)[/dim]")

    await sandbox_session.write(path, content)

    return f"wrote {path}"


@function_tool
async def read_file(path: str) -> str:
    """Read a file from the sandbox workspace.

    Args:
        path: File path (relative to /workspace or absolute)

    Returns:
        File contents
    """
    console.print(f"[dim]  → read:[/dim] [yellow]{path}[/yellow]")

    content = await sandbox_session.read(path)

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
- Python 3 with pip and uv (fast package manager)
- Pre-installed: requests, httpx, pandas, numpy, matplotlib, beautifulsoup4, pyyaml
- Development tools: git, curl, wget, jq
- Persistent /workspace directory
- Stateful shell (cd, env vars, background processes persist)
- Full internet access

Package management:
- Use 'pip install <package>' for quick installs
- Use 'uv pip install <package>' for faster installs
- Use 'uv venv && source .venv/bin/activate' to create virtual environments

Always:
1. Be helpful and thorough
2. Write clean, working code
3. Test your code by running it
4. Explain what you're doing
5. Use pre-installed packages when possible to save time
""",
    tools=[exec, write_file, read_file],
)


def print_header():
    """Print welcome header."""
    header = Panel.fit(
        "[bold cyan]Sandkasten Interactive Agent[/bold cyan]\n"
        "[dim]Streaming • History • Rich UI[/dim]",
        box=box.DOUBLE,
        border_style="cyan",
    )
    console.print(header)
    console.print()


def print_session_info(session: Session):
    """Print session information."""
    info = Table.grid(padding=(0, 2))
    info.add_column(style="dim")
    info.add_column()
    info.add_row("Sandbox:", f"[green]{session.id}[/green]")
    info.add_row("Image:", "[cyan]sandbox-runtime:python[/cyan]")
    info.add_row("Network:", "[yellow]full[/yellow]")
    info.add_row("Packages:", "[dim]requests, httpx, pandas, numpy, matplotlib, bs4, yaml[/dim]")
    info.add_row("Tools:", "[dim]python3, pip, uv, git, curl, wget, jq[/dim]")

    console.print(Panel(info, title="Session", border_style="dim", box=box.ROUNDED))
    console.print()


def print_help():
    """Print command help."""
    help_text = """
[bold]Commands:[/bold]
  • Type your message and press Enter
  • [cyan]/history[/cyan] - Show conversation history
  • [cyan]/clear[/cyan] - Clear conversation history
  • [cyan]/help[/cyan] - Show this help
  • [cyan]/quit[/cyan] or [cyan]/exit[/cyan] - Exit
    """
    console.print(Panel(help_text.strip(), title="Help", border_style="blue", box=box.ROUNDED))
    console.print()


def _item_display_text(item: dict) -> str:
    """Extract displayable text from a session item (stored as dict/JSON)."""
    kind = item.get("type")
    if kind == "input_text":
        return item.get("text") or ""
    if kind == "message":
        content = item.get("content")
        if isinstance(content, str):
            return content
        if isinstance(content, list):
            parts = []
            for part in content:
                if isinstance(part, dict):
                    parts.append(part.get("text") or part.get("input", ""))
                else:
                    parts.append(str(part))
            return "\n".join(parts)
        return ""
    return str(item)


async def show_history():
    """Display conversation history."""
    if not conversation_session:
        console.print("[yellow]No conversation history available.[/yellow]")
        return

    history = await conversation_session.get_items()
    if not history:
        console.print("[dim]No messages yet.[/dim]")
        return

    console.print()
    for i, item in enumerate(history, 1):
        # Items from get_items() are dicts (JSON from DB)
        if not isinstance(item, dict):
            continue
        kind = item.get("type")
        role = item.get("role")
        text = _item_display_text(item)
        if not text.strip():
            continue
        if kind == "input_text" or (kind == "message" and role == "user"):
            console.print(Panel(
                text,
                title=f"[bold cyan]You[/bold cyan] (message {i})",
                border_style="cyan",
                box=box.ROUNDED,
            ))
        elif kind == "message" and role == "assistant":
            console.print(Panel(
                Markdown(text),
                title=f"[bold green]Assistant[/bold green] (message {i})",
                border_style="green",
                box=box.ROUNDED,
            ))
    console.print()


async def run_with_streaming(user_input: str):
    """Run agent with streaming and visual feedback."""
    console.print()

    # Buffer for collecting tool calls and response
    tool_calls = []
    response_text = ""
    live_context = None

    # Start streaming
    result = Runner.run_streamed(
        agent,
        input=user_input,
        session=conversation_session,
    )

    async for event in result.stream_events():
        if event.type == "run_item_stream_event":
            # Tool calls happen first
            if event.item.type == "tool_call_item":
                tool_name = getattr(event.item.raw_item, 'name', 'unknown')
                tool_calls.append(tool_name)

        elif event.type == "raw_response_event":
            # Response text comes after tools
            if isinstance(event.data, ResponseTextDeltaEvent):
                # First token - display buffered tool calls and start live update
                if live_context is None:
                    # Display tool calls first
                    if tool_calls:
                        for tool in tool_calls:
                            console.print(f"[dim]  → tool:[/dim] [magenta]{tool}[/magenta]")
                        console.print()

                    # Start live streaming display
                    live_context = Live(console=console, refresh_per_second=10)
                    live_context.start()

                response_text += event.data.delta

                # Update live display
                if live_context:
                    live_context.update(Panel(
                        Markdown(response_text),
                        title="[bold green]Assistant[/bold green]",
                        border_style="green",
                        box=box.ROUNDED,
                    ))

    # Stop live display
    if live_context:
        live_context.stop()

    console.print()


async def interactive_loop():
    """Main interactive loop."""
    global client, sandbox_session, conversation_session

    print_header()

    # Create client and sandbox session
    with console.status("[cyan]Creating sandbox session...[/cyan]"):
        client = SandboxClient(
            base_url=SANDKASTEN_URL,
            api_key=SANDKASTEN_API_KEY,
        )
        sandbox_session = await client.create_session(image="sandbox-runtime:python")

    print_session_info(sandbox_session)

    # Create conversation session
    user_id = f"user_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    conversation_session = SQLiteSession(user_id, SESSION_DB)

    console.print("[dim]Type /help for commands, /quit to exit[/dim]\n")

    try:
        while True:
            # Get user input
            try:
                user_input = console.input("[bold cyan]You:[/bold cyan] ").strip()
            except EOFError:
                break

            if not user_input:
                continue

            # Handle commands
            if user_input.lower() in ("/quit", "/exit", "/q"):
                break
            elif user_input.lower() == "/help":
                print_help()
                continue
            elif user_input.lower() == "/history":
                await show_history()
                continue
            elif user_input.lower() == "/clear":
                await conversation_session.clear_session()
                console.print("[yellow]Conversation history cleared.[/yellow]\n")
                continue

            # Run agent with streaming
            await run_with_streaming(user_input)

    except KeyboardInterrupt:
        console.print("\n\n[yellow]Interrupted.[/yellow]")
    finally:
        # Clean up
        console.print()
        with console.status("[dim]Cleaning up...[/dim]"):
            if sandbox_session:
                await sandbox_session.destroy()
            if client:
                await client.close()

        console.print(Panel(
            f"[green]✓[/green] Session [cyan]{sandbox_session.id if sandbox_session else 'unknown'}[/cyan] destroyed\n"
            f"[dim]Conversation history saved to {SESSION_DB}[/dim]",
            border_style="dim",
            box=box.ROUNDED,
        ))
        console.print()


async def main():
    """Entry point."""
    # Check OpenAI API key
    if not os.getenv("OPENAI_API_KEY"):
        console.print("[red]Error: OPENAI_API_KEY environment variable not set.[/red]")
        console.print("[dim]Set it with: export OPENAI_API_KEY='sk-...'[/dim]")
        sys.exit(1)

    await interactive_loop()


if __name__ == "__main__":
    asyncio.run(main())
