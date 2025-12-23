# Tool Development Guide

## Tool Registration

Use the `@register_tool` decorator to register tools:

```python
from aenv import register_tool
from typing import Dict, Any

@register_tool
async def my_tool(param: str) -> Dict[str, Any]:
    """
    Tool description.
    
    Args:
        param: Parameter description
        
    Returns:
        Dictionary with status and result
    """
    try:
        # Tool logic
        result = process(param)
        return {
            "status": "success",
            "result": result
        }
    except Exception as e:
        return {
            "status": "error",
            "message": str(e)
        }
```

## LLM Integration Pattern

Common pattern for LLM-based tools:

```python
import os
from openai import AsyncOpenAI
from typing import List, Dict

_client: AsyncOpenAI | None = None
_conversation_history: List[Dict[str, str]] = []

def _get_client() -> AsyncOpenAI:
    global _client
    if _client is None:
        api_key = os.getenv("OPENAI_API_KEY")
        base_url = os.getenv("OPENAI_BASE_URL")
        if not api_key:
            raise ValueError("OPENAI_API_KEY not set")
        _client = AsyncOpenAI(
            api_key=api_key,
            base_url=base_url if base_url else None,
        )
    return _client

@register_tool
async def chat(user_request: str) -> Dict[str, Any]:
    global _conversation_history
    
    client = _get_client()
    model = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
    
    messages = [
        {"role": "system", "content": "Your system prompt"},
        *(_conversation_history),
        {"role": "user", "content": user_request}
    ]
    
    response = await client.chat.completions.create(
        model=model,
        messages=messages,
        temperature=0.7,
        max_tokens=2000,
    )
    
    result = response.choices[0].message.content.strip()
    
    # Update history
    _conversation_history.append({"role": "user", "content": user_request})
    _conversation_history.append({"role": "assistant", "content": result})
    if len(_conversation_history) > 20:
        _conversation_history = _conversation_history[-20:]
    
    return {
        "status": "success",
        "result": result
    }
```

## Error Handling

Always return structured error responses:

```python
@register_tool
async def robust_tool(param: str) -> Dict[str, Any]:
    try:
        # Main logic
        result = process(param)
        return {"status": "success", "result": result}
    except ValueError as e:
        return {"status": "error", "message": f"Invalid input: {str(e)}"}
    except Exception as e:
        return {"status": "error", "message": f"Unexpected error: {str(e)}"}
```

## Return Format Standards

Tools should return dictionaries with:
- `status`: "success" or "error"
- `message`: Human-readable status message
- Additional fields as needed for the specific tool

Example success response:
```python
{
    "status": "success",
    "result": "...",
    "message": "Operation completed"
}
```

Example error response:
```python
{
    "status": "error",
    "message": "Error description"
}
```

