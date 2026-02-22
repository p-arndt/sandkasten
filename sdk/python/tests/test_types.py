"""Tests for type parsing."""

import pytest

from sandkasten.types import SessionInfo, _parse_session_info


def test_parse_session_info_minimal():
    """_parse_session_info handles minimal backend response."""
    data = {
        "id": "s1",
        "image": "python",
        "status": "running",
        "cwd": "/workspace",
        "created_at": "2025-01-01T00:00:00Z",
        "expires_at": "2025-01-02T00:00:00Z",
    }
    info = _parse_session_info(data)
    assert info.id == "s1"
    assert info.workspace_id is None
    assert info.container_id is None
    assert info.last_activity is None


def test_parse_session_info_full():
    """_parse_session_info handles all optional fields."""
    data = {
        "id": "s2",
        "image": "base",
        "status": "running",
        "cwd": "/tmp",
        "created_at": "2025-01-01T00:00:00Z",
        "expires_at": "2025-01-02T00:00:00Z",
        "workspace_id": "ws-abc",
        "container_id": "cont-123",
        "last_activity": "2025-01-01T12:00:00Z",
    }
    info = _parse_session_info(data)
    assert info.workspace_id == "ws-abc"
    assert info.container_id == "cont-123"
    assert info.last_activity == "2025-01-01T12:00:00Z"
