# Client Application Patterns

Common patterns for using AEnvironment SDK in client applications.

## Basic Usage

```python
import asyncio
from aenv import Environment

async def basic_example():
    env = Environment("my-env@1.0.0")
    await env.initialize()
    
    tools = await env.list_tools()
    result = await env.call_tool("tool_name", {"param": "value"})
    
    await env.release()
```

## Context Manager Pattern (Recommended)

```python
async def context_manager_example():
    async with Environment("my-env@1.0.0") as env:
        tools = await env.list_tools()
        result = await env.call_tool("tool_name", {"param": "value"})
        # Automatic cleanup on exit
```

## Environment Variables

```python
async def with_env_vars():
    async with Environment(
        "my-env@1.0.0",
        environment_variables={
            "OPENAI_API_KEY": "...",
            "OPENAI_BASE_URL": "...",
            "OPENAI_MODEL": "gpt-4"
        }
    ) as env:
        # Environment variables available to agent tools
        result = await env.call_tool("chat", {"message": "Hello"})
```

## Result Parsing

### Pattern 1: List Content

```python
result = await env.call_tool("tool_name", {"param": "value"})
content = result.content

if isinstance(content, list):
    for item in content:
        if isinstance(item, dict) and item.get("type") == "text":
            text_content = item.get("text", "")
            # Process text_content
            break
```

### Pattern 2: String Content

```python
result = await env.call_tool("tool_name", {"param": "value"})
content = result.content

if isinstance(content, str):
    # Process string content directly
    data = json.loads(content)
```

### Pattern 3: Dict Content

```python
result = await env.call_tool("tool_name", {"param": "value"})
content = result.content

if isinstance(content, dict):
    # Process dict directly
    status = content.get("status")
    result_data = content.get("result")
```

### Complete Parsing Pattern

```python
import json

def parse_tool_result(result):
    """Parse ToolResult content into structured data."""
    content = result.content
    response_data = None
    
    if isinstance(content, list):
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                text = item.get("text", "")
                if text:
                    try:
                        response_data = json.loads(text)
                        break
                    except json.JSONDecodeError:
                        response_data = {"raw": text}
                        break
    elif isinstance(content, str):
        try:
            response_data = json.loads(content)
        except json.JSONDecodeError:
            response_data = {"raw": content}
    elif isinstance(content, dict):
        response_data = content
    else:
        raise ValueError(f"Cannot parse content type: {type(content)}")
    
    return response_data

# Usage
result = await env.call_tool("chat", {"user_request": "..."})
data = parse_tool_result(result)
if data.get("status") == "success":
    html_code = data.get("html_code")
```

## Error Handling

```python
async def robust_client():
    try:
        async with Environment("my-env@1.0.0", timeout=120) as env:
            result = await env.call_tool("tool_name", {"param": "value"})
            # Process result
    except TimeoutError:
        print("Environment initialization timeout")
    except Exception as e:
        print(f"Error: {e}")
```

## Interactive Client Pattern

```python
async def interactive_client():
    async with Environment("my-env@1.0.0") as env:
        # List available tools
        tools = await env.list_tools()
        print("Available tools:")
        for tool in tools:
            print(f"  - {tool['name']}: {tool.get('description', '')}")
        
        # Interactive loop
        while True:
            user_input = input("\nEnter request (or 'quit'): ")
            if user_input.lower() == 'quit':
                break
            
            try:
                result = await env.call_tool("chat", {"user_request": user_input})
                data = parse_tool_result(result)
                
                if data.get("status") == "success":
                    print(f"✅ Success: {data.get('message', '')}")
                    # Process success result
                else:
                    print(f"❌ Error: {data.get('message', 'Unknown error')}")
            except Exception as e:
                print(f"❌ Exception: {e}")
```

## File Output Pattern

```python
from pathlib import Path

async def save_result_to_file():
    async with Environment("my-env@1.0.0") as env:
        result = await env.call_tool("generate_html", {"request": "..."})
        data = parse_tool_result(result)
        
        if data.get("status") == "success":
            html_code = data.get("html_code", "")
            output_file = Path("output.html")
            output_file.write_text(html_code, encoding="utf-8")
            print(f"Saved to: {output_file.absolute()}")
```

## Browser Display Pattern

```python
import subprocess
from pathlib import Path

async def display_in_browser():
    async with Environment("my-env@1.0.0") as env:
        result = await env.call_tool("generate_html", {"request": "..."})
        data = parse_tool_result(result)
        
        if data.get("status") == "success":
            html_code = data.get("html_code", "")
            output_file = Path("output.html")
            output_file.write_text(html_code, encoding="utf-8")
            
            # Open in browser
            try:
                subprocess.run(["open", "-a", "Google Chrome", str(output_file)])
            except:
                print("Could not open browser automatically")
```

## Configuration from Environment

```python
import os

async def config_from_env():
    # Read configuration from environment variables
    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4")
    
    if not api_key:
        raise ValueError("OPENAI_API_KEY not set")
    
    async with Environment(
        "my-env@1.0.0",
        environment_variables={
            "OPENAI_API_KEY": api_key,
            "OPENAI_BASE_URL": base_url,
            "OPENAI_MODEL": model
        }
    ) as env:
        # Use environment
        result = await env.call_tool("chat", {"message": "Hello"})
```

## Multiple Tool Calls

```python
async def multiple_tools():
    async with Environment("my-env@1.0.0") as env:
        # Call multiple tools in sequence
        result1 = await env.call_tool("tool1", {"param": "value1"})
        result2 = await env.call_tool("tool2", {"param": "value2"})
        
        # Process results
        data1 = parse_tool_result(result1)
        data2 = parse_tool_result(result2)
```

## Timeout Configuration

```python
async def with_timeout():
    # Set timeout for environment operations (in seconds)
    async with Environment("my-env@1.0.0", timeout=120) as env:
        # Long-running operations will timeout after 120 seconds
        result = await env.call_tool("slow_tool", {"param": "value"})
```

## Best Practices

1. **Always use context manager**: `async with Environment(...)` ensures proper cleanup
2. **Set appropriate timeout**: Long-running operations need longer timeouts
3. **Parse results consistently**: Use helper function for result parsing
4. **Handle errors gracefully**: Catch and handle exceptions appropriately
5. **Pass environment variables**: Use `environment_variables` parameter for configuration
6. **Validate inputs**: Check required environment variables before creating Environment

