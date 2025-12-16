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

from unittest.mock import AsyncMock, patch

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
