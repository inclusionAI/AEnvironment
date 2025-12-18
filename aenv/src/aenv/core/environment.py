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
Environment class for managing tools using AEnv Scheduler API.
Updated to use FastMCP client for direct MCP communication.
"""

import asyncio
import json
import os
import traceback
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from urllib.parse import urlparse, urlunparse

import httpx
from fastmcp import Client
from fastmcp.client.transports import StreamableHttpTransport

from agents.tool import FunctionTool, Tool as OpenAITool
from agents.tool_context import ToolContext

from aenv.client.scheduler_client import AEnvSchedulerClient
from aenv.core.exceptions import EnvironmentError, ToolError
from aenv.core.logging import getLogger
from aenv.core.models import EnvInstance, EnvStatus
from aenv.core.tool import Tool

logger = getLogger("aenv.environment", "colored")


def make_mcp_url(aenv_url: str, port: str, path: str = "") -> str:
    if "://" not in aenv_url:
        aenv_url = f"http://{aenv_url}"

    p = urlparse(aenv_url)
    host = p.hostname or "127.0.0.1"
    new = p._replace(
        scheme="http",
        netloc=f"{host}:{port}",
        path=path,
        params="",
        query="",
        fragment="",
    )
    return urlunparse(new)


class ToolResult:
    """Result of a tool execution."""

    def __init__(self, content: List[Dict[str, Any]], is_error: bool = False):
        self.content = content
        self.is_error = is_error

    def __repr__(self):
        return f"ToolResult(content={self.content}, is_error={self.is_error})"


class Environment:
    """
    Environment class for managing tools using AEnv Scheduler API.

    This class provides a unified interface for:
    - Creating and managing environment instances
    - Discovering available tools
    - Executing tools with proper error handling
    - Managing environment lifecycle via AScheduler
    - Executing reward functions via HTTP endpoints
    """

    def __init__(
        self,
        env_name: str,
        datasource: str = "",
        ttl: str = "30m",
        environment_variables: Optional[Dict[str, str]] = None,
        arguments: Optional[List[str]] = None,
        aenv_url: Optional[str] = None,
        timeout: float = 60.0,
        startup_timeout: float = 500.0,
        max_retries: int = 10,
        api_key: Optional[str] = None,
    ):
        """
        Initialize environment.

        Args:
            env_name: Name of the environment
            datasource: the data source for mounting on the mcp server
            environment_variables: Optional environment variables set in envInstance
            arguments: Optional command line arguments for envInstance entrypoint
            aenv_url: AEnv Scheduler URL (defaults to env var AENV_SYSTEM_URL)
            timeout: Request timeout in seconds
            startup_timeout: Startup timeout in seconds
            ttl: Time to live in seconds defaults to 10 minutes
            max_retries: Maximum retry attempts for failed requests
            api_key: Optional API key for authentication
        """
        self.env_name = env_name
        self.datasource = datasource
        self.environment_variables = environment_variables or {}
        self.arguments = arguments or []
        self.dummy_instance_ip = os.getenv("DUMMY_INSTANCE_IP")

        if not aenv_url:
            aenv_url = self.dummy_instance_ip or os.getenv(
                "AENV_SYSTEM_URL", "http://localhost"
            )
        self.aenv_control_url = make_mcp_url(aenv_url, 8080)
        self.aenv_data_url = make_mcp_url(aenv_url, 8081, "/mcp")
        self.aenv_health_url = make_mcp_url(aenv_url, 8081, "/health")
        self.aenv_reward_url = make_mcp_url(aenv_url, 8081, "/task/reward")
        self.aenv_functions_base_url = make_mcp_url(aenv_url, 8081, "/functions")
        self.proxy_headers = {}
        self.timeout = timeout
        self._startup_timeout = startup_timeout
        self.ttl = ttl
        self.max_retries = max_retries
        self.api_key = api_key

        self._instance: Optional[EnvInstance] = None
        self._tools: Dict[str, Tool] = {}
        self._initialized = False
        self._client: Optional[AEnvSchedulerClient] = None
        self._mcp_client: Optional[Client] = None

    def _log_prefix(self) -> str:
        """Get log prefix with instance ID."""
        instance_id = (
            getattr(self._instance, "id", "None") if self._instance else "None"
        )
        return f"[ENV:{instance_id}]"

    async def __aenter__(self):
        """Async context manager entry."""
        await self.initialize()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit."""
        await self.release()

    async def initialize(self) -> bool:
        """
        Initialize the environment by creating EnvInstance via AScheduler.

        Returns:
            True if initialization successful

        Raises:
            EnvironmentError: If initialization fails
        """
        if self._initialized:
            logger.info(
                f"{self._log_prefix()} Environment '{self.env_name}' already initialized"
            )
            return True

        logger.info(f"{self._log_prefix()} Initializing environment: {self.env_name}")
        try:
            self._client = AEnvSchedulerClient(
                base_url=self.aenv_control_url,
                timeout=self.timeout,
                max_retries=self.max_retries,
                api_key=self.api_key,
            )

            if self.dummy_instance_ip:
                now_str = datetime.now(timezone.utc).isoformat(timespec="milliseconds")
                self._instance = EnvInstance(
                    id="test",
                    status="Running",
                    ip=self.dummy_instance_ip,
                    created_at=now_str,
                    updated_at=now_str,
                )
                await self._wait_for_healthy()
                self._initialized = True
                return

            await self._client.connect()
            await self._create_env_instance()
            self._initialized = True
            logger.info(
                f"{self._log_prefix()} Environment '{self.env_name}' initialized successfully"
            )
            return True

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to initialize environment '{self.env_name}': {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Control URL: {self.aenv_control_url} | "
                f"Data URL: {self.aenv_data_url} | "
                f"Timeout: {self.timeout}s | "
                f"Max retries: {self.max_retries} | "
                f"Traceback:\n{traceback.format_exc()}"
            )
            await self.release()
            raise EnvironmentError(
                f"Failed to initialize environment '{self.env_name}': {str(e)}"
            ) from e

    async def release(self):
        """Release environment resources."""
        logger.info(
            f"{self._log_prefix()} Releasing environment resources: {self.env_name}"
        )

        if self._mcp_client:
            try:
                await self._mcp_client.close()
                logger.debug(f"{self._log_prefix()} MCP client closed")
            except Exception as e:
                logger.warning(
                    f"{self._log_prefix()} Failed to close MCP client: {str(e)}"
                )
            finally:
                self._mcp_client = None

        if self._client:
            if self._instance and not self.dummy_instance_ip:
                try:
                    logger.debug(
                        f"{self._log_prefix()} Deleting environment instance: {self._instance.id}"
                    )
                    await self._client.delete_env_instance(self._instance.id)
                    logger.debug(f"{self._log_prefix()} Environment instance deleted")
                except Exception as e:
                    logger.warning(
                        f"{self._log_prefix()} Failed to delete environment instance: {str(e)}"
                    )

            try:
                logger.debug(f"{self._log_prefix()} Closing scheduler client...")
                await self._client.close()
                logger.debug(f"{self._log_prefix()} Scheduler client closed")
            except Exception as e:
                logger.warning(
                    f"{self._log_prefix()} Failed to close scheduler client: {str(e)}"
                )
            finally:
                self._client = None

        self._instance = None
        self._initialized = False
        logger.info(
            f"{self._log_prefix()} Environment resources released: {self.env_name}"
        )

    async def list_tools(self) -> List[Dict[str, Any]]:
        """
        List all available tools in the environment using MCP client.

        Returns:
            List of tool descriptors in MCP format
        """
        await self._ensure_initialized()

        try:
            client = await self._get_mcp_client()
            async with client:
                tools = await client.list_tools()
                logger.info(
                    f"{self._log_prefix()} Found {len(tools)} tools in environment {self.env_name}"
                )

                formatted_tools = [
                    {
                        "name": f"{self.env_name}/{tool.name}",
                        "description": tool.description,
                        "inputSchema": tool.inputSchema,
                    }
                    for tool in tools
                ]

                return formatted_tools
        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to list tools for {self.env_name}: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Initialized: {self._initialized} | "
                f"Instance ID: {getattr(self._instance, 'id', 'None')} | "
                f"MCP URL: {self.aenv_data_url}"
            )
            raise EnvironmentError(
                f"Failed to list tools for environment '{self.env_name}': {str(e)}"
            )

    async def list_openai_tools(self) -> List[OpenAITool]:
        tools = await self.list_tools()

        openai_tools: List[OpenAITool] = []
        for tool in tools:
            name = str(tool.get("name", ""))
            description = str(tool.get("description", ""))
            input_schema = tool.get("inputSchema")
            if not isinstance(input_schema, dict):
                input_schema = {"type": "object", "properties": {}}

            async def _on_invoke_tool(
                ctx: ToolContext[Any], input: str, *, _name: str = name
            ) -> Any:
                try:
                    args: Dict[str, Any] = json.loads(input) if input else {}
                except Exception as e:
                    return f"Invalid JSON input for tool '{_name}': {str(e)}"

                result = await self.call_tool(_name, args)
                if result.is_error:
                    return json.dumps(result.content, ensure_ascii=False)

                text_parts: List[str] = []
                for item in result.content:
                    if (
                        isinstance(item, dict)
                        and item.get("type") == "text"
                        and "text" in item
                    ):
                        text_parts.append(str(item.get("text")))
                    else:
                        text_parts.append(json.dumps(item, ensure_ascii=False))

                return "\n".join(text_parts)

            openai_tools.append(
                FunctionTool(
                    name=name,
                    description=description,
                    params_json_schema=input_schema,
                    on_invoke_tool=_on_invoke_tool,
                    strict_json_schema=True,
                )
            )

        return openai_tools

    async def list_functions(self) -> Dict[str, Any]:
        """
        List all registered functions in the environment including reward and health.

        Returns:
            Dictionary containing categorized function lists (functions, reward, health)
        """
        await self._ensure_initialized()

        try:
            # Build the functions/list endpoint URL
            functions_list_url = f"{self.aenv_functions_base_url}/list"

            # Call the endpoint using _call_function
            # _call_function returns result.get("data", {}), so result is already the data part
            result = await self._call_function(
                functions_list_url,
                arguments={},
                method="GET",
                ensure_initialized=False,  # Already initialized above
            )

            # _call_function already extracts the "data" field, so result should be the function data directly
            # Handle both cases: direct data or wrapped in "data" key
            if isinstance(result, dict):
                # If wrapped in "data" key, extract it
                if "data" in result and isinstance(result["data"], dict):
                    function_data = result["data"]
                else:
                    function_data = result

                # Log the result
                total = function_data.get("total", 0)
                logger.info(
                    f"{self._log_prefix()} Found {total} functions in environment {self.env_name}"
                )
                return function_data

            # Fallback: return as-is
            logger.warning(
                f"{self._log_prefix()} Unexpected response format from list_functions: {result}"
            )
            return result
        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to list functions for {self.env_name}: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Initialized: {self._initialized} | "
                f"Instance ID: {getattr(self._instance, 'id', 'None')} | "
                f"MCP URL: {self.aenv_data_url}"
            )
            raise EnvironmentError(
                f"Failed to list functions for environment '{self.env_name}': {str(e)}"
            )

    async def call_reward(
        self,
        arguments: Dict[str, Any],
        timeout: Optional[float] = None,
    ) -> Dict[str, Any]:
        """
        Execute the reward function via the /task/reward endpoint.

        Args:
            arguments: Arguments to pass to the reward function
            timeout: Override default timeout

        Returns:
            Reward function execution result

        Raises:
            EnvironmentError: If reward execution fails
        """
        return await self._call_function(
            self.aenv_reward_url, arguments=arguments, timeout=timeout
        )

    async def _call_function(
        self,
        function_url: str,
        arguments: Dict[str, Any] = {},
        method: str = "POST",
        timeout: Optional[float] = None,
        ensure_initialized: bool = True,
    ) -> Dict[str, Any]:
        """
        Execute a registered function via HTTP endpoint.

        Args:
            function_url: url of the registered function
            arguments: Arguments to pass to the function
            timeout: Override default timeout

        Returns:
            Function execution result

        Raises:
            EnvironmentError: If function execution fails
        """
        if ensure_initialized:
            await self._ensure_initialized()

        logger.info(
            f"{self._log_prefix()} Executing function in environment {self.env_name} with url={function_url} proxy_headers={self.proxy_headers}, timeout={timeout}"
        )

        try:
            async with httpx.AsyncClient(timeout=timeout or self.timeout) as client:
                if method == "GET":
                    params = {
                        k: (json.dumps(v) if isinstance(v, (dict, list)) else v)
                        for k, v in arguments.items()
                    }

                    response = await client.request(
                        method.upper(),
                        function_url,
                        params=params,
                        headers=self.proxy_headers,
                    )

                elif method in ("POST", "PUT", "PATCH", "DELETE"):
                    response = await client.request(
                        method.upper(),
                        function_url,
                        json=arguments,
                        headers=self.proxy_headers,
                    )
                else:
                    raise ValueError(
                        f"{self._log_prefix()} Unsupported HTTP method: {method}"
                    )

                response.raise_for_status()
                result = response.json()

                if not result.get("success", False):
                    raise EnvironmentError(result.get("error", "Unknown error"))

                logger.info(
                    f"{self._log_prefix()} Function '{function_url}' executed successfully with result={result}"
                )
                return result.get("data", {})

        except httpx.HTTPStatusError as e:
            # Extract error details from response body
            server_error = None
            try:
                error_body = e.response.json()
                if isinstance(error_body, dict) and "error" in error_body:
                    server_error = error_body.get("error")
                else:
                    server_error = str(error_body)
            except Exception:
                try:
                    server_error = e.response.text
                except Exception:
                    server_error = f"HTTP {e.response.status_code}"

            error_msg = str(e)
            if server_error:
                error_msg = f"{error_msg} | Server error: {server_error}"

            logger.error(
                f"{self._log_prefix()} Function '{function_url}' execution http request failed: {error_msg} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Arguments: {arguments} | "
                f"Timeout: {timeout or self.timeout}s | "
                f"Function URL: {function_url}"
            )
            raise EnvironmentError(
                f"Function '{function_url}' execution failed: {error_msg}"
            )
        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Function '{function_url}' execution failed: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Arguments: {arguments} | "
                f"Timeout: {timeout or self.timeout}s | "
                f"Function URL: {function_url}"
            )
            raise EnvironmentError(
                f"Function '{function_url}' execution failed: {str(e)}"
            )

    async def check_health(
        self,
        arguments: Dict[str, Any],
        timeout: Optional[float] = None,
    ) -> Dict[str, Any]:
        """
        Execute the check-health function via the /health endpoint.

        Returns:

        Raises:
            EnvironmentError: If health check execution fails
        """
        await self._ensure_initialized()

        logger.info(
            f"{self._log_prefix()} Executing health function in environment {self.env_name} with url:{self.aenv_health_url}"
        )

        try:
            async with httpx.AsyncClient(timeout=timeout or self.timeout) as client:
                response = await client.get(
                    self.aenv_health_url, params=arguments, headers=self.proxy_headers
                )

                response.raise_for_status()
                result = response.json()
                logger.info(
                    f"{self._log_prefix()} Health function executed successfully with result: {result}"
                )
                return result

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Health function execution failed: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Arguments: {arguments} | "
                f"Timeout: {timeout or self.timeout}s | "
                f"Health URL: {self.aenv_health_url}"
            )
            raise EnvironmentError(f"Health function execution failed: {str(e)}")

    async def call_tool(
        self,
        tool_name: str,
        arguments: Dict[str, Any],
        timeout: Optional[float] = None,
    ) -> ToolResult:
        """
        Execute a tool with given arguments using MCP client.

        Args:
            tool_name: Name of the tool (format: "env_name/tool_name" or just "tool_name")
            arguments: Tool arguments
            timeout: Override default timeout

        Returns:
            Tool execution result

        Raises:
            ToolError: If tool execution fails
            ToolTimeoutError: If execution times out
        """
        await self._ensure_initialized()

        # Parse tool name
        if "/" in tool_name:
            env_name, actual_tool_name = tool_name.split("/", 1)
            if env_name != self.env_name:
                raise ToolError(
                    f"Tool '{tool_name}' not found in environment '{self.env_name}'"
                )
        else:
            actual_tool_name = tool_name

        logger.info(
            f"{self._log_prefix()} Executing tool: {actual_tool_name} in environment {self.env_name}"
        )

        try:
            client = await self._get_mcp_client()
            async with client:
                result = await client.call_tool_mcp(
                    name=actual_tool_name, arguments=arguments, timeout=timeout
                )

                # Convert FastMCP result to ToolResult
                content = []
                if result.content:
                    for item in result.content:
                        if hasattr(item, "text") and item.text:
                            content.append({"type": "text", "text": item.text})
                        elif hasattr(item, "type") and hasattr(item, "data"):
                            content.append({"type": item.type, "data": item.data})
                        else:
                            content.append({"type": "text", "text": str(item)})

                return ToolResult(content=content, is_error=result.isError)

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Tool execution failed: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Tool: {actual_tool_name} | "
                f"Arguments: {arguments} | "
                f"Timeout: {timeout or self.timeout}s | "
                f"MCP URL: {self.aenv_data_url}"
            )
            raise ToolError(f"Tool '{actual_tool_name}' execution failed: {str(e)}")

    async def get_env_info(self) -> Dict[str, Any]:
        """Get environment information."""
        await self._ensure_initialized()

        return {
            "name": self.env_name,
            "instance_id": self._instance.id,
            "status": self._instance.status,
            "ip": self._instance.ip,
            "created_at": self._instance.created_at,
            "updated_at": self._instance.updated_at,
        }

    async def _ensure_initialized(self):
        """Ensure environment is initialized, with proper async locking."""
        if self._initialized:
            return

        # Use a simple async lock pattern to prevent race conditions
        if not hasattr(self, "_init_lock"):
            self._init_lock = asyncio.Lock()

        async with self._init_lock:
            # Double-check after acquiring lock
            if not self._initialized:
                await self.initialize()

    async def _wait_for_healthy(self, timeout: float = 300.0) -> None:
        """Wait for environment instance to be healthy."""
        logger.info(
            f"{self._log_prefix()} Waiting for environment {self.env_name} to be healthy..."
        )
        try:
            self.proxy_headers = {
                "AEnvCore-MCPProxy-URL": make_mcp_url(self._instance.ip, 8081),
                "AEnvCore-EnvInstance-ID": self._instance.id,
            }

            check_interval = 2.0
            start_time = asyncio.get_event_loop().time()
            times = 0
            result = ""

            while True:
                logger.info(
                    f"{self._log_prefix()} check {self.env_name} health at round {times} with url: {self.aenv_health_url}, last_check_result={result}"
                )

                try:
                    result = await self._call_function(
                        self.aenv_health_url,
                        timeout=3.0,
                        method="GET",
                        ensure_initialized=False,
                    )

                    logger.debug(
                        f"{self._log_prefix()} check {self.env_name} health result={result}"
                    )

                    if (
                        result.get("status") == "success"
                        or result.get("status") == "healthy"
                    ):
                        logger.info(
                            f"{self._log_prefix()} Environment {self.env_name} is healthy"
                        )
                        return

                except Exception as e:
                    logger.debug(
                        f"{self._log_prefix()} Health check failed: {str(e)}, retrying..."
                    )

                if asyncio.get_event_loop().time() - start_time > timeout:
                    raise EnvironmentError(
                        f"Timeout waiting for status healthy: {self._instance.id}"
                    )

                await asyncio.sleep(check_interval)
                times = times + 1

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to wait for environment healthy: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Instance ID: {getattr(self._instance, 'id', 'None')} | "
                f"Timeout: {timeout}s | "
                f"Health URL: {self.aenv_health_url}"
            )
            raise EnvironmentError(
                f"Failed to wait for environment '{self.env_name}' to be healthy: {str(e)}"
            )

    async def wait_for_ready(self, timeout: float = 300.0) -> None:
        """Wait for environment instance to be ready."""
        if not self._client or not self._instance:
            await self.initialize()

        logger.info(
            f"{self._log_prefix()} Waiting for environment {self.env_name} to be ready with timeout {timeout}s..."
        )
        try:
            instance = await self._client.wait_for_status(
                self._instance.id, EnvStatus.RUNNING, timeout=timeout
            )

            self._instance = instance
            await self._wait_for_healthy()
        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to wait for environment ready: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Instance ID: {getattr(self._instance, 'id', 'None')} | "
                f"Timeout: {timeout}s | "
                f"Control URL: {self.aenv_control_url}"
            )
            raise EnvironmentError(
                f"Failed to wait for environment '{self.env_name}' to be ready: {str(e)}"
            )

    async def _create_env_instance(self):
        """Create environment instance via AScheduler."""
        if not self._client:
            raise EnvironmentError("Scheduler client not initialized")

        logger.info(
            f"{self._log_prefix()} Creating environment instance: {self.env_name}"
        )
        try:
            self._instance = await self._client.create_env_instance(
                name=self.env_name,
                datasource=self.datasource,
                environment_variables=self.environment_variables,
                arguments=self.arguments,
                ttl=self.ttl,
            )
            logger.info(
                f"{self._log_prefix()} Environment instance created with ID: {self._instance.id}"
            )

            await self.wait_for_ready(timeout=self._startup_timeout)
            logger.info(f"{self._log_prefix()} Environment ready: {self.env_name}")

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to create environment instance: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Control URL: {self.aenv_control_url}"
            )
            raise EnvironmentError(f"Failed to create environment instance: {str(e)}")

    async def call_function(
        self,
        function_name: str,
        arguments: Dict[str, Any],
        timeout: Optional[float] = None,
    ) -> Dict[str, Any]:
        """
        Execute a registered function via HTTP endpoint.

        Args:
            function_name: name of the registered function
            arguments: Arguments to pass to the function
            timeout: Override default timeout

        Returns:
            Function execution result

        Raises:
            EnvironmentError: If function execution fails
        """
        function_url = f"{self.aenv_functions_base_url}/{function_name}"
        return await self._call_function(
            function_url, arguments=arguments, timeout=timeout
        )

    async def _get_mcp_client(self) -> Client:
        """Get or create MCP client for direct MCP communication."""
        if not self._instance or not self._instance.ip:
            error_msg = "Environment instance IP not available"
            logger.error(f"{self._log_prefix()} {error_msg}")
            logger.error(f"{self._log_prefix()} Instance details: {self._instance}")
            raise EnvironmentError(error_msg)

        if self._mcp_client:
            logger.debug(f"{self._log_prefix()} Reusing existing MCP client")
            return self._mcp_client

        try:
            logger.info(
                f"{self._log_prefix()} Creating MCP client with headers: {self.proxy_headers}, URL: { self.aenv_data_url}"
            )

            self._mcp_client = Client(
                transport=StreamableHttpTransport(
                    self.aenv_data_url, headers=self.proxy_headers
                ),
                timeout=self.timeout,
            )

            logger.debug(f"{self._log_prefix()} MCP client created successfully")
            return self._mcp_client

        except Exception as e:
            logger.error(
                f"{self._log_prefix()} Failed to create MCP client: {str(e)} | "
                f"Type: {type(e).__name__} | "
                f"Environment: {self.env_name} | "
                f"Instance IP: {getattr(self._instance, 'ip', 'None')} | "
                f"Data URL: {self.aenv_data_url} | "
                f"Timeout: {self.timeout}s "
            )
            raise EnvironmentError(f"Failed to create MCP client: {str(e)}")
