"""Sandkasten Python SDK — Agent sandbox runtime client."""

from importlib.metadata import version

from .client import SandboxClient
from .exceptions import SandkastenAPIError, SandkastenError, SandkastenStreamError
from .session import Session
from .types import (
    ExecChunk,
    ExecResult,
    ReadResult,
    SessionInfo,
    SessionStats,
    WorkspaceInfo,
)

__version__ = version(__name__)
__all__ = [
    "SandboxClient",
    "Session",
    "ExecResult",
    "ExecChunk",
    "SessionInfo",
    "SessionStats",
    "ReadResult",
    "WorkspaceInfo",
    "SandkastenError",
    "SandkastenAPIError",
    "SandkastenStreamError",
]


def __getattr__(name: str):
    """Lazy import for agents integration (requires sandkasten[agents])."""
    if name in ("SandkastenContext", "sandkasten_tools", "create_sandkasten_tools", "sandbox_tools_for_workspace"):
        from . import agents as _agents

        return getattr(_agents, name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
