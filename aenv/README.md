# AEnv Python SDK

A production-grade Python SDK for managing AI agent tools in containerized environments with MCP protocol support.

## Features

- **Simple Tool Definition**: Create tools with just a decorator
- **MCP Protocol Support**: Full compatibility with Model Context Protocol using FastMCP
- **Containerized Environments**: Run tools in isolated, scalable environments
- **Production Ready**: Built-in error handling, retries, and monitoring
- **Streamable HTTP**: Uses FastMCP's streamable_http protocol for better performance
- **Type Safety**: Full type hints and validation

## Quick Start

### code architecture

aenv/
├── pyproject.toml
├── src/
│   └── cli/
│       ├── **init**.py
│       ├── cmds/
│       │   ├── **init**.py
│       │   └── cli.py
│   └── env/
│           ├── **init**.py
│           └── another_file.py
└── README.md

### Installation

```bash
pip install aenvironment
```

### Creating Tools

Create a Python file with your tools:

```python
# my_tools.py
from aenv import register_tool

@register_tool
def get_weather(location: str, unit: str = "celsius") -> dict:
    """Get weather for a location."""
    return {"location": location, "temperature": 22.5, "unit": unit}

@register_tool
async def search_web(query: str, max_results: int = 10) -> list:
    """Search the web."""
    return [{"title": f"Result for {query}", "url": "https://example.com"}]
```

### Local Development

Start an MCP server with your tools using FastMCP:

```bash
# Start MCP server on http://localhost:8081
python main.py ./my_tools.py

# Start with custom host and port
python main.py ./my_tools.py --host 0.0.0.0 --port 8080

# Start with tools from directory
python main.py ./tools/

# Start with custom server name
python main.py ./tools/ --name my-server
```

### Using Environments

```python
import asyncio
import Environment


async def main():
    # Create environment
    async with Environment("my-env") as environment:
        # List available tools
        tools = environment.list_tools()
        print("Available tools:", tools)

        # Call a tool
        result = await environment.call_tool(
            "my-env/get_weather",
            {"location": "Beijing", "unit": "celsius"}
        )
        print("Weather:", result.content)


asyncio.run(main())
```

## Architecture

### Core Concepts

- **Tool**: The smallest executable unit with input/output schemas
- **Environment**: Containerized runtime for tools with lifecycle management
- **Registry**: Global tool discovery and management

### Directory Structure

```bash
aenv/
├── core/           # Core SDK components
├── server/         # FastMCP server implementation
├── client/         # API client for backend
└── examples/       # Usage examples
```

## Tool Definition

Tools are defined using the `@register_tool` decorator:

```python
from aenv import register_tool
from typing import Dict, Any

@register_tool
def my_tool(
    param1: str,
    param2: int = 42,
    param3: bool = False
) -> Dict[str, Any]:
    """
    Tool description for LLMs.

    Args:
        param1: Description of param1
        param2: Description of param2 (default: 42)
        param3: Description of param3

    Returns:
        Dictionary with results
    """
    return {"result": f"Processed {param1} with {param2}"}
```

## Environment Management

### Creating Environments

```python
from aenv import Environment

# Create environment
env = Environment("my-env", scheduler_url="http://localhost:8080")
await env.initialize()

# Use as context manager
async with Environment("my-env") as env:
    tools = env.list_tools()
    result = await env.call_tool("tool_name", {"arg": "value"})
```

### Configuration

Environment variables:

- `AENV_SCHEDULER_URL`: Default scheduler URL
- `AENV_API_KEY`: API key for authentication
- `AENV_TIMEOUT`: Default timeout in seconds

## Error Handling

The SDK provides comprehensive error handling:

```python
from env.core import (
    ToolError, ToolTimeoutError, EnvironmentError, NetworkError
)

try:
    result = await env.call_tool("tool_name", {"arg": "value"})
except ToolTimeoutError as e:
    print(f"Tool timed out: {e.timeout}s")
except ToolError as e:
    print(f"Tool failed: {e.message}")
except EnvironmentError as e:
    print(f"Environment error: {e.message}")
```

## Testing

Run tests:

```bash
pip install -e ".[dev]"
pytest tests/
```

## Examples

See the `examples/` directory for complete examples:

- `weather_tools.py`: Weather-related tools
- `search_tools.py`: Web search and analysis tools

## Development

### Setup

```bash
git clone https://github.com/inclusionAI/AEnvironment
cd AEnvironment
pip install -e ".[dev]"
```

### Running Tests

```bash
pytest tests/ -v
```

### Code Quality

```bash
black aenv/
isort aenv/
mypy aenv/
```

## License

MIT License - see LICENSE file for details.
