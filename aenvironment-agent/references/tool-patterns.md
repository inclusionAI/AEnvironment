# Agent Tool Implementation Patterns

Common patterns for implementing agent tools in AEnvironment.

## Basic Tool Registration

```python
from typing import Dict, Any
from aenv import register_tool

@register_tool
def simple_tool(param: str) -> Dict[str, Any]:
    """
    Tool description.
    
    Args:
        param: Parameter description
        
    Returns:
        Result dictionary
    """
    return {"result": f"Processed: {param}"}
```

## Async Tool Pattern

```python
@register_tool
async def async_tool(param: str) -> dict:
    """Async tool implementation."""
    # Async operations here
    await some_async_operation()
    return {"status": "success"}
```

## LLM Integration Pattern

### Pattern 1: Module-level Client

```python
import os
from openai import AsyncOpenAI
from aenv import register_tool

_client = None

def get_client():
    global _client
    if _client is None:
        _client = AsyncOpenAI(
            api_key=os.getenv("OPENAI_API_KEY"),
            base_url=os.getenv("OPENAI_BASE_URL")
        )
    return _client

@register_tool
async def llm_tool(prompt: str) -> dict:
    client = get_client()
    response = await client.chat.completions.create(
        model=os.getenv("OPENAI_MODEL", "gpt-4"),
        messages=[{"role": "user", "content": prompt}]
    )
    return {
        "result": response.choices[0].message.content,
        "status": "success"
    }
```

### Pattern 2: Context Management

```python
from contextlib import asynccontextmanager

@asynccontextmanager
async def get_llm_client():
    client = AsyncOpenAI(
        api_key=os.getenv("OPENAI_API_KEY"),
        base_url=os.getenv("OPENAI_BASE_URL")
    )
    try:
        yield client
    finally:
        await client.close()

@register_tool
async def llm_tool(prompt: str) -> dict:
    async with get_llm_client() as client:
        response = await client.chat.completions.create(...)
        return {"result": response.choices[0].message.content}
```

## Error Handling Pattern

```python
@register_tool
async def robust_tool(param: str) -> dict:
    """
    Tool with comprehensive error handling.
    
    Returns:
        Always returns dict with 'status' and 'message' keys
    """
    try:
        # Tool logic
        result = perform_operation(param)
        return {
            "result": result,
            "status": "success",
            "message": "Operation completed successfully"
        }
    except ValueError as e:
        return {
            "result": None,
            "status": "error",
            "message": f"Invalid input: {str(e)}"
        }
    except Exception as e:
        return {
            "result": None,
            "status": "error",
            "message": f"Unexpected error: {str(e)}"
        }
```

## Multi-turn Conversation Pattern

```python
from collections import defaultdict

_conversations = defaultdict(list)

@register_tool
async def chat_with_context(user_message: str, session_id: str = "default") -> dict:
    """
    Chat tool with conversation context.
    
    Args:
        user_message: User's message
        session_id: Session identifier for context management
    """
    # Get conversation history
    messages = _conversations[session_id]
    
    # Add user message
    messages.append({"role": "user", "content": user_message})
    
    # Call LLM
    client = get_client()
    response = await client.chat.completions.create(
        model=os.getenv("OPENAI_MODEL"),
        messages=messages
    )
    
    # Add assistant response
    assistant_message = response.choices[0].message.content
    messages.append({"role": "assistant", "content": assistant_message})
    
    # Keep last N messages (optional)
    if len(messages) > 20:
        _conversations[session_id] = messages[-20:]
    
    return {
        "response": assistant_message,
        "status": "success",
        "session_id": session_id
    }
```

## File Processing Pattern

```python
from pathlib import Path

@register_tool
async def process_file(file_path: str) -> dict:
    """
    Process file and return results.
    
    Args:
        file_path: Path to file to process
    """
    try:
        path = Path(file_path)
        if not path.exists():
            return {
                "status": "error",
                "message": f"File not found: {file_path}"
            }
        
        # Process file
        content = path.read_text()
        result = process_content(content)
        
        return {
            "result": result,
            "status": "success",
            "file_path": file_path
        }
    except Exception as e:
        return {
            "status": "error",
            "message": str(e)
        }
```

## Structured Output Pattern

```python
from typing import List
from pydantic import BaseModel

class Item(BaseModel):
    name: str
    value: float

@register_tool
async def structured_output(query: str) -> dict:
    """
    Return structured data.
    
    Returns:
        Dict with structured data following schema
    """
    # Generate structured result
    items = [
        {"name": "item1", "value": 10.5},
        {"name": "item2", "value": 20.3}
    ]
    
    return {
        "items": items,
        "count": len(items),
        "status": "success"
    }
```

## Validation Pattern

```python
def validate_input(param: str) -> tuple[bool, str]:
    """Validate input parameter."""
    if not param:
        return False, "Parameter cannot be empty"
    if len(param) > 1000:
        return False, "Parameter too long (max 1000 chars)"
    return True, ""

@register_tool
async def validated_tool(param: str) -> dict:
    """Tool with input validation."""
    is_valid, error_msg = validate_input(param)
    if not is_valid:
        return {
            "status": "error",
            "message": error_msg
        }
    
    # Process valid input
    result = process(param)
    return {
        "result": result,
        "status": "success"
    }
```

## Best Practices

1. **Always return consistent format**: Dict with `status` and `message` keys
2. **Handle all exceptions**: Never let exceptions propagate
3. **Use type hints**: Improve code quality and IDE support
4. **Document parameters**: Clear docstrings help users understand tool usage
5. **Async for I/O**: Use async/await for network calls and file operations
6. **Resource management**: Clean up resources properly (connections, files, etc.)

