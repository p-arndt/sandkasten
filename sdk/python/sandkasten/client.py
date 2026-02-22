"""Sandkasten client for managing sandbox sessions."""

import httpx

from .session import Session
from .types import SessionInfo, WorkspaceInfo, _parse_session_info


class SandboxClient:
    """Client for interacting with the Sandkasten daemon.

    This client manages sandbox sessions - isolated container environments
    where AI agents can execute code safely.

    Example:
        >>> client = SandboxClient(
        ...     base_url="http://localhost:8080",
        ...     api_key="sk-sandbox-quickstart"
        ... )
        >>> session = await client.create_session(image="python")
        >>> try:
        ...     result = await session.exec("pip install requests")
        ...     print(result.output)
        ... finally:
        ...     await session.destroy()
    """

    def __init__(
        self,
        *,
        base_url: str = "http://localhost:8080",
        api_key: str,
        timeout: float = 120.0,
    ):
        """Initialize the Sandkasten client.

        Args:
            base_url: URL of the Sandkasten daemon (default: http://localhost:8080)
            api_key: API key for authentication
            timeout: HTTP request timeout in seconds (default: 120.0)
        """
        self._http = httpx.AsyncClient(
            base_url=base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=timeout,
        )

    async def create_session(
        self,
        *,
        image: str = "python",
        ttl_seconds: int | None = None,
        workspace_id: str | None = None,
    ) -> Session:
        """Create a new sandbox session.

        Args:
            image: Docker image to use (default: python)
                   Available: base, python, node
            ttl_seconds: Session TTL in seconds (None = daemon default)
            workspace_id: Optional persistent workspace ID (None = ephemeral)

        Returns:
            Session object for interacting with the sandbox

        Raises:
            httpx.HTTPError: If session creation fails

        Example:
            >>> # Ephemeral session
            >>> session = await client.create_session(image="python")

            >>> # Persistent workspace
            >>> session = await client.create_session(
            ...     image="python",
            ...     workspace_id="user123-project456"
            ... )
        """
        payload = {"image": image}
        if ttl_seconds is not None:
            payload["ttl_seconds"] = ttl_seconds
        if workspace_id is not None:
            payload["workspace_id"] = workspace_id

        resp = await self._http.post("/v1/sessions", json=payload)
        resp.raise_for_status()
        data = resp.json()

        return Session(self, data["id"])

    async def get_session(self, session_id: str) -> Session:
        """Get an existing session by ID.

        Args:
            session_id: Session identifier

        Returns:
            Session object

        Raises:
            httpx.HTTPError: If session doesn't exist

        Example:
            >>> session = await client.get_session("sess_abc123")
        """
        # Verify session exists
        resp = await self._http.get(f"/v1/sessions/{session_id}")
        resp.raise_for_status()

        return Session(self, session_id)

    async def list_sessions(self) -> list[SessionInfo]:
        """List all active sessions.

        Returns:
            List of SessionInfo objects

        Example:
            >>> sessions = await client.list_sessions()
            >>> for s in sessions:
            ...     print(f"{s.id}: {s.status}")
        """
        resp = await self._http.get("/v1/sessions")
        resp.raise_for_status()
        data = resp.json()

        # API returns raw array, not {"sessions": [...]}
        sessions = data if isinstance(data, list) else data.get("sessions", [])
        return [_parse_session_info(s) for s in sessions]

    async def list_workspaces(self) -> list[WorkspaceInfo]:
        """List all persistent workspaces.

        Returns:
            List of WorkspaceInfo objects

        Example:
            >>> workspaces = await client.list_workspaces()
            >>> for ws in workspaces:
            ...     print(ws.id)
        """
        resp = await self._http.get("/v1/workspaces")
        resp.raise_for_status()
        data = resp.json()
        raw = data.get("workspaces", [])
        return [WorkspaceInfo(id=w["id"]) for w in raw]

    async def delete_workspace(self, workspace_id: str) -> None:
        """Delete a persistent workspace.

        Args:
            workspace_id: Workspace identifier

        Raises:
            httpx.HTTPError: If deletion fails

        Example:
            >>> await client.delete_workspace("user123-project456")
        """
        resp = await self._http.delete(f"/v1/workspaces/{workspace_id}")
        resp.raise_for_status()

    async def close(self) -> None:
        """Close the HTTP client and release resources.

        Example:
            >>> await client.close()
        """
        await self._http.aclose()

    async def __aenter__(self) -> "SandboxClient":
        """Context manager entry."""
        return self

    async def __aexit__(self, *args) -> None:
        """Context manager exit - automatically closes client."""
        await self.close()
