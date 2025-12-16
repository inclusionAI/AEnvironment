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
MCP health check with efficient client reuse and backward compatibility.
"""

import time
from typing import Any, Dict, Optional

from fastmcp import Client
from fastmcp.client.transports import StreamableHttpTransport

from aenv.core.logging import getLogger

logger = getLogger(__name__, "colored")


# Module-level client cache
_mcp_client: Optional[Client] = None
_last_active_time: float = 0.0


def update_active_time():
    """Update the last active time."""
    global _last_active_time
    _last_active_time = time.time()


def _get_mcp_client() -> Client:
    """Get or create MCP client."""
    global _mcp_client
    if _mcp_client is None:
        _mcp_client = Client(
            StreamableHttpTransport(
                url="http://localhost:8081/mcp",
                headers={"Content-Type": "application/json"},
            )
        )
    return _mcp_client


async def close_health_clients():
    """Close cached clients."""
    global _mcp_client
    if _mcp_client is not None:
        await _mcp_client.aclose()
        _mcp_client = None


async def check_mcp_health(
    mcp_url: str = "http://localhost:8081/mcp",
) -> Dict[str, Any]:
    """
    Check MCP server health using cached clients for efficiency.

    This is the new simplified API that health_registry.py uses.
    """
    try:
        client = _get_mcp_client()

        async with client:
            tools = await client.list_tools()

            return {
                "status": "success",
                "message": "MCP server is healthy",
                "details": {"tool_count": len(tools)},
            }

    except Exception as e:
        return {"status": "error", "message": str(e)}


async def default_health_handler() -> Dict[str, Any]:
    """Enhanced health check handler that validates MCP functionality."""
    global _last_active_time
    try:
        result = await check_mcp_health()

        result.update({"last_active": _last_active_time})
        return result

    except Exception as e:
        logger.error(f"Health handler error: {str(e)}")
        return {
            "status": "error",
            "message": f"Health check failed: {str(e)}",
            "last_active": _last_active_time,
        }
