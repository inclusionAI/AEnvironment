#!/usr/bin/env python3
"""
Helper script to parse AEnvironment ToolResult content.
Can be used as a reference or imported in client applications.
"""

import json
from typing import Any, Dict


def parse_tool_result(result: Any) -> Dict[str, Any]:
    """
    Parse ToolResult content into structured data.
    
    Args:
        result: ToolResult object with content attribute
        
    Returns:
        Parsed dictionary data
        
    Raises:
        ValueError: If content cannot be parsed
    """
    content = result.content if hasattr(result, 'content') else result
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


if __name__ == "__main__":
    # Example usage
    class MockResult:
        def __init__(self, content):
            self.content = content
    
    # Test cases
    test_cases = [
        MockResult({"status": "success", "message": "OK"}),
        MockResult('{"status": "success", "message": "OK"}'),
        MockResult([{"type": "text", "text": '{"status": "success"}'}]),
    ]
    
    for i, test in enumerate(test_cases):
        try:
            result = parse_tool_result(test)
            print(f"Test {i+1}: ✅ {result}")
        except Exception as e:
            print(f"Test {i+1}: ❌ {e}")

