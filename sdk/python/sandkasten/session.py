"""Sandbox session management."""

import base64
import json
from pathlib import Path
from typing import TYPE_CHECKING, AsyncIterator, BinaryIO

import httpx

from .exceptions import SandkastenStreamError
from .types import ExecChunk, ExecResult, ReadResult, SessionInfo, SessionStats
from .types import _parse_session_info

if TYPE_CHECKING:
    from .client import SandboxClient


class Session:
    """A stateful sandbox session with persistent shell and filesystem.

    Sessions maintain:
    - Persistent bash shell (cd, environment variables, background processes)
    - Filesystem state in /workspace
    - Current working directory across exec calls

    Example:
        >>> session = await client.create_session(image="python")
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
        raw_output: bool = False,
    ) -> ExecResult:
        """Execute a shell command in the sandbox.

        The sandbox maintains a persistent bash shell, so:
        - Directory changes (cd) persist
        - Environment variables persist
        - Background processes remain running

        Args:
            cmd: Shell command to execute
            timeout_ms: Timeout in milliseconds (default 30000)
            raw_output: Return raw PTY output (keeps ANSI and CRLF)

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
            json={"cmd": cmd, "timeout_ms": timeout_ms, "raw_output": raw_output},
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

    async def exec_stream(
        self,
        cmd: str,
        *,
        timeout_ms: int = 30000,
        raw_output: bool = False,
    ) -> AsyncIterator[ExecChunk]:
        """Execute a command and stream output chunks in real-time.

        The sandbox maintains a persistent bash shell, so:
        - Directory changes (cd) persist
        - Environment variables persist
        - Background processes remain running

        Args:
            cmd: Shell command to execute
            timeout_ms: Timeout in milliseconds (default 30000)
            raw_output: Return raw PTY output (keeps ANSI and CRLF)

        Yields:
            ExecChunk objects with output and metadata
            Final chunk has done=True with exit code and cwd

        Raises:
            httpx.HTTPError: If the request fails

        Example:
            >>> async for chunk in session.exec_stream("pip install pandas"):
            ...     print(chunk.output, end='', flush=True)
            ...     if chunk.done:
            ...         print(f"\\nExit code: {chunk.exit_code}")
        """
        async with self._client._http.stream(
            "POST",
            f"/v1/sessions/{self._id}/exec/stream",
            json={"cmd": cmd, "timeout_ms": timeout_ms, "raw_output": raw_output},
        ) as resp:
            resp.raise_for_status()

            event_type = ""
            async for line in resp.aiter_lines():
                if not line.strip():
                    continue

                # Parse SSE format: "event: <type>\ndata: <json>"
                if line.startswith("event: "):
                    event_type = line[7:].strip()
                    continue

                if line.startswith("data: "):
                    data_json = line[6:].strip()
                    data = json.loads(data_json)

                    if event_type == "chunk":
                        yield ExecChunk(
                            output=data.get("chunk", ""),
                            timestamp=data.get("timestamp", 0),
                            done=False,
                        )
                    elif event_type == "done":
                        yield ExecChunk(
                            output="",
                            timestamp=0,
                            done=True,
                            exit_code=data.get("exit_code", 0),
                            cwd=data.get("cwd", ""),
                            duration_ms=data.get("duration_ms", 0),
                        )
                    elif event_type == "error":
                        raise SandkastenStreamError(data.get("error", "Unknown error"))

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
    ) -> ReadResult:
        """Read a file from the sandbox.

        Args:
            path: File path (relative to /workspace or absolute)
            max_bytes: Maximum bytes to read (None for no limit)

        Returns:
            ReadResult with content, path, and truncated flag

        Raises:
            httpx.HTTPError: If the request fails or file doesn't exist

        Example:
            >>> result = await session.read("output.txt")
            >>> print(result.content.decode())
            >>> if result.truncated:
            ...     print("(output was truncated)")
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

        content = base64.b64decode(data["content_base64"])
        return ReadResult(
            content=content,
            path=data.get("path", path),
            truncated=data.get("truncated", False),
        )

    async def upload(
        self,
        file: str | Path | BinaryIO,
        *,
        dest_path: str = "/workspace",
        filename: str | None = None,
    ) -> list[str]:
        """Upload a file to the sandbox via multipart form.

        Args:
            file: Path to file (str/Path) or file-like object (BinaryIO)
            dest_path: Base directory for upload (default: /workspace)
            filename: Override filename (required when file is BinaryIO)

        Returns:
            List of uploaded paths

        Raises:
            httpx.HTTPError: If the request fails
            ValueError: If filename is required but not provided (BinaryIO without name)

        Example:
            >>> paths = await session.upload("script.py")
            >>> paths = await session.upload("data.csv", dest_path="/workspace/data")
        """
        to_close: BinaryIO | None = None
        try:
            if hasattr(file, "read"):
                # BinaryIO
                f = file
                name = filename or getattr(f, "name", None)
                if not name:
                    raise ValueError("filename required when uploading file-like object")
                files = {"file": (Path(name).name, f)}
            else:
                path = Path(file)
                if not path.is_file():
                    raise FileNotFoundError(f"File not found: {path}")
                to_close = path.open("rb")
                files = {"file": (path.name, to_close)}

            resp = await self._client._http.post(
                f"/v1/sessions/{self._id}/fs/upload",
                data={"path": dest_path},
                files=files,
            )
            resp.raise_for_status()
            data = resp.json()
            return data.get("paths", [])
        finally:
            if to_close is not None:
                to_close.close()

    async def stats(self) -> SessionStats:
        """Get resource usage statistics for this session.

        Returns:
            SessionStats with memory_bytes, memory_limit, cpu_usage_usec

        Raises:
            httpx.HTTPError: If the request fails

        Example:
            >>> stats = await session.stats()
            >>> print(f"Memory: {stats.memory_bytes / 1024 / 1024:.1f} MB")
        """
        resp = await self._client._http.get(f"/v1/sessions/{self._id}/stats")
        resp.raise_for_status()
        data = resp.json()
        return SessionStats(
            memory_bytes=data.get("memory_bytes", 0),
            memory_limit=data.get("memory_limit", 0),
            cpu_usage_usec=data.get("cpu_usage_usec", 0),
        )

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
        return _parse_session_info(data)

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
