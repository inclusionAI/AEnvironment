# Client Development Guide

Advanced patterns for developing client applications. For a complete working template, see [scripts/client_template.py](../scripts/client_template.py).

## Tool Result Parsing

Tool results may be returned in different formats. Use this parser to handle all cases:

```python
def parse_tool_result(result) -> Dict[str, Any]:
    """Parse tool result that may be in different formats"""
    content_text = None
    
    # Handle list format
    if isinstance(result.content, list):
        for item in result.content:
            if isinstance(item, dict) and item.get("type") == "text":
                content_text = item.get("text", "")
                break
    # Handle string format
    elif isinstance(result.content, str):
        content_text = result.content
    # Handle dict format
    else:
        if isinstance(result.content, dict):
            return result.content
        raise ValueError(f"Cannot parse result content: {type(result.content)}")
    
    # Try to parse as JSON
    if content_text:
        try:
            return json.loads(content_text)
        except json.JSONDecodeError:
            # If not JSON, return as plain content
            return {
                "content": content_text,
                "status": "success"
            }
    
    raise ValueError("Cannot parse result content")
```

## Advanced Patterns

### Interactive Tool Loop

```python
async with Environment(env_name, environment_variables=env_config) as env:
    print("Enter requests (type 'quit' to exit):")
    
    while True:
        try:
            user_input = input("\nRequest: ").strip()
            
            if not user_input or user_input.lower() in ["quit", "exit", "q"]:
                break
            
            result = await env.call_tool("chat", {"user_request": user_input})
            response = parse_tool_result(result)
            
            if response.get("status") == "error":
                print(f"Error: {response.get('message')}")
            else:
                print(f"Result: {response.get('result')}")
                
        except KeyboardInterrupt:
            print("\nExiting...")
            break
        except Exception as e:
            print(f"Error: {e}")
            continue
```

## Error Handling

Always handle errors appropriately:

```python
try:
    async with Environment(env_name, environment_variables=env_config) as env:
        result = await env.call_tool("tool_name", {"param": "value"})
        # Process result
except ValueError as e:
    # Configuration errors
    print(f"Configuration error: {e}")
    sys.exit(1)
except Exception as e:
    # Other errors (API failures, timeouts, etc.)
    print(f"Error: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)
```

## Complete Example

See [scripts/client_template.py](../scripts/client_template.py) for a complete, production-ready client template.

