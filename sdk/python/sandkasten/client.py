"""Sandkasten client for managing sandbox sessions."""

import httpx

from .session import Session
from .types import SessionInfo


class SandboxClient:
    """Client for interacting with the Sandkasten daemon.

    This client manages sandbox sessions - isolated container environments
    where AI agents can execute code safely.

    Example:
        >>> client = SandboxClient(
        ...     base_url="http://localhost:8080",
        ...     api_key="sk-sandbox-quickstart"
        ... )
        >>> session = await client.create_session(image="sandbox-runtime:python")
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
        image: str = "sandbox-runtime:python",
        ttl_seconds: int | None = None,
    ) -> Session:
        """Create a new sandbox session.

        Args:
            image: Docker image to use (default: sandbox-runtime:python)
                   Available: sandbox-runtime:base, :python, :node
            ttl_seconds: Session TTL in seconds (None = daemon default)

        Returns:
            Session object for interacting with the sandbox

        Raises:
            httpx.HTTPError: If session creation fails

        Example:
            >>> session = await client.create_session(
            ...     image="sandbox-runtime:python",
            ...     ttl_seconds=3600
            ... )
        """
        payload = {"image": image}
        if ttl_seconds is not None:
            payload["ttl_seconds"] = ttl_seconds

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

        return [
            SessionInfo(
                id=s["id"],
                image=s["image"],
                container_id=s["container_id"],
                status=s["status"],
                cwd=s["cwd"],
                created_at=s["created_at"],
                expires_at=s["expires_at"],
                last_activity=s["last_activity"],
            )
            for s in data.get("sessions", [])
        ]

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
