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
Generic function registry for registering functions as HTTP endpoints.
This is a generalized version of RewardRegistry to support any function registration.
"""

import inspect
import json
import traceback
from typing import Any, Callable, Dict, Optional

from fastmcp import FastMCP
from starlette.requests import Request
from starlette.responses import JSONResponse

from aenv.core.logging import getLogger

logger = getLogger(__name__, "colored")


class BaseRegistry:
    """Base class for all registry implementations."""

    def __init__(self, registry_name: str = "registry"):
        self._server: Optional[FastMCP] = None
        self._registry_name = registry_name

    def set_server(self, server: FastMCP):
        """Set the FastMCP server instance."""
        self._server = server
        logger.debug(f"Server set for {self._registry_name}")

    def get_server(self) -> Optional[FastMCP]:
        """Get the FastMCP server instance."""
        return self._server

    @property
    def has_server(self) -> bool:
        """Check if server is set."""
        return self._server is not None


class FunctionRegistry(BaseRegistry):
    """Generic registry for registering functions as HTTP endpoints."""

    def __init__(self):
        super().__init__("function_registry")
        self._functions: Dict[str, Callable] = {}
        self._function_metadata: Dict[str, Dict[str, Any]] = {}

    def register_function(
        self, func: Callable, endpoint: str = None, method: str = "POST"
    ) -> Callable:
        """
        Register a function to a specific HTTP endpoint.

        Args:
            func: Function to register
            endpoint: HTTP endpoint path (e.g., "/task/reward", "/functions/my_func")
                   If None, uses "/functions/{func.__name__}"

        Returns:
            The original function (for decorator usage)
        """
        if not self._server:
            raise RuntimeError("FastMCP server not set")

        if endpoint is None:
            endpoint = f"/functions/{func.__name__}"

        # Store the function and metadata
        func_name = func.__name__
        self._functions[func_name] = func

        # Store function metadata
        func_doc = inspect.getdoc(func) or ""
        sig = inspect.signature(func)
        params = {}
        for param_name, param in sig.parameters.items():
            params[param_name] = {
                "type": (
                    str(param.annotation)
                    if param.annotation != inspect.Parameter.empty
                    else "Any"
                ),
                "default": (
                    str(param.default)
                    if param.default != inspect.Parameter.empty
                    else None
                ),
            }

        self._function_metadata[func_name] = {
            "endpoint": endpoint,
            "method": method,
            "description": func_doc,
            "parameters": params,
            "is_async": inspect.iscoroutinefunction(func),
        }

        async def handler(request: Request) -> JSONResponse:
            """Generic handler for function requests."""
            try:
                if request.method in ["POST", "PUT", "PATCH"]:
                    body = await request.json()
                elif request.method == "GET":
                    body = dict(request.query_params)
                else:
                    return JSONResponse(
                        {"success": False, "error": "Method not allowed"},
                        status_code=405,
                    )

                # Update active time for health check
                from aenv.core.mcp_health import update_active_time

                update_active_time()

                # Check if the function is async
                if inspect.iscoroutinefunction(func):
                    result = await func(**body)
                else:
                    result = func(**body)

                # Ensure result is JSON serializable
                try:
                    json.dumps(result)
                except (TypeError, ValueError):
                    # Convert non-serializable objects to string
                    result = str(result)

                return JSONResponse({"success": True, "data": result})
            except Exception as e:
                traceback.print_exc()
                logger.error(f"Function handler error for {endpoint}: {str(e)}")
                return JSONResponse(
                    {"success": False, "error": str(e)}, status_code=500
                )

        self._server.custom_route(endpoint, [method])(handler)
        logger.info(
            f"Registered function '{func_name}' at endpoint '{endpoint}' with {method}"
        )
        return func

    def list_functions(self) -> Dict[str, Dict[str, Any]]:
        """List all registered functions with detailed information including endpoints, methods, and metadata."""
        result = {}
        for func_name, func in self._functions.items():
            metadata = self._function_metadata.get(func_name, {})
            result[func_name] = {
                "name": func_name,
                "endpoint": metadata.get("endpoint", f"/functions/{func_name}"),
                "method": metadata.get("method", "POST"),
                "description": metadata.get("description", ""),
                "parameters": metadata.get("parameters", {}),
                "is_async": metadata.get("is_async", False),
            }
        return result

    def list_functions_simple(self) -> Dict[str, str]:
        """List all registered functions with their endpoints (simple format for backward compatibility)."""
        return {
            name: self._function_metadata.get(name, {}).get(
                "endpoint", f"/functions/{name}"
            )
            for name in self._functions.keys()
        }


# Global function registry instance
_function_registry = FunctionRegistry()


def register_function(
    func: Optional[Callable] = None, *, endpoint: str = None, method: str = "POST"
):
    """
    Decorator to register a function as an HTTP endpoint.

    Usage:
        @register_function
        def my_func(...): ...

        @register_function(endpoint="/custom/path", method="GET")
        def my_func(...): ...

    Args:
        func: Function to register
        endpoint: Custom endpoint path (optional)
    """

    def decorator(f: Callable) -> Callable:
        return _function_registry.register_function(f, endpoint=endpoint, method=method)

    # Handle both @register_function and @register_function() usage patterns
    if func is not None and callable(func):
        return decorator(func)
    else:
        return decorator


def register_reward(
    func: Optional[Callable] = None,
):
    """
    Decorator to register a function as a reward endpoint.

    The function will be exposed as a POST endpoint at /task/reward
    instead of being registered as an MCP tool.

    Args:
        func: Function to register
    """

    def decorator(f: Callable) -> Callable:
        return _function_registry.register_function(f, endpoint="/task/reward")

    # Handle both @register_reward and @register_reward() usage patterns
    if func is not None and callable(func):
        return decorator(func)
    else:
        return decorator


def register_health(
    func: Optional[Callable] = None,
):
    """
    Decorator to register a function as a health check endpoint.

    The function will be exposed as a GET endpoint at /health
    instead of being registered as an MCP tool.

    If no function is provided, a default health check will be used.

    Args:
        func: Function to register for health checks
    """

    def decorator(f: Callable) -> Callable:
        return _function_registry.register_function(f, endpoint="/health", method="GET")

    # Handle both @register_health and @register_health() usage patterns
    if func is not None and callable(func):
        return decorator(func)
    else:
        return decorator


def get_function_registry() -> FunctionRegistry:
    """Get the global function registry."""
    return _function_registry
