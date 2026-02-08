"""Sandbox session management."""

import base64
from typing import TYPE_CHECKING

import httpx

from .types import ExecResult, SessionInfo

if TYPE_CHECKING:
    from .client import SandboxClient


class Session:
    """A stateful sandbox session with persistent shell and filesystem.

    Sessions maintain:
    - Persistent bash shell (cd, environment variables, background processes)
    - Filesystem state in /workspace
    - Current working directory across exec calls

    Example:
        >>> session = await client.create_session(image="sandbox-runtime:python")
        >>> result = await session.exec("echo 'Hello from sandbox'")
        >>> print(result.output)
        Hello from sandbox
        >>> await session.destroy()
    """

    def __init__(self, client: "SandboxClient", session_id: str):
        """Internal: use SandboxClient.create_session() instead."""
        self._client = client
        self._id = session_id

    @property
    def id(self) -> str:
        """Unique session identifier."""
        return self._id

    async def exec(
        self,
        cmd: str,
        *,
        timeout_ms: int = 30000,
    ) -> ExecResult:
        """Execute a shell command in the sandbox.

        The sandbox maintains a persistent bash shell, so:
        - Directory changes (cd) persist
        - Environment variables persist
        - Background processes remain running

        Args:
            cmd: Shell command to execute
            timeout_ms: Timeout in milliseconds (default 30000)

        Returns:
            ExecResult with exit code, output, and current directory

        Raises:
            httpx.HTTPError: If the request fails

        Example:
            >>> result = await session.exec("cd /tmp && pwd")
            >>> print(result.cwd)
            /tmp
            >>> result = await session.exec("pwd")  # Still in /tmp
            >>> print(result.output)
            /tmp
        """
        resp = await self._client._http.post(
            f"/v1/sessions/{self._id}/exec",
            json={"cmd": cmd, "timeout_ms": timeout_ms},
        )
        resp.raise_for_status()
        data = resp.json()

        return ExecResult(
            exit_code=data["exit_code"],
            cwd=data["cwd"],
            output=data["output"],
            truncated=data.get("truncated", False),
            duration_ms=data.get("duration_ms", 0),
        )

    async def write(
        self,
        path: str,
        content: str | bytes,
    ) -> None:
        """Write content to a file in the sandbox.

        Args:
            path: File path (relative to /workspace or absolute)
            content: File content (string or bytes)

        Raises:
            httpx.HTTPError: If the request fails

        Example:
            >>> await session.write("hello.py", "print('Hello, World!')")
            >>> result = await session.exec("python3 hello.py")
            >>> print(result.output)
            Hello, World!
        """
        if isinstance(content, str):
            content = content.encode("utf-8")

        resp = await self._client._http.post(
            f"/v1/sessions/{self._id}/fs/write",
            json={
                "path": path,
                "content_base64": base64.b64encode(content).decode("ascii"),
            },
        )
        resp.raise_for_status()

    async def read(
        self,
        path: str,
        *,
        max_bytes: int | None = None,
    ) -> bytes:
        """Read a file from the sandbox.

        Args:
            path: File path (relative to /workspace or absolute)
            max_bytes: Maximum bytes to read (None for no limit)

        Returns:
            File content as bytes

        Raises:
            httpx.HTTPError: If the request fails or file doesn't exist

        Example:
            >>> content = await session.read("output.txt")
            >>> print(content.decode())
            File contents here
        """
        params = {"path": path}
        if max_bytes is not None:
            params["max_bytes"] = max_bytes

        resp = await self._client._http.get(
            f"/v1/sessions/{self._id}/fs/read",
            params=params,
        )
        resp.raise_for_status()
        data = resp.json()

        return base64.b64decode(data["content_base64"])

    async def info(self) -> SessionInfo:
        """Get current session information.

        Returns:
            SessionInfo with status, expiry, and metadata

        Example:
            >>> info = await session.info()
            >>> print(f"Session expires at {info.expires_at}")
        """
        resp = await self._client._http.get(f"/v1/sessions/{self._id}")
        resp.raise_for_status()
        data = resp.json()

        return SessionInfo(
            id=data["id"],
            image=data["image"],
            container_id=data["container_id"],
            status=data["status"],
            cwd=data["cwd"],
            created_at=data["created_at"],
            expires_at=data["expires_at"],
            last_activity=data["last_activity"],
        )

    async def destroy(self) -> None:
        """Destroy the sandbox session and clean up resources.

        This stops the container and releases all resources.
        The session cannot be used after calling destroy().

        Example:
            >>> await session.destroy()
        """
        resp = await self._client._http.delete(f"/v1/sessions/{self._id}")
        resp.raise_for_status()

    async def __aenter__(self) -> "Session":
        """Context manager entry."""
        return self

    async def __aexit__(self, *args) -> None:
        """Context manager exit - automatically destroys session."""
        await self.destroy()
