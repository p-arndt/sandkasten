"""Pytest configuration and fixtures."""

import pytest


@pytest.fixture
def base_url():
    """Base URL for Sandkasten API."""
    return "http://localhost:8080"


@pytest.fixture
def api_key():
    """Test API key."""
    return "sk-test-key"
