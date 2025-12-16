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
FastMCP-based MCP Server implementation for AEnv tools.
Uses streamable_http protocol for better performance.
"""

import importlib.util
import sys
from pathlib import Path
from typing import Callable

from fastmcp import FastMCP
from starlette.requests import Request
from starlette.responses import JSONResponse

from aenv.core.function_registry import get_function_registry, register_health
from aenv.core.logging import getLogger
from aenv.core.mcp_health import default_health_handler
from aenv.core.tool import Tool, get_registry

logger = getLogger("aenv.mcp_server", "system")


class AEnvMCPServer:
    """FastMCP-based server for AEnv tools."""

    def __init__(self, name: str = "aenv-server", version: str = "0.1.0"):
        self.name = name
        self.version = version
        self.mcp = FastMCP(
            name=name, version=version, host="0.0.0.0", port=8081, log_level="DEBUG"
        )
        # Set up registries with this server
        function_registry = get_function_registry()
        function_registry.set_server(self.mcp)
        register_health(default_health_handler)

        # Register list_functions endpoint
        self._setup_list_functions_endpoint()

    def _setup_tools(self):
        """Setup tools from registry."""
        registry = get_registry()

        for tool_descriptor in registry.list_mcp_tools():
            # Get the original tool descriptor to check if it's hidden
            original_descriptor = registry.get_tool_descriptor(tool_descriptor.name)
            if original_descriptor:
                tool_func = registry.get_tool(original_descriptor.name)
                if tool_func:
                    self._register_tool_with_mcp(original_descriptor, tool_func)

    def _register_tool_with_mcp(self, tool_descriptor: Tool, tool_func: Callable):
        """Register a tool with FastMCP."""
        # Register original function directly
        self.mcp.tool()(tool_func)

    def _setup_list_functions_endpoint(self):
        """Setup the list_functions endpoint to expose registered functions."""

        async def list_functions_handler(request: Request) -> JSONResponse:
            """Handler for listing all registered functions."""
            try:
                function_registry = get_function_registry()
                functions = function_registry.list_functions()

                # Categorize functions
                categorized = {
                    "reward": [],
                    "health": [],
                    "functions": [],
                }

                for func_name, func_info in functions.items():
                    endpoint = func_info.get("endpoint", "")
                    func_data = {
                        "name": func_name,
                        "endpoint": endpoint,
                        "method": func_info.get("method", "POST"),
                        "description": func_info.get("description", ""),
                        "parameters": func_info.get("parameters", {}),
                        "is_async": func_info.get("is_async", False),
                    }

                    if endpoint == "/task/reward":
                        categorized["reward"].append(func_data)
                    elif endpoint == "/health":
                        categorized["health"].append(func_data)
                    else:
                        categorized["functions"].append(func_data)

                return JSONResponse(
                    {
                        "success": True,
                        "data": {
                            "total": len(functions),
                            "functions": categorized["functions"],
                            "reward": categorized["reward"],
                            "health": categorized["health"],
                        },
                    }
                )
            except Exception as e:
                logger.error(f"Error listing functions: {str(e)}", exc_info=True)
                return JSONResponse(
                    {"success": False, "error": str(e)}, status_code=500
                )

        self.mcp.custom_route("/functions/list", ["GET"])(list_functions_handler)
        logger.info("Registered list_functions endpoint at /functions/list")

    def load_tools_from_directory(self, directory: Path) -> int:
        """Load tools from Python files in directory."""
        if not directory.exists() or not directory.is_dir():
            raise ValueError(
                f"Directory {directory} does not exist or is not a directory"
            )

        initial_count = len(get_registry().list_tools())

        # First, try to find src/ subdirectory (common pattern)
        src_dir = directory / "src"
        if src_dir.exists() and src_dir.is_dir():
            logger.info(f"Found src/ subdirectory, loading tools from {src_dir}")
            directory = src_dir

        # Add directory to sys.path to allow imports from same directory
        directory_str = str(directory.resolve())
        if directory_str not in sys.path:
            sys.path.insert(0, directory_str)

        try:
            # Load Python files recursively from directory
            python_files = list(directory.rglob("*.py"))
            logger.info(f"Found {len(python_files)} Python files in {directory}")

            for file_path in python_files:
                if file_path.name.startswith("_"):
                    logger.debug(f"Skipping file starting with '_': {file_path.name}")
                    continue

                try:
                    # Use a unique module name to avoid conflicts
                    module_name = f"aenv_tool_{file_path.stem}_{id(file_path)}"
                    spec = importlib.util.spec_from_file_location(
                        module_name, file_path
                    )
                    if spec and spec.loader:
                        module = importlib.util.module_from_spec(spec)
                        # Set __file__ for proper imports
                        module.__file__ = str(file_path)
                        # Calculate proper package name from directory structure
                        # If file is in a subdirectory, set __package__ to allow relative imports
                        relative_to_dir = file_path.relative_to(directory)
                        if len(relative_to_dir.parts) > 1:
                            # File is in a subdirectory
                            package_parts = relative_to_dir.parts[:-1]
                            module.__package__ = ".".join(package_parts)
                        else:
                            # File is directly in the directory
                            module.__package__ = ""
                        # Get tool count before loading
                        tools_before = len(get_registry().list_tools())
                        spec.loader.exec_module(module)
                        # Get tool count after loading
                        tools_after = len(get_registry().list_tools())
                        tools_added = tools_after - tools_before
                        logger.info(
                            f"Loaded module {file_path.name} (tools: {tools_before} -> {tools_after}, +{tools_added})"
                        )
                    else:
                        logger.warning(f"Failed to create spec for {file_path}")

                except Exception as e:
                    logger.error(
                        f"Failed to load {file_path}: {e}",
                        exc_info=True,
                        extra={
                            "file_path": str(file_path),
                            "error_type": type(e).__name__,
                            "error_message": str(e),
                        },
                    )

            # Setup tools after loading all modules
            self._setup_tools()

        finally:
            # Remove directory from sys.path to avoid side effects
            if directory_str in sys.path:
                sys.path.remove(directory_str)

        final_count = len(get_registry().list_tools())
        loaded_count = final_count - initial_count
        logger.info(f"Loaded {loaded_count} tools from directory {directory}")
        return loaded_count

    def load_tools_from_module(self, module_path: str) -> int:
        """Load tools from a specific module."""
        try:
            initial_count = len(get_registry().list_tools())

            file_path = Path(module_path)
            if not file_path.exists():
                raise FileNotFoundError(f"Module file not found: {module_path}")

            # Add parent directory to sys.path to allow imports from same directory
            parent_dir = str(file_path.parent.resolve())
            if parent_dir not in sys.path:
                sys.path.insert(0, parent_dir)

            try:
                module_name = f"aenv_tool_{file_path.stem}_{id(file_path)}"
                spec = importlib.util.spec_from_file_location(module_name, file_path)
                if spec and spec.loader:
                    module = importlib.util.module_from_spec(spec)
                    # Set __file__ for proper imports
                    module.__file__ = str(file_path)
                    # File is loaded directly, set empty package
                    module.__package__ = ""
                    spec.loader.exec_module(module)

                    # Setup tools after loading module
                    self._setup_tools()

                    final_count = len(get_registry().list_tools())
                    return final_count - initial_count
            finally:
                # Remove parent directory from sys.path
                if parent_dir in sys.path:
                    sys.path.remove(parent_dir)

        except Exception as e:
            logger.error(f"Failed to load module {module_path}: {e}", exc_info=True)
            raise

        return 0

    def run(self, host: str, port: int):
        """Run the MCP server synchronously."""
        self.mcp.host = host
        self.mcp.port = port

        registry = get_registry()
        tools = registry.list_tools()

        function_registry = get_function_registry()
        functions = function_registry.list_functions()

        logger.info(
            f"Starting AEnv MCP server | "
            f"Host: {host} | "
            f"Port: {port} | "
            f"Name: {self.name} | "
            f"Version: {self.version} | "
            f"Tools: {len(tools)} | "
            f"Functions: {len(functions)}"
        )

        if functions:
            logger.info(f"Registered functions: {', '.join(functions.keys())}")

        # Use FastMCP's built-in synchronous run method
        self.mcp.run(transport="streamable-http", host=host, port=port)


def create_server(name: str = "aenv-server", version: str = "0.1.0") -> AEnvMCPServer:
    """Create a new MCP server instance."""
    return AEnvMCPServer(name=name, version=version)
