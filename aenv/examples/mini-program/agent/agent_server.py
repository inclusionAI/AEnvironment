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
Agent server for mini program IDE.
Integrates OpenAI API with MCP tools via AEnvironment.
"""

import asyncio
import json
import os
from typing import Any, Dict, List, Optional

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect
from fastapi.responses import HTMLResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

try:
    from openai import OpenAI, RateLimitError
except ImportError:
    OpenAI = None
    RateLimitError = None

from aenv import Environment
from aenv.core.logging import getLogger

logger = getLogger("mini-program.agent", "colored")

app = FastAPI(title="Mini Program IDE Agent Server")

# Serve static files
# agent_server.py is in agent/, so frontend/ is in ../frontend
static_dir = os.path.join(os.path.dirname(__file__), "..", "frontend")
if os.path.exists(static_dir):
    app.mount("/static", StaticFiles(directory=static_dir), name="static")
    logger.info(f"Serving static files from: {static_dir}")
else:
    logger.warning(f"Static directory not found: {static_dir}")


class ChatMessage(BaseModel):
    """Chat message model."""

    role: str  # "user" or "assistant"
    content: str
    tool_calls: Optional[List[Dict[str, Any]]] = None
    tool_results: Optional[List[Dict[str, Any]]] = None


class ChatRequest(BaseModel):
    """Chat request model."""

    message: str
    conversation_id: Optional[str] = None
    stream: bool = False


class ToolCall(BaseModel):
    """Tool call model."""

    name: str
    arguments: Dict[str, Any]


# Global environment instance
_env: Optional[Environment] = None
# Global OpenAI client instance
_openai_client: Optional[Any] = None


def get_openai_client() -> Any:
    """Get or create OpenAI client instance with proper configuration."""
    global _openai_client

    if _openai_client is None:
        if not OpenAI:
            raise RuntimeError(
                "OpenAI client not available. Please install openai package."
            )

        api_key = os.getenv("OPENAI_API_KEY")
        if not api_key:
            raise RuntimeError("OPENAI_API_KEY environment variable not set")

        # Get configuration from environment variables
        base_url = os.getenv("OPENAI_BASE_URL")  # For custom API endpoints

        # Create client with timeout configuration
        client_kwargs = {
            "api_key": api_key,
            "timeout": 300.0,  # Default 5 minutes
        }

        if base_url:
            client_kwargs["base_url"] = base_url

        _openai_client = OpenAI(**client_kwargs)
        logger.info(
            f"Initialized OpenAI client with base_url={base_url or 'default'}, "
            f"timeout=300s"
        )

    return _openai_client


async def get_environment() -> Environment:
    """Get or create environment instance."""
    global _env
    if _env is None:
        env_name = os.getenv("AENV_ENV_NAME", "mini-program@1.0.0")
        _env = Environment(env_name)
        await _env.initialize()
    return _env


async def call_tool(tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
    """
    Call a tool via environment.

    Args:
        tool_name: Name of the tool to call (e.g., "read_file", "write_file")
        arguments: Arguments for the tool

    Returns:
        Tool execution result
    """
    try:
        env = await get_environment()
        # Get available tools
        tools = await env.list_tools()

        # Find matching tool - handle both list and dict formats
        tool_found = None

        if isinstance(tools, list):
            # tools is a list of dicts: [{"name": "...", "description": "...", ...}, ...]
            for tool_info in tools:
                tool_full_name = tool_info.get("name", "")
                # Tool name might be "env_name/tool_name" or just "tool_name"
                tool_base_name = (
                    tool_full_name.split("/")[-1]
                    if "/" in tool_full_name
                    else tool_full_name
                )
                if tool_base_name == tool_name:
                    tool_found = tool_full_name
                    break

            # If not found, try with env prefix
            if not tool_found:
                env_tool_name = f"{env.env_name}/{tool_name}"
                for tool_info in tools:
                    if tool_info.get("name") == env_tool_name:
                        tool_found = env_tool_name
                        break

            # Last resort: try direct call
            if not tool_found:
                tool_found = tool_name

        elif isinstance(tools, dict):
            # tools is a dict: {"tool_name": {"description": "...", ...}, ...}
            for tool_key, tool_info in tools.items():
                # Tool key might be "env_name/tool_name" or just "tool_name"
                tool_base_name = (
                    tool_key.split("/")[-1] if "/" in tool_key else tool_key
                )
                if tool_base_name == tool_name:
                    tool_found = tool_key
                    break

            if not tool_found:
                # Try with env name prefix
                env_tool_name = f"{env.env_name}/{tool_name}"
                if env_tool_name in tools:
                    tool_found = env_tool_name
                else:
                    # Try direct call
                    tool_found = tool_name
        else:
            logger.warning(f"Unexpected tools type: {type(tools)}, trying direct call")
            tool_found = tool_name

        logger.debug(f"Calling tool '{tool_found}' (requested: '{tool_name}')")
        tool_result = await env.call_tool(tool_found, arguments)

        # Convert ToolResult to serializable format
        # ToolResult.content is a list of {"type": "text", "text": "..."} objects
        # The text field contains JSON string of the actual tool result
        serialized_result = _make_json_serializable(tool_result)

        # Extract actual tool result from ToolResult.content if needed
        # For tools that return dict, it's serialized in content[0].text as JSON string
        if isinstance(serialized_result, dict) and "content" in serialized_result:
            content_list = serialized_result.get("content", [])
            if content_list and isinstance(content_list, list):
                for item in content_list:
                    if isinstance(item, dict) and item.get("type") == "text":
                        text_content = item.get("text", "")
                        if text_content:
                            try:
                                # Try to parse JSON - if successful, it's the actual tool result
                                parsed = json.loads(text_content)
                                # Return the parsed result directly
                                return {"success": True, "result": parsed}
                            except (json.JSONDecodeError, TypeError):
                                # Not JSON, return as-is
                                pass

        return {"success": True, "result": serialized_result}
    except Exception as e:
        logger.error(f"Error calling tool {tool_name}: {str(e)}", exc_info=True)
        return {"success": False, "error": str(e)}


def _make_json_serializable(obj: Any) -> Any:
    """
    Convert objects to JSON-serializable format.

    Args:
        obj: Object to convert

    Returns:
        JSON-serializable object
    """
    # Import here to avoid circular imports
    try:
        from aenv.core.environment import ToolResult
    except ImportError:
        ToolResult = None

    # Handle ToolResult objects
    if ToolResult and isinstance(obj, ToolResult):
        return {
            "content": obj.content,
            "is_error": obj.is_error,
        }
    # Handle basic types
    elif isinstance(obj, (str, int, float, bool, type(None))):
        return obj
    # Handle dictionaries
    elif isinstance(obj, dict):
        return {k: _make_json_serializable(v) for k, v in obj.items()}
    # Handle lists
    elif isinstance(obj, list):
        return [_make_json_serializable(item) for item in obj]
    # Handle tuples
    elif isinstance(obj, tuple):
        return tuple(_make_json_serializable(item) for item in obj)
    # Handle other objects - try to convert to dict or string
    else:
        try:
            # Check if it's a ToolResult-like object (has content and is_error attributes)
            if hasattr(obj, "content") and hasattr(obj, "is_error"):
                return {
                    "content": _make_json_serializable(obj.content),
                    "is_error": obj.is_error,
                }
            # Try to convert to dict
            elif hasattr(obj, "__dict__"):
                return _make_json_serializable(obj.__dict__)
            # Fallback to string representation
            else:
                return str(obj)
        except Exception as e:
            logger.warning(f"Failed to serialize object {type(obj)}: {str(e)}")
            return str(obj)


def format_tools_for_openai(tools: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """
    Format MCP tools for OpenAI API.

    Args:
        tools: List of tool descriptors from MCP

    Returns:
        List of tools in OpenAI format
    """
    openai_tools = []
    for tool in tools:
        try:
            tool_name = tool.get("name", "")
            if not tool_name:
                logger.warning(f"Skipping tool with no name: {tool}")
                continue

            # Remove env prefix if present
            tool_base_name = tool_name.split("/")[-1]

            # Get inputSchema - it might be nested or direct
            input_schema = tool.get("inputSchema", {})
            if not input_schema and "inputSchema" in tool:
                input_schema = tool["inputSchema"]

            # If inputSchema is empty, try to get it from tool_info structure
            if not input_schema:
                logger.debug(f"Tool {tool_name} has no inputSchema, using empty dict")
                input_schema = {}

            openai_tool = {
                "type": "function",
                "function": {
                    "name": tool_base_name,
                    "description": tool.get("description", ""),
                    "parameters": (
                        input_schema
                        if input_schema
                        else {"type": "object", "properties": {}}
                    ),
                },
            }
            openai_tools.append(openai_tool)
            logger.debug(f"Formatted tool: {tool_base_name}")
        except Exception as e:
            logger.error(f"Error formatting tool {tool}: {str(e)}", exc_info=True)

    return openai_tools


async def call_openai_with_retry(
    client: Any,
    model: str,
    messages: List[Dict[str, Any]],
    tools: Optional[List[Dict[str, Any]]] = None,
    tool_choice: str = "auto",
    max_retries: int = 3,
    initial_delay: float = 1.0,
    max_delay: float = 60.0,
    backoff_factor: float = 2.0,
) -> Any:
    """
    Call OpenAI API with retry logic for rate limit errors.

    Args:
        client: OpenAI client instance
        model: Model name
        messages: List of messages
        tools: Optional list of tools
        tool_choice: Tool choice strategy
        max_retries: Maximum number of retries
        initial_delay: Initial delay in seconds before first retry
        max_delay: Maximum delay in seconds between retries
        backoff_factor: Factor to multiply delay by for each retry

    Returns:
        OpenAI API response

    Raises:
        Exception: If all retries fail
    """
    delay = initial_delay

    for attempt in range(max_retries + 1):
        try:
            response = client.chat.completions.create(
                model=model,
                messages=messages,
                tools=tools if tools else None,
                tool_choice=tool_choice,
            )
            return response
        except Exception as e:
            # Check if it's a rate limit error
            is_rate_limit = False
            if RateLimitError and isinstance(e, RateLimitError):
                is_rate_limit = True
            elif hasattr(e, "status_code") and e.status_code == 429:
                is_rate_limit = True
            elif "429" in str(e) or "RateLimit" in str(type(e).__name__):
                is_rate_limit = True

            if is_rate_limit and attempt < max_retries:
                # Calculate delay with exponential backoff
                current_delay = min(delay, max_delay)
                logger.warning(
                    f"Rate limit error (attempt {attempt + 1}/{max_retries + 1}). "
                    f"Retrying after {current_delay:.2f} seconds..."
                )
                await asyncio.sleep(current_delay)
                delay *= backoff_factor
            else:
                # Not a rate limit error, or max retries reached
                if is_rate_limit:
                    logger.error(
                        f"Rate limit error after {max_retries + 1} attempts. "
                        f"Giving up."
                    )
                raise


@app.get("/", response_class=HTMLResponse)
async def root():
    """Serve the main HTML page."""
    html_path = os.path.join(static_dir, "index.html")
    logger.debug(f"Looking for index.html at: {html_path}")
    logger.debug(f"Static dir exists: {os.path.exists(static_dir)}")
    logger.debug(f"HTML file exists: {os.path.exists(html_path)}")

    if os.path.exists(html_path):
        with open(html_path, "r", encoding="utf-8") as f:
            return HTMLResponse(content=f.read())
    else:
        error_msg = (
            f"<h1>Mini Program IDE</h1>"
            f"<p>Static files not found.</p>"
            f"<p>Expected path: {html_path}</p>"
            f"<p>Static dir: {static_dir}</p>"
            f"<p>Current working dir: {os.getcwd()}</p>"
            f"<p>Agent server file: {__file__}</p>"
        )
        logger.error(f"HTML file not found: {html_path}")
        return HTMLResponse(content=error_msg)


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "ok"}


@app.post("/api/chat")
async def chat(request: ChatRequest):
    """
    Handle chat requests with OpenAI API and MCP tools.

    Args:
        request: Chat request

    Returns:
        Chat response
    """
    try:
        client = get_openai_client()
        env = await get_environment()

        # Get available tools
        tools_result = await env.list_tools()
        logger.info(f"Retrieved {len(tools_result)} tools from environment")

        # Handle both list and dict formats
        tools_list = []
        if isinstance(tools_result, list):
            # list_tools() returns List[Dict[str, Any]]
            tools_list = tools_result
            logger.debug("Tools are in list format, using directly")
        elif isinstance(tools_result, dict):
            # Convert dict to list format
            for tool_name, tool_info in tools_result.items():
                tools_list.append(
                    {
                        "name": tool_name,
                        "description": tool_info.get("description", ""),
                        "inputSchema": tool_info.get("inputSchema", {}),
                    }
                )
            logger.debug("Tools are in dict format, converted to list")
        else:
            logger.warning(
                f"Unexpected tools_result type: {type(tools_result)}, value: {tools_result}"
            )

        logger.info(f"Prepared {len(tools_list)} tools for formatting")
        openai_tools = format_tools_for_openai(tools_list)
        logger.info(
            f"Formatted {len(tools_list)} tools into {len(openai_tools)} OpenAI tools"
        )

        # Prepare messages
        # List available tool names for the system prompt
        available_tool_names = (
            [tool["function"]["name"] for tool in openai_tools] if openai_tools else []
        )
        system_content = (
            "You are an AI assistant helping users develop mini programs (web applications). "
            "You can read and write files, execute Python code for validation, and create complete web applications.\n\n"
            f"Available tools: {', '.join(available_tool_names)}\n\n"
            "CRITICAL WORKFLOW CONSTRAINTS:\n"
            "1. CONSISTENCY ACROSS MULTIPLE TURNS:\n"
            "   - ALWAYS use the SAME file names and structure across all conversation turns\n"
            "   - If you created 'index.html' in a previous turn, continue using 'index.html' (not 'app.html' or 'main.html')\n"
            "   - If you created 'style.css', keep using 'style.css' consistently\n"
            "   - Maintain consistent variable names, function names, and class names across files\n"
            "   - This consistency speeds up reasoning and prevents confusion\n"
            "   - Before creating new files, ALWAYS use list_files to check existing files first\n\n"
            "2. WORK DIRECTORY: All files must be created in the root directory (no subdirectories).\n"
            "   - Use simple filenames like 'index.html', 'style.css', 'game.js'\n"
            "   - DO NOT use paths like '/tmp/code/index.html' or 'src/index.html'\n"
            "   - Just use 'index.html' directly\n\n"
            "3. ENTRY POINT: The main HTML file MUST be named 'index.html'.\n"
            "   - This is the entry point that will be displayed in the preview\n"
            "   - All other files (CSS, JS) should be referenced from index.html\n"
            "   - If index.html already exists, modify it instead of creating a new file\n\n"
            "4. FILE STRUCTURE:\n"
            "   - index.html: Main HTML structure (REQUIRED, use consistently)\n"
            "   - style.css: CSS styling (optional, can be inline, but if used, keep the name consistent)\n"
            "   - *.js: JavaScript logic (optional, can be inline, but if used, keep the name consistent)\n"
            "   - Keep it simple - prefer inline styles/scripts for small projects\n\n"
            "5. WORKFLOW:\n"
            "   - When given a task (e.g., 'create a snake game'):\n"
            "     a) ALWAYS first use list_files to check existing files\n"
            "     b) If files exist, read them first to understand current state\n"
            "     c) Plan the implementation (HTML + CSS + JS)\n"
            "     d) Create or modify index.html with complete structure\n"
            "     e) Add CSS styling (inline or separate file, but be consistent)\n"
            "     f) Add JavaScript logic (inline or separate file, but be consistent)\n"
            "     g) After writing HTML, use validate_html to check for errors\n"
            "     h) Use check_responsive_design to ensure responsive design requirements are met\n"
            "     i) Use execute_python_code to validate logic if needed\n"
            "     j) Continue iterating until the application is complete\n"
            "   - Don't stop after one tool call - keep working until done\n"
            "   - Verify files work together correctly\n"
            "   - In subsequent turns, maintain the same file names and structure\n\n"
            "6. RESPONSIVE DESIGN (CRITICAL - MUST FOLLOW EXACTLY):\n"
            "   - ALL games/applications MUST be responsive and fit the preview window WITHOUT scrolling\n"
            "   - HTML structure:\n"
            '     * <html><head><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>\n'
            '     * <body style="margin:0;padding:0;overflow:hidden;width:100vw;height:100vh;display:flex;align-items:center;justify-content:center;">\n'
            "   - Use viewport-relative units (vw, vh, %, clamp) instead of fixed pixels\n"
            "   - Canvas elements MUST scale to fit:\n"
            "     * CSS: canvas { max-width: min(100vw, 800px); max-height: min(100vh, 600px); width: auto; height: auto; display: block; margin: 0 auto; }\n"
            "     * JavaScript: Calculate scale = Math.min(window.innerWidth / canvas.width, window.innerHeight / canvas.height, 1)\n"
            "   - Container elements MUST use:\n"
            "     * width: 100%; height: 100vh; max-width: 100vw; max-height: 100vh;\n"
            "     * overflow: hidden; display: flex; align-items: center; justify-content: center;\n"
            "     * padding: clamp(5px, 1vw, 10px);\n"
            "   - Body MUST have: margin: 0; padding: 0; overflow: hidden; width: 100vw; height: 100vh;\n"
            "   - All text and buttons MUST use relative units (em, rem, vw, vh) or clamp()\n"
            "   - NEVER use fixed pixel sizes for layout (only for small details like borders)\n"
            "   - Test that content fits within viewport without ANY scrolling\n\n"
            "7. CODE QUALITY:\n"
            "   - Write clean, well-structured code\n"
            "   - Include proper HTML5 structure\n"
            "   - Use semantic HTML elements\n"
            "   - Ensure JavaScript is error-free\n"
            "   - Test the complete application flow\n"
            "   - Maintain consistent naming conventions across all code\n"
            "   - After writing HTML files, ALWAYS use validate_html to check for structural errors\n"
            "   - Use check_responsive_design to verify responsive design compliance\n\n"
            "8. VERSION NUMBER (CRITICAL):\n"
            "   - EVERY time you modify index.html, you MUST increment and display a version number\n"
            "   - Add a visible version indicator in the HTML (e.g., in a corner or header)\n"
            "   - Format: 'Version: X' or 'vX' where X is an incrementing number starting from 1\n"
            "   - If index.html already exists, read it first to find the current version number\n"
            "   - Increment the version number by 1 each time you modify the file\n"
            "   - Display it prominently so users can see when the page has been updated\n"
            '   - Example: <div style="position:fixed;top:10px;right:10px;background:rgba(0,0,0,0.7);color:#fff;padding:5px 10px;border-radius:5px;font-size:12px;">Version: 2</div>\n'
            "   - This helps users verify that refresh is working correctly\n\n"
            "9. TOOL USAGE:\n"
            "   - Use 'write_file' to create/modify files (NOT 'create_file')\n"
            "   - Use 'read_file' to check existing files before modifying\n"
            "   - Use 'list_files' to see what files exist (ALWAYS do this first in each turn)\n"
            "   - Use 'validate_html' after writing HTML files to check for errors and get suggestions\n"
            "   - Use 'check_responsive_design' to verify your code meets responsive design requirements\n"
            "   - Use 'execute_python_code' for calculations or validation\n"
            "   - Use 'log_browser_console' to check for browser console errors (if available)\n\n"
            "Remember: The goal is to create a working mini program that can be previewed in a browser. "
            "Always create index.html as the entry point, and ensure all code is complete and functional. "
            "Most importantly, maintain consistency in file names and structure across all conversation turns to speed up reasoning. "
            "ALWAYS include and increment the version number in index.html so users can verify updates."
        )

        messages = [
            {
                "role": "system",
                "content": system_content,
            },
            {"role": "user", "content": request.message},
        ]

        # Call OpenAI API with retry logic
        model = os.getenv("OPENAI_MODEL", "gpt-4")
        logger.info(
            f"Calling OpenAI API with model={model}, "
            f"message_length={len(request.message)}, "
            f"tools_count={len(openai_tools) if openai_tools else 0}"
        )

        try:
            response = await call_openai_with_retry(
                client=client,
                model=model,
                messages=messages,
                tools=openai_tools if openai_tools else None,
                tool_choice="auto",
            )
        except Exception as e:
            logger.error(
                f"OpenAI API call failed after retries: {str(e)}", exc_info=True
            )
            raise

        message = response.choices[0].message

        # Multi-turn tool calling loop - continue until no more tool calls
        max_iterations = 10  # Prevent infinite loops
        iteration = 0
        all_tool_results = []

        while message.tool_calls and iteration < max_iterations:
            iteration += 1
            logger.info(f"Tool calling iteration {iteration}/{max_iterations}")
            # First, add the assistant message with tool_calls
            messages.append(
                {
                    "role": "assistant",
                    "content": message.content or None,
                    "tool_calls": [
                        {
                            "id": tc.id,
                            "type": tc.type,
                            "function": {
                                "name": tc.function.name,
                                "arguments": tc.function.arguments,
                            },
                        }
                        for tc in message.tool_calls
                    ],
                }
            )

            # Then add tool result messages
            tool_results = []
            for tool_call in message.tool_calls:
                tool_name = tool_call.function.name
                try:
                    arguments = json.loads(tool_call.function.arguments)
                except json.JSONDecodeError:
                    arguments = {}

                logger.info(f"Calling tool: {tool_name} with args: {arguments}")
                result = await call_tool(tool_name, arguments)
                tool_results.append(
                    {
                        "tool_call_id": tool_call.id,
                        "tool_name": tool_name,
                        "result": result,
                    }
                )

                # Add tool result message
                # Ensure result is JSON serializable
                # OpenAI API expects content to be a JSON string
                serializable_result = _make_json_serializable(result)
                try:
                    # Try to serialize as JSON string
                    content_str = json.dumps(serializable_result, ensure_ascii=False)
                except Exception as e:
                    logger.warning(
                        f"Failed to serialize tool result as JSON: {e}, using string representation"
                    )
                    content_str = str(serializable_result)

                messages.append(
                    {
                        "role": "tool",
                        "tool_call_id": tool_call.id,
                        "content": content_str,
                    }
                )

            # Collect all tool results
            all_tool_results.extend(tool_results)

            # Continue the loop - call OpenAI API again to see if more tools are needed
            logger.info(
                f"Calling OpenAI API for next iteration (iteration {iteration})"
            )
            try:
                response = await call_openai_with_retry(
                    client=client,
                    model=model,
                    messages=messages,
                    tools=openai_tools if openai_tools else None,
                    tool_choice="auto",
                )
                message = response.choices[0].message

                # If no more tool calls, break the loop
                if not message.tool_calls:
                    logger.info(f"No more tool calls after {iteration} iterations")
                    break

            except Exception as e:
                logger.error(
                    f"OpenAI API call failed in iteration {iteration} after retries: {str(e)}",
                    exc_info=True,
                )
                raise

        # After loop completes, get final response if needed
        if message.tool_calls:
            # Still have tool calls but hit max iterations
            logger.warning(
                f"Reached max iterations ({max_iterations}), stopping tool calls"
            )
            # Get final response anyway
            try:
                final_response = await call_openai_with_retry(
                    client=client,
                    model=model,
                    messages=messages,
                    tools=openai_tools if openai_tools else None,
                    tool_choice="none",  # Force no more tool calls
                )
                message = final_response.choices[0].message
            except Exception as e:
                logger.error(
                    f"Failed to get final response after retries: {str(e)}",
                    exc_info=True,
                )

        # Return final response
        return {
            "role": "assistant",
            "content": message.content or "Task completed. Check the files created.",
            "tool_calls": [
                {
                    "id": tc.id,
                    "name": tc.function.name,
                    "arguments": json.loads(tc.function.arguments),
                }
                for tc in (message.tool_calls or [])
            ],
            "tool_results": all_tool_results,
            "iterations": iteration,
        }

    except Exception as e:
        logger.error(f"Error in chat endpoint: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """
    WebSocket endpoint for real-time communication.

    Args:
        websocket: WebSocket connection
    """
    await websocket.accept()
    try:
        while True:
            data = await websocket.receive_json()
            # Handle WebSocket messages
            # For now, just echo back
            await websocket.send_json({"type": "message", "data": data})
    except WebSocketDisconnect:
        logger.info("WebSocket client disconnected")


@app.get("/api/files")
async def list_files_api():
    """List all files in VFS."""
    try:
        await get_environment()
        result = await call_tool("list_files", {})
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/api/files/{file_path:path}")
async def get_file(file_path: str):
    """Get file content."""
    try:
        await get_environment()
        result = await call_tool("read_file", {"file_path": file_path})
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/api/files/{file_path:path}")
async def save_file(file_path: str, content: Dict[str, str]):
    """Save file content."""
    try:
        await get_environment()
        result = await call_tool(
            "write_file",
            {"file_path": file_path, "content": content.get("content", "")},
        )
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
