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
class SessionInfo:
    """Information about a sandbox session."""

    id: str
    """Unique session identifier"""

    image: str
    """Docker image used for this session"""

    container_id: str
    """Docker container ID"""

    status: Literal["running", "expired", "destroyed"]
    """Current session status"""

    cwd: str
    """Current working directory"""

    created_at: str
    """ISO8601 timestamp of session creation"""

    expires_at: str
    """ISO8601 timestamp when session will expire"""

    last_activity: str
    """ISO8601 timestamp of last activity"""
