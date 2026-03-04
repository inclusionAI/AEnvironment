# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Tests for AEnv environment functionality.
"""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from aenv.core.environment import Environment, ToolResult
from aenv.core.exceptions import EnvironmentError, ToolError


class TestEnvironment:
    """Test Environment class."""

    @pytest.fixture
    def mock_client(self):
        """Mock HTTP client."""
        with patch("aenv.core.environment.httpx.AsyncClient") as mock:
            yield mock

    @pytest.mark.asyncio
    async def test_environment_initialization(self, mock_client):
        """Test environment initialization."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.return_value.status_code = 201
        mock_instance.post.return_value.json.return_value = {
            "instance_id": "test-123",
            "name": "test-env",
            "status": "created",
        }

        env = Environment("test-env", scheduler_url="http://test.com")

        with patch("aenv.core.environment.get_registry") as mock_registry:
            mock_registry.return_value.list_tools.return_value = []
            result = await env.initialize()

        assert result is True
        assert env._initialized is True
        assert env._instance_id == "test-123"

    @pytest.mark.asyncio
    async def test_environment_initialization_failure(self, mock_client):
        """Test environment initialization failure."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.return_value.status_code = 500
        mock_instance.post.return_value.text = "Server error"

        env = Environment("test-env", scheduler_url="http://test.com")

        with patch("aenv.core.environment.get_registry") as mock_registry:
            mock_registry.return_value.list_tools.return_value = []
            with pytest.raises(EnvironmentError):
                await env.initialize()

    @pytest.mark.asyncio
    async def test_list_tools(self, mock_client):
        """Test listing tools."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.return_value.status_code = 201
        mock_instance.post.return_value.json.return_value = {"instance_id": "test-123"}

        env = Environment("test-env", scheduler_url="http://test.com")

        with patch("aenv.core.environment.get_registry") as mock_registry:
            from aenv.core.tool import Tool

            mock_tool = Tool(
                name="test_tool",
                description="A test tool",
                inputSchema={"type": "object", "properties": {}},
            )
            mock_registry.return_value.list_tools.return_value = [mock_tool]

            await env.initialize()
            tools = env.list_tools()

        assert len(tools) == 1
        assert tools[0]["name"] == "test-env/test_tool"

    @pytest.mark.asyncio
    async def test_call_tool(self, mock_client):
        """Test tool execution."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.side_effect = [
            # Environment creation
            AsyncMock(status_code=201, json=lambda: {"instance_id": "test-123"}),
            # Tool call
            AsyncMock(
                status_code=200,
                json=lambda: {
                    "content": [{"type": "text", "text": "Success"}],
                    "isError": False,
                },
            ),
        ]

        env = Environment("test-env", scheduler_url="http://test.com")

        with patch("aenv.core.environment.get_registry") as mock_registry:
            from aenv.core.tool import Tool

            mock_tool = Tool(
                name="test_tool",
                description="A test tool",
                inputSchema={
                    "type": "object",
                    "properties": {"query": {"type": "string"}},
                },
            )
            mock_registry.return_value.list_tools.return_value = [mock_tool]

            await env.initialize()
            result = await env.call_tool("test_tool", {"query": "test"})

        assert isinstance(result, ToolResult)
        assert result.isError is False
        assert len(result.content) == 1

    @pytest.mark.asyncio
    async def test_call_tool_not_found(self, mock_client):
        """Test calling non-existent tool."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.return_value.status_code = 201
        mock_instance.post.return_value.json.return_value = {"instance_id": "test-123"}

        env = Environment("test-env", scheduler_url="http://test.com")

        with patch("aenv.core.environment.get_registry") as mock_registry:
            mock_registry.return_value.list_tools.return_value = []
            await env.initialize()

            with pytest.raises(ToolError):
                await env.call_tool("nonexistent", {})

    @pytest.mark.asyncio
    async def test_context_manager(self, mock_client):
        """Test async context manager."""
        mock_instance = AsyncMock()
        mock_client.return_value.__aenter__.return_value = mock_instance
        mock_instance.post.return_value.status_code = 201
        mock_instance.post.return_value.json.return_value = {"instance_id": "test-123"}
        mock_instance.delete.return_value.status_code = 204

        with patch("aenv.core.environment.get_registry") as mock_registry:
            mock_registry.return_value.list_tools.return_value = []

            async with Environment("test-env", scheduler_url="http://test.com") as env:
                assert env._initialized is True
                assert env._instance_id == "test-123"

    def test_env_convenience_function(self):
        """Test env() convenience function."""
        from aenv.core.environment import Environment

        environment = Environment("test-env", scheduler_url="http://test.com")
        assert isinstance(environment, Environment)
        assert environment.env_name == "test-env"
        assert environment.scheduler_url == "http://test.com"


class TestMCPSessionReuse:
    """Tests for MCP session reuse behavior.

    Verifies that _ensure_mcp_session() lazily creates a single MCP session
    and reuses it across multiple call_tool()/list_tools() invocations,
    only tearing it down on release() or when the connection is lost.
    """

    def _make_env(self):
        """Create an Environment instance pre-configured for unit testing.

        Returns an Environment with _initialized=True and a fake _instance
        so that call_tool / list_tools skip the real initialization path.
        """
        env = Environment("test-env")
        env._initialized = True

        # Minimal fake instance so _get_mcp_client does not complain
        class _FakeInstance:
            ip = "127.0.0.1"
            id = "fake-id"

        env._instance = _FakeInstance()
        env.proxy_headers = {"AEnvCore-MCPProxy-URL": "http://127.0.0.1:8081"}
        return env

    def _make_mock_client(self):
        """Build a mock fastmcp Client with sensible defaults.

        Uses MagicMock as the base so that synchronous methods like
        is_connected() return plain values (not coroutines).  Async
        methods (__aenter__, call_tool_mcp, etc.) are explicitly set
        to AsyncMock instances.
        """
        mock_client = MagicMock()

        # is_connected() is a regular (sync) method on the real Client
        mock_client.is_connected.return_value = True

        # __aenter__ / __aexit__ are async on the real Client
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=None)

        # call_tool_mcp returns a result with .content=[] and .isError=False
        call_result = MagicMock()
        call_result.content = []
        call_result.isError = False
        mock_client.call_tool_mcp = AsyncMock(return_value=call_result)

        # list_tools returns an empty list
        mock_client.list_tools = AsyncMock(return_value=[])

        # close() is async on the real Client
        mock_client.close = AsyncMock()

        return mock_client

    def _patch_get_mcp_client(self, env, mock_client):
        """Create a side-effect function for _get_mcp_client that also
        sets env._mcp_client, mirroring the real implementation."""

        async def _side_effect():
            env._mcp_client = mock_client
            return mock_client

        return patch.object(env, "_get_mcp_client", side_effect=_side_effect)

    @pytest.mark.asyncio
    async def test_session_created_once_across_multiple_call_tool(self):
        """Calling call_tool 3 times should only establish the MCP session once."""
        env = self._make_env()
        mock_client = self._make_mock_client()

        with self._patch_get_mcp_client(env, mock_client):
            await env.call_tool("tool_a", {"x": 1})
            await env.call_tool("tool_b", {"x": 2})
            await env.call_tool("tool_c", {"x": 3})

        # Session established exactly once
        assert mock_client.__aenter__.await_count == 1
        # Each call_tool invoked call_tool_mcp
        assert mock_client.call_tool_mcp.await_count == 3
        # Session never torn down (that only happens in release)
        assert mock_client.__aexit__.await_count == 0

    @pytest.mark.asyncio
    async def test_session_shared_between_list_and_call(self):
        """list_tools and call_tool should share the same MCP session."""
        env = self._make_env()
        mock_client = self._make_mock_client()

        with self._patch_get_mcp_client(env, mock_client):
            await env.list_tools()
            await env.call_tool("tool_a", {"x": 1})

        # Only one session established for both operations
        assert mock_client.__aenter__.await_count == 1

    @pytest.mark.asyncio
    async def test_release_closes_session(self):
        """After call_tool, release() should close the client and reset state."""
        env = self._make_env()
        mock_client = self._make_mock_client()

        with self._patch_get_mcp_client(env, mock_client):
            await env.call_tool("tool_a", {"x": 1})

        # Sanity: session is active before release
        assert env._mcp_session_active is True
        assert env._mcp_client is not None

        await env.release()

        # close() was called on the client
        mock_client.close.assert_awaited_once()
        # State fully reset
        assert env._mcp_session_active is False
        assert env._mcp_client is None

    @pytest.mark.asyncio
    async def test_session_reconnect_on_disconnect(self):
        """If the connection drops, the next call should re-establish the session."""
        env = self._make_env()
        mock_client = self._make_mock_client()

        # We need _get_mcp_client to return a fresh mock on reconnect
        # because _ensure_mcp_session sets self._mcp_client = None before
        # calling _get_mcp_client again.
        mock_client_2 = self._make_mock_client()
        get_client_calls = [mock_client, mock_client_2]

        async def _get_mcp_client_side_effect():
            client = get_client_calls.pop(0)
            env._mcp_client = client
            return client

        with patch.object(
            env, "_get_mcp_client", side_effect=_get_mcp_client_side_effect
        ):
            # First call: session created normally
            await env.call_tool("tool_a", {"x": 1})
            assert mock_client.__aenter__.await_count == 1

            # Simulate connection loss
            mock_client.is_connected.return_value = False

            # Second call: should detect disconnect and reconnect
            await env.call_tool("tool_b", {"x": 2})

        # First client entered once, second client entered once = 2 total sessions
        assert mock_client.__aenter__.await_count == 1
        assert mock_client_2.__aenter__.await_count == 1
        # First client was closed during stale-client cleanup
        mock_client.close.assert_awaited_once()
        # Second call went through the new client
        mock_client_2.call_tool_mcp.assert_awaited_once()
