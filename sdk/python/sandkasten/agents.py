"""OpenAI Agents SDK integration for Sandkasten.

Provides pre-built @function_tool-compatible tools for the OpenAI Agents SDK.
Requires: pip install sandkasten[agents] or pip install openai-agents
"""

from __future__ import annotations

import shlex
from contextlib import asynccontextmanager
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from .client import SandboxClient
    from .session import Session


def _check_agents_installed() -> None:
    """Raise helpful ImportError if openai-agents is not installed."""
    try:
        import agents  # noqa: F401
    except ImportError as e:
        raise ImportError(
            "OpenAI Agents SDK is required for sandkasten.agents. "
            "Install with: pip install sandkasten[agents] or pip install openai-agents"
        ) from e


def _get_pydantic() -> Any:
    """Get Pydantic BaseModel - agents SDK uses pydantic for context."""
    try:
        from pydantic import BaseModel

        return BaseModel
    except ImportError:
        raise ImportError(
            "pydantic is required for sandkasten.agents (dependency of openai-agents). "
            "Install with: pip install sandkasten[agents]"
        ) from None


def _create_sandkasten_context_class() -> type:
    """Create SandkastenContext Pydantic model."""
    BaseModel = _get_pydantic()

    class SandkastenContext(BaseModel):
        """Context for Sandkasten tools in OpenAI Agents SDK.

        Holds the session (required) and optional workspace_id for multi-user tracking.
        """

        session: "Session"
        workspace_id: str | None = None

    return SandkastenContext


# Lazy initialization - populated on first use
_SandkastenContext: type | None = None
_sandkasten_tools: list | None = None


def _get_context_class() -> type:
    global _SandkastenContext
    if _SandkastenContext is None:
        _check_agents_installed()
        _SandkastenContext = _create_sandkasten_context_class()
    return _SandkastenContext




def _create_context_tools() -> list:
    """Create context-aware tools (use RunContextWrapper[SandkastenContext])."""
    from agents import RunContextWrapper, function_tool

    # Ensure RunContextWrapper is in module globals for get_type_hints resolution
    globals()["RunContextWrapper"] = RunContextWrapper
    ctx_class = _get_context_class()

    @function_tool
    async def sandbox_exec(
        ctx: RunContextWrapper[Any],
        cmd: str,
        timeout_ms: int = 30000,
    ) -> str:
        """Execute a shell command in the sandbox.

        The sandbox maintains a persistent bash shell: directory changes (cd),
        environment variables, and background processes persist across calls.

        Args:
            cmd: The shell command to execute (e.g. 'pip install pandas', 'python script.py').
            timeout_ms: Timeout in milliseconds (default 30000).

        Returns:
            String with exit_code, cwd, and command output separated by ---.
        """
        session = ctx.context.session
        result = await session.exec(cmd, timeout_ms=timeout_ms)
        return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

    @function_tool
    async def sandbox_write_file(
        ctx: RunContextWrapper[Any],
        path: str,
        content: str,
    ) -> str:
        """Write content to a file in the sandbox.

        Args:
            path: File path relative to /workspace or absolute (e.g. 'script.py', 'data/out.txt').
            content: The text content to write.

        Returns:
            Confirmation message with the path written.
        """
        session = ctx.context.session
        await session.write(path, content)
        return f"wrote {path}"

    @function_tool
    async def sandbox_read_file(
        ctx: RunContextWrapper[Any],
        path: str,
        max_bytes: int | None = None,
    ) -> str:
        """Read a file from the sandbox.

        Args:
            path: File path relative to /workspace or absolute.
            max_bytes: Maximum bytes to read (omit for full file).

        Returns:
            File contents as string. Indicates if truncated.
        """
        session = ctx.context.session
        result = await session.read(path, max_bytes=max_bytes)
        text = result.content.decode("utf-8", errors="replace")
        if result.truncated:
            text += "\n(truncated)"
        return text

    @function_tool
    async def sandbox_list_files(
        ctx: RunContextWrapper[Any],
        path: str = ".",
    ) -> str:
        """List files and directories in the sandbox.

        Args:
            path: Directory path to list (default '.' for current directory).

        Returns:
            Output of ls -la for the given path.
        """
        session = ctx.context.session
        cmd = "ls -la" if path in (".", "") else f"ls -la {shlex.quote(path)}"
        result = await session.exec(cmd)
        return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

    @function_tool
    async def sandbox_stats(ctx: RunContextWrapper[Any]) -> str:
        """Get resource usage statistics for the sandbox.

        Returns:
            Memory usage (bytes), memory limit, and CPU usage.
        """
        session = ctx.context.session
        stats = await session.stats()
        mem_mb = stats.memory_bytes / (1024 * 1024)
        limit_mb = stats.memory_limit / (1024 * 1024) if stats.memory_limit else 0
        return (
            f"memory_bytes={stats.memory_bytes} ({mem_mb:.2f} MB), "
            f"memory_limit={stats.memory_limit} ({limit_mb:.2f} MB), "
            f"cpu_usage_usec={stats.cpu_usage_usec}"
        )

    return [
        sandbox_exec,
        sandbox_write_file,
        sandbox_read_file,
        sandbox_list_files,
        sandbox_stats,
    ]


def sandkasten_tools() -> list:
    """Return context-aware tools for use with Agent[SandkastenContext].

    Tools expect RunContextWrapper[SandkastenContext] to be injected.
    Use with: Agent[SandkastenContext](..., tools=sandkasten_tools)
    and Runner.run(..., context=SandkastenContext(session=session))
    """
    global _sandkasten_tools
    if _sandkasten_tools is None:
        _check_agents_installed()
        _sandkasten_tools = _create_context_tools()
    return _sandkasten_tools


def create_sandkasten_tools(session: "Session") -> list:
    """Create session-bound tools (no context required).

    Returns a list of tools that use the given session. Suitable for
    Agent(tools=create_sandkasten_tools(session)) without context.

    Args:
        session: The sandbox session to bind tools to.

    Returns:
        List of @function_tool-decorated tools.
    """
    from agents import function_tool

    @function_tool
    async def sandbox_exec(cmd: str, timeout_ms: int = 30000) -> str:
        """Execute a shell command in the sandbox. Stateful: cd, env vars persist."""
        result = await session.exec(cmd, timeout_ms=timeout_ms)
        return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

    @function_tool
    async def sandbox_write_file(path: str, content: str) -> str:
        """Write content to a file in the sandbox."""
        await session.write(path, content)
        return f"wrote {path}"

    @function_tool
    async def sandbox_read_file(path: str, max_bytes: int | None = None) -> str:
        """Read a file from the sandbox."""
        result = await session.read(path, max_bytes=max_bytes)
        text = result.content.decode("utf-8", errors="replace")
        if result.truncated:
            text += "\n(truncated)"
        return text

    @function_tool
    async def sandbox_list_files(path: str = ".") -> str:
        """List files and directories in the sandbox."""
        cmd = "ls -la" if path in (".", "") else f"ls -la {shlex.quote(path)}"
        result = await session.exec(cmd)
        return f"exit_code={result.exit_code}\ncwd={result.cwd}\n---\n{result.output}"

    @function_tool
    async def sandbox_stats() -> str:
        """Get resource usage statistics for the sandbox."""
        stats = await session.stats()
        mem_mb = stats.memory_bytes / (1024 * 1024)
        limit_mb = stats.memory_limit / (1024 * 1024) if stats.memory_limit else 0
        return (
            f"memory_bytes={stats.memory_bytes} ({mem_mb:.2f} MB), "
            f"memory_limit={stats.memory_limit} ({limit_mb:.2f} MB), "
            f"cpu_usage_usec={stats.cpu_usage_usec}"
        )

    return [
        sandbox_exec,
        sandbox_write_file,
        sandbox_read_file,
        sandbox_list_files,
        sandbox_stats,
    ]


@asynccontextmanager
async def sandbox_tools_for_workspace(
    client: "SandboxClient",
    workspace_id: str,
    *,
    image: str = "python",
    ttl_seconds: int | None = None,
):
    """Async context manager for per-user workspace tools.

    Creates a session with the given workspace_id, yields (session, tools),
    and destroys the session on exit. Use for multi-user isolation.

    Args:
        client: SandboxClient instance.
        workspace_id: Workspace ID for persistent user storage (e.g. user_id or tenant-user_id).
        image: Image to use (default 'python').
        ttl_seconds: Session TTL (None for daemon default).

    Yields:
        Tuple of (session, tools). Tools are bound to the session.
    """
    session = await client.create_session(
        image=image,
        workspace_id=workspace_id,
        ttl_seconds=ttl_seconds,
    )
    try:
        tools = create_sandkasten_tools(session)
        yield session, tools
    finally:
        await session.destroy()


def __getattr__(name: str) -> Any:
    """Lazy load SandkastenContext (requires openai-agents)."""
    if name == "SandkastenContext":
        return _get_context_class()
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


__all__ = [
    "SandkastenContext",
    "sandkasten_tools",
    "create_sandkasten_tools",
    "sandbox_tools_for_workspace",
]
