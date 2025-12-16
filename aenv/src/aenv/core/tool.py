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
Tool definitions based on official MCP protocol.
Updated to use mcp library directly.
"""

import inspect
from typing import Any, Callable, Dict, List, Optional, get_type_hints

from mcp import types as mcp_types
from pydantic import BaseModel, ConfigDict, Field


class Tool(BaseModel):
    """
    Definition for a tool the client can call.
    Based on MCP protocol specification using official mcp library.
    """

    name: str = Field(..., description="Tool name, must be unique within environment")
    description: Optional[str] = Field(None, description="Human-readable description")
    inputSchema: Dict[str, Any] = Field(
        ..., description="JSON Schema for input validation"
    )
    outputSchema: Optional[Dict[str, Any]] = Field(
        None, description="JSON Schema for output structure"
    )
    hidden: bool = Field(False, description="Whether to hide this tool from list_tools")

    model_config = ConfigDict(extra="allow")

    def get_mcp_tool(self) -> mcp_types.Tool:
        """Convert to official MCP Tool format."""
        return mcp_types.Tool(
            name=self.name,
            description=self.description or "",
            inputSchema=self.inputSchema,
        )

    def get_openai_tool(self) -> Dict[str, Any]:
        """Convert to OpenAI tool format."""
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description or "",
                "parameters": self.inputSchema,
            },
        }


class ToolRegistry:
    """Global tool registry for managing registered tools."""

    def __init__(self):
        self._tools: Dict[str, Callable] = {}
        self._tool_descriptors: Dict[str, Tool] = {}

    def register(
        self,
        func: Callable,
        name: Optional[str] = None,
        description: Optional[str] = None,
        hidden: bool = False,
    ) -> Callable:
        """Register a function as a tool using MCP format."""

        tool_name = name or func.__name__
        if tool_name in self._tools:
            raise ValueError(f"Tool '{tool_name}' already registered")

        # Generate JSON schema from type hints and docstring
        input_schema = self._generate_input_schema(func)
        output_schema = self._generate_output_schema(func)

        tool = Tool(
            name=tool_name,
            description=description or func.__doc__,
            inputSchema=input_schema,
            outputSchema=output_schema,
            hidden=hidden,
        )

        self._tools[tool_name] = func
        self._tool_descriptors[tool_name] = tool

        return func

    def get_tool(self, name: str) -> Optional[Callable]:
        """Get registered tool by name."""
        return self._tools.get(name)

    def get_tool_descriptor(self, name: str) -> Optional[Tool]:
        """Get tool descriptor by name."""
        return self._tool_descriptors.get(name)

    def list_tools(self) -> List[Tool]:
        """List all registered tool descriptors."""
        return list(self._tool_descriptors.values())

    def list_mcp_tools(self) -> List[mcp_types.Tool]:
        """List all tools in official MCP format."""
        return [
            tool.get_mcp_tool()
            for tool in self._tool_descriptors.values()
            if not tool.hidden
        ]

    def _generate_input_schema(self, func: Callable) -> Dict[str, Any]:
        """Generate JSON schema from function signature."""
        sig = inspect.signature(func)
        type_hints = get_type_hints(func)

        properties = {}
        required = []

        for param_name, param in sig.parameters.items():
            if param_name == "self":
                continue

            param_type = type_hints.get(param_name, str)

            # Convert Python types to JSON schema types
            type_mapping = {
                str: "string",
                int: "integer",
                float: "number",
                bool: "boolean",
                list: "array",
                dict: "object",
                List: "array",
                Dict: "object",
            }

            json_type = type_mapping.get(param_type, "string")

            properties[param_name] = {
                "type": json_type,
                "description": f"Parameter {param_name}",
            }

            if param.default == inspect.Parameter.empty:
                required.append(param_name)
            else:
                properties[param_name]["default"] = param.default

        return {
            "type": "object",
            "properties": properties,
            "required": required,
        }

    def _generate_output_schema(self, func: Callable) -> Optional[Dict[str, Any]]:
        """Generate JSON schema for function return type."""
        type_hints = get_type_hints(func)
        return_type = type_hints.get("return")

        if return_type is None or return_type is type(None):
            return None

        # Basic type mapping for return types
        type_mapping = {
            str: {"type": "string"},
            int: {"type": "integer"},
            float: {"type": "number"},
            bool: {"type": "boolean"},
            list: {"type": "array"},
            dict: {"type": "object"},
            List: {"type": "array"},
            Dict: {"type": "object"},
        }

        return type_mapping.get(return_type, {"type": "object"})


# Global registry instance
_registry = ToolRegistry()


def register_tool(
    func: Optional[Callable] = None,
    *,
    name: Optional[str] = None,
    description: Optional[str] = None,
):
    """
    Decorator to register a function as a tool using MCP format.

    Args:
        func: Function to register
        name: Optional custom tool name
        description: Optional custom description
    """

    def decorator(f: Callable) -> Callable:
        return _registry.register(f, name=name, description=description)

    if func is None:
        return decorator
    else:
        return decorator(func)


def get_registry() -> ToolRegistry:
    """Get the global tool registry."""
    return _registry
