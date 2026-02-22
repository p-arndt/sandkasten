"""Tests for Session."""

import base64
import tempfile
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from sandkasten import SandboxClient
from sandkasten.exceptions import SandkastenStreamError
from sandkasten.types import ExecChunk, ReadResult, SessionStats


@pytest.fixture
def session():
    """Create a session with mocked client."""
    client = MagicMock(spec=SandboxClient)
    client._http = AsyncMock()
    from sandkasten.session import Session

    return Session(client, "sess-123")


@pytest.mark.asyncio
async def test_exec(session):
    """exec returns ExecResult with correct fields."""
    session._client._http.post = AsyncMock(
        return_value=MagicMock(
            json=lambda: {
                "exit_code": 0,
                "cwd": "/workspace",
                "output": "hello",
                "truncated": False,
                "duration_ms": 100,
            },
            raise_for_status=MagicMock(),
        )
    )

    result = await session.exec("echo hello")

    assert result.exit_code == 0
    assert result.output == "hello"
    assert result.cwd == "/workspace"
    assert result.truncated is False


@pytest.mark.asyncio
async def test_exec_stream_event_type_init(session):
    """exec_stream does not raise NameError when event comes before data."""
    async def mock_aiter_lines():
        yield "event: chunk"
        yield 'data: {"chunk": "hi", "timestamp": 1000}'
        yield ""
        yield "event: done"
        yield 'data: {"exit_code": 0, "cwd": "/workspace", "duration_ms": 50}'

    mock_resp = MagicMock()
    mock_resp.raise_for_status = MagicMock()
    mock_resp.aiter_lines = mock_aiter_lines
    mock_resp.__aenter__ = AsyncMock(return_value=mock_resp)
    mock_resp.__aexit__ = AsyncMock(return_value=None)

    session._client._http.stream = MagicMock(return_value=mock_resp)

    chunks = []
    async for chunk in session.exec_stream("echo hi"):
        chunks.append(chunk)

    assert len(chunks) == 2
    assert chunks[0].output == "hi"
    assert chunks[0].done is False
    assert chunks[1].done is True
    assert chunks[1].exit_code == 0


@pytest.mark.asyncio
async def test_exec_stream_error_event(session):
    """exec_stream raises SandkastenStreamError on error event."""
    async def mock_aiter_lines():
        yield "event: error"
        yield 'data: {"error": "Command failed"}'

    mock_resp = MagicMock()
    mock_resp.raise_for_status = MagicMock()
    mock_resp.aiter_lines = mock_aiter_lines
    mock_resp.__aenter__ = AsyncMock(return_value=mock_resp)
    mock_resp.__aexit__ = AsyncMock(return_value=None)

    session._client._http.stream = MagicMock(return_value=mock_resp)

    with pytest.raises(SandkastenStreamError) as exc_info:
        async for _ in session.exec_stream("fail"):
            pass

    assert "Command failed" in str(exc_info.value)


@pytest.mark.asyncio
async def test_read_returns_read_result(session):
    """read returns ReadResult with content, path, truncated."""
    content_b64 = base64.b64encode(b"file content").decode("ascii")
    session._client._http.get = AsyncMock(
        return_value=MagicMock(
            json=lambda: {
                "path": "/workspace/out.txt",
                "content_base64": content_b64,
                "truncated": True,
            },
            raise_for_status=MagicMock(),
        )
    )

    result = await session.read("/workspace/out.txt")

    assert isinstance(result, ReadResult)
    assert result.content == b"file content"
    assert result.path == "/workspace/out.txt"
    assert result.truncated is True


@pytest.mark.asyncio
async def test_info_handles_optional_fields(session):
    """info parses response without container_id or last_activity."""
    session._client._http.get = AsyncMock(
        return_value=MagicMock(
            json=lambda: {
                "id": "sess-123",
                "image": "python",
                "status": "running",
                "cwd": "/workspace",
                "created_at": "2025-01-01T00:00:00Z",
                "expires_at": "2025-01-02T00:00:00Z",
            },
            raise_for_status=MagicMock(),
        )
    )

    info = await session.info()

    assert info.id == "sess-123"
    assert info.container_id is None
    assert info.last_activity is None


@pytest.mark.asyncio
async def test_stats(session):
    """stats returns SessionStats."""
    session._client._http.get = AsyncMock(
        return_value=MagicMock(
            json=lambda: {
                "memory_bytes": 1024000,
                "memory_limit": 536870912,
                "cpu_usage_usec": 50000,
            },
            raise_for_status=MagicMock(),
        )
    )

    stats = await session.stats()

    assert isinstance(stats, SessionStats)
    assert stats.memory_bytes == 1024000
    assert stats.memory_limit == 536870912
    assert stats.cpu_usage_usec == 50000


@pytest.mark.asyncio
async def test_upload_file_path(session):
    """upload with file path returns uploaded paths."""
    mock_resp = MagicMock()
    mock_resp.json.return_value = {"ok": True, "paths": ["/workspace/script.py"]}
    mock_resp.raise_for_status = MagicMock()

    session._client._http.post = AsyncMock(return_value=mock_resp)

    with tempfile.NamedTemporaryFile(mode="wb", suffix=".py", delete=False) as f:
        f.write(b"print('hello')")
        path = f.name

    try:
        paths = await session.upload(path)

        assert paths == ["/workspace/script.py"]
        assert session._client._http.post.called
        call_kwargs = session._client._http.post.call_args[1]
        assert "files" in call_kwargs
        assert call_kwargs["data"]["path"] == "/workspace"
    finally:
        Path(path).unlink(missing_ok=True)
