"""Sandkasten SDK exception hierarchy."""

import httpx


class SandkastenError(Exception):
    """Base exception for all Sandkasten SDK errors."""


class SandkastenAPIError(SandkastenError):
    """Raised when the API returns an error response."""

    def __init__(self, message: str, status_code: int | None = None, response_body: str | None = None):
        super().__init__(message)
        self.status_code = status_code
        self.response_body = response_body


class SandkastenStreamError(SandkastenError):
    """Raised when streaming (e.g. exec_stream) encounters an error."""
