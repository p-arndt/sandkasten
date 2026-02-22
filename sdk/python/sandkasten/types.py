"""Type definitions for Sandkasten SDK."""

from dataclasses import dataclass
from typing import Literal


@dataclass
class ExecResult:
    """Result from executing a command in the sandbox."""

    exit_code: int
    """Exit code of the command (0 = success)"""

    cwd: str
    """Current working directory after command execution"""

    output: str
    """Combined stdout/stderr output"""

    truncated: bool
    """Whether output was truncated due to size limits"""

    duration_ms: int
    """Command execution duration in milliseconds"""


@dataclass
class ExecChunk:
    """Chunk of output from streaming command execution."""

    output: str
    """Output chunk text"""

    timestamp: int
    """Unix timestamp in milliseconds"""

    done: bool = False
    """Whether this is the final chunk"""

    exit_code: int = 0
    """Exit code (only set when done=True)"""

    cwd: str = ""
    """Current working directory (only set when done=True)"""

    duration_ms: int = 0
    """Execution duration (only set when done=True)"""


@dataclass
class SessionInfo:
    """Information about a sandbox session."""

    id: str
    """Unique session identifier"""

    image: str
    """Image used for this session"""

    status: str
    """Current session status (running, expired, destroyed, etc.)"""

    cwd: str
    """Current working directory"""

    created_at: str
    """ISO8601 timestamp of session creation"""

    expires_at: str
    """ISO8601 timestamp when session will expire"""

    workspace_id: str | None = None
    """Persistent workspace ID if attached (None for ephemeral)"""

    container_id: str | None = None
    """Container ID (if exposed by backend)"""

    last_activity: str | None = None
    """ISO8601 timestamp of last activity (if exposed by backend)"""


@dataclass
class SessionStats:
    """Resource usage statistics for a session."""

    memory_bytes: int
    """Current memory usage in bytes"""

    memory_limit: int
    """Memory limit in bytes (0 if not set)"""

    cpu_usage_usec: int
    """CPU usage in microseconds"""


@dataclass
class ReadResult:
    """Result from reading a file from the sandbox."""

    content: bytes
    """File content"""

    path: str
    """Path that was read"""

    truncated: bool
    """Whether output was truncated due to max_bytes limit"""


@dataclass
class WorkspaceInfo:
    """Information about a persistent workspace."""

    id: str
    """Workspace identifier"""


def _parse_session_info(data: dict) -> SessionInfo:
    """Parse API session dict into SessionInfo. Handles optional fields."""
    return SessionInfo(
        id=data["id"],
        image=data["image"],
        status=data["status"],
        cwd=data["cwd"],
        created_at=data["created_at"],
        expires_at=data["expires_at"],
        workspace_id=data.get("workspace_id") or None,
        container_id=data.get("container_id"),
        last_activity=data.get("last_activity"),
    )
