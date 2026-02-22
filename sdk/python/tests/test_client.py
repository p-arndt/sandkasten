"""Tests for SandboxClient."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from sandkasten import SandboxClient
from sandkasten.types import SessionInfo, WorkspaceInfo


@pytest.mark.asyncio
async def test_create_session(base_url, api_key):
    """create_session returns Session with ID from response."""
    mock_resp = MagicMock()
    mock_resp.json.return_value = {"id": "abc12345-678", "image": "python", "status": "running"}
    mock_resp.raise_for_status = MagicMock()

    client = SandboxClient(base_url=base_url, api_key=api_key)
    client._http = AsyncMock()
    client._http.post = AsyncMock(return_value=mock_resp)

    session = await client.create_session(image="python")

    assert session.id == "abc12345-678"
    client._http.post.assert_called_once_with("/v1/sessions", json={"image": "python"})


@pytest.mark.asyncio
async def test_list_sessions_raw_array(base_url, api_key):
    """list_sessions handles raw JSON array response (not {"sessions": [...]})."""
    mock_resp = MagicMock()
    # Backend returns raw array
    mock_resp.json.return_value = [
        {
            "id": "s1",
            "image": "python",
            "status": "running",
            "cwd": "/workspace",
            "created_at": "2025-01-01T00:00:00Z",
            "expires_at": "2025-01-02T00:00:00Z",
        },
    ]
    mock_resp.raise_for_status = MagicMock()

    client = SandboxClient(base_url=base_url, api_key=api_key)
    client._http = AsyncMock()
    client._http.get = AsyncMock(return_value=mock_resp)

    sessions = await client.list_sessions()

    assert len(sessions) == 1
    assert isinstance(sessions[0], SessionInfo)
    assert sessions[0].id == "s1"
    assert sessions[0].image == "python"
    assert sessions[0].status == "running"
    assert sessions[0].container_id is None
    assert sessions[0].last_activity is None


@pytest.mark.asyncio
async def test_list_sessions_wrapped_format(base_url, api_key):
    """list_sessions handles wrapped {"sessions": [...]} format for compatibility."""
    mock_resp = MagicMock()
    mock_resp.json.return_value = {
        "sessions": [
            {
                "id": "s2",
                "image": "base",
                "status": "running",
                "cwd": "/workspace",
                "created_at": "2025-01-01T00:00:00Z",
                "expires_at": "2025-01-02T00:00:00Z",
            },
        ]
    }
    mock_resp.raise_for_status = MagicMock()

    client = SandboxClient(base_url=base_url, api_key=api_key)
    client._http = AsyncMock()
    client._http.get = AsyncMock(return_value=mock_resp)

    sessions = await client.list_sessions()

    assert len(sessions) == 1
    assert sessions[0].id == "s2"


@pytest.mark.asyncio
async def test_list_workspaces(base_url, api_key):
    """list_workspaces returns WorkspaceInfo list."""
    mock_resp = MagicMock()
    mock_resp.json.return_value = {"workspaces": [{"id": "ws-1"}, {"id": "ws-2"}]}
    mock_resp.raise_for_status = MagicMock()

    client = SandboxClient(base_url=base_url, api_key=api_key)
    client._http = AsyncMock()
    client._http.get = AsyncMock(return_value=mock_resp)

    workspaces = await client.list_workspaces()

    assert len(workspaces) == 2
    assert all(isinstance(w, WorkspaceInfo) for w in workspaces)
    assert workspaces[0].id == "ws-1"
    assert workspaces[1].id == "ws-2"
