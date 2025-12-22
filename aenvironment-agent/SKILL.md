---
name: aenvironment-agent
description: Guide for creating, developing, and deploying AI agents using AEnvironment framework. Use when developers need to: (1) Create custom agent environments with MCP protocol tools, (2) Implement agent tools using @register_tool decorator, (3) Build and deploy agent environments to cloud, (4) Develop client applications using AEnvironment SDK to interact with agent environments, (5) Set up complete agent synthesis workflows from initialization to deployment.
---

# AEnvironment Agent Development

Guide for creating, developing, and deploying AI agents using AEnvironment framework.

## Quick Start

### Create Agent Environment

Initialize a new agent environment project:

```bash
cd /path/to/workspace
export ENV_IDR="my-agent-$(openssl rand -hex 4)"
aenv init "$ENV_IDR"
```

This generates:
- `config.json` - Environment configuration
- `Dockerfile` - Container build file
- `requirements.txt` - Python dependencies
- `src/custom_env.py` - Agent tool implementation

### Implement Agent Tools

Register tools using `@register_tool` decorator in `src/custom_env.py`:

```python
from typing import Dict, Any
from aenv import register_tool

@register_tool
async def chat(user_request: str) -> dict:
    """
    Generate HTML code based on user's natural language request.
    
    Args:
        user_request: Natural language description of the HTML page to generate
        
    Returns:
        Dictionary with keys:
        - html_code: The generated HTML code as string
        - status: Success status ("success" or "error")
        - message: Status message or error description
    """
    # Implementation here
    return {
        "html_code": "<!DOCTYPE html>...",
        "status": "success",
        "message": "HTML generated successfully"
    }
```

### Build and Deploy

Build and push environment:

```bash
cd "$ENV_IDR"
aenv build --push -n "$ENV_IDR"
aenv push
```

Verify deployment:

```bash
aenv get "$ENV_IDR" -v 1.0.0
```

## Client Application Development

Use AEnvironment SDK to interact with agent environments:

```python
import asyncio
from aenv import Environment

async def main():
    async with Environment(
        "my-agent@1.0.0",
        environment_variables={
            "OPENAI_API_KEY": "...",
            "OPENAI_BASE_URL": "...",
            "OPENAI_MODEL": "..."
        }
    ) as env:
        # List available tools
        tools = await env.list_tools()
        
        # Call agent tool
        result = await env.call_tool(
            "chat",
            {"user_request": "Create a welcome page"}
        )
        
        # Parse result
        content = result.content
        if isinstance(content, list):
            for item in content:
                if isinstance(item, dict) and item.get("type") == "text":
                    data = json.loads(item.get("text", ""))
                    break
        elif isinstance(content, str):
            data = json.loads(content)
        
        print(f"HTML Code: {data['html_code']}")

asyncio.run(main())
```

## Common Patterns

### LLM Integration

For agent tools that use LLM:

1. Read LLM configuration from environment variables:
```python
import os
openai_api_key = os.getenv("OPENAI_API_KEY")
openai_base_url = os.getenv("OPENAI_BASE_URL")
openai_model = os.getenv("OPENAI_MODEL")
```

2. Pass configuration via `environment_variables` when creating Environment instance

3. Maintain conversation context for multi-turn interactions

### Error Handling

Always return consistent format:

```python
try:
    # Tool logic
    return {
        "html_code": result,
        "status": "success",
        "message": "Operation completed"
    }
except Exception as e:
    return {
        "html_code": "",
        "status": "error",
        "message": str(e)
    }
```

### Dockerfile Configuration

For LLM-based agents, use OpenAI base image:

```dockerfile
FROM reg.antgroup-inc.cn/aenv/openai-base:1.0.0
# ... rest of Dockerfile
```

## Workflow Checklist

### Environment Development
- [ ] Use `aenv init` to create project (never create files manually)
- [ ] Implement tools with `@register_tool` decorator
- [ ] Add dependencies to `requirements.txt` (e.g., `openai>=1.0.0`)
- [ ] Update Dockerfile base image if needed
- [ ] Build and push with `aenv build --push`
- [ ] Verify with `aenv get`

### Client Development
- [ ] Use `Environment` class with `async with` context manager
- [ ] Pass LLM config via `environment_variables`
- [ ] Parse `ToolResult.content` correctly (list or string)
- [ ] Handle errors gracefully
- [ ] Release resources properly

## Advanced Topics

For detailed examples and advanced patterns, see:
- **Complete workflow**: See [references/complete-workflow.md](references/complete-workflow.md) for end-to-end agent synthesis example
- **Tool patterns**: See [references/tool-patterns.md](references/tool-patterns.md) for common tool implementation patterns
- **Client patterns**: See [references/client-patterns.md](references/client-patterns.md) for SDK usage patterns

## Key Principles

1. **Always use `aenv init`** - Never create project structure manually
2. **Consistent return format** - Tools must return dict with `status`, `message`, and result fields
3. **Environment variables** - Pass LLM config via `environment_variables` parameter
4. **Resource cleanup** - Use `async with` context manager for proper cleanup
5. **Error handling** - Return structured error responses, never raise exceptions

