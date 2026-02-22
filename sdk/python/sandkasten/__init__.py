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
