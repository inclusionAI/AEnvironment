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
Tests for AEnv tool functionality.
"""

import asyncio
from typing import Any, Dict

import pytest

from aenv.core.tool import Tool, get_registry, register_tool


class TestTool:
    """Test Tool class."""

    def test_tool_creation(self):
        """Test basic tool creation."""
        tool = Tool(
            name="test_tool",
            description="A test tool",
            inputSchema={
                "type": "object",
                "properties": {
                    "param1": {"type": "string"},
                    "param2": {"type": "integer", "default": 42},
                },
                "required": ["param1"],
            },
        )

        assert tool.name == "test_tool"
        assert tool.description == "A test tool"
        assert "param1" in tool.inputSchema["properties"]

    def test_openai_format(self):
        """Test OpenAI tool format conversion."""
        tool = Tool(
            name="test_tool",
            description="A test tool",
            inputSchema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        )

        openai_tool = tool.get_openai_tool()
        assert openai_tool["type"] == "function"
        assert openai_tool["function"]["name"] == "test_tool"
        assert "query" in openai_tool["function"]["parameters"]["properties"]

    def test_mcp_format(self):
        """Test MCP tool format conversion."""
        tool = Tool(
            name="test_tool",
            description="A test tool",
            inputSchema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        )

        mcp_tool = tool.get_mcp_tool()
        assert mcp_tool["name"] == "test_tool"
        assert mcp_tool["description"] == "A test tool"


class TestToolRegistry:
    """Test tool registry functionality."""

    def test_register_tool(self):
        """Test tool registration."""
        registry = get_registry()

        @register_tool
        def simple_tool(query: str) -> str:
            """A simple test tool."""
            return f"Result for {query}"

        assert "simple_tool" in [t.name for t in registry.list_tools()]

        tool_func = registry.get_tool("simple_tool")
        assert tool_func is not None
        assert tool_func("test") == "Result for test"

    def test_register_tool_with_custom_name(self):
        """Test tool registration with custom name."""
        registry = get_registry()

        @register_tool(name="custom_name")
        def original_function(query: str) -> str:
            """A tool with custom name."""
            return f"Custom: {query}"

        assert "custom_name" in [t.name for t in registry.list_tools()]
        assert registry.get_tool("custom_name") is not None

    def test_tool_schema_generation(self):
        """Test automatic schema generation."""
        registry = get_registry()

        @register_tool
        def complex_tool(
            query: str, limit: int = 10, include_metadata: bool = False
        ) -> Dict[str, Any]:
            """A tool with multiple parameter types."""
            return {"query": query, "limit": limit, "metadata": include_metadata}

        tool = registry.get_tool_descriptor("complex_tool")
        assert tool is not None

        schema = tool.inputSchema
        assert "query" in schema["properties"]
        assert schema["properties"]["query"]["type"] == "string"
        assert "limit" in schema["properties"]
        assert schema["properties"]["limit"]["type"] == "integer"
        assert "include_metadata" in schema["properties"]
        assert schema["properties"]["include_metadata"]["type"] == "boolean"

        assert "query" in schema["required"]
        assert "limit" not in schema["required"]

    def test_duplicate_tool_registration(self):
        """Test duplicate tool registration raises error."""

        @register_tool
        def duplicate_tool():
            return "first"

        with pytest.raises(ValueError):

            @register_tool
            def duplicate_tool():  # noqa: F811
                return "second"


class TestAsyncTools:
    """Test async tool functionality."""

    @pytest.mark.asyncio
    async def test_async_tool(self):
        """Test async tool registration and execution."""
        registry = get_registry()

        @register_tool
        async def async_tool(query: str) -> str:
            """An async test tool."""
            await asyncio.sleep(0.01)  # Small delay
            return f"Async result for {query}"

        tool_func = registry.get_tool("async_tool")
        assert tool_func is not None

        # Note: In real usage, this would be handled by the MCP server
        # This is just testing registration
        assert asyncio.iscoroutinefunction(tool_func)
