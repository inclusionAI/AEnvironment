# Complete Agent Synthesis Workflow

End-to-end guide for creating a complete AEnvironment agent demo with HTML generation capability.

## Overview

This workflow creates:
1. **Environment project**: SWE Agent exposing `chat` tool via MCP protocol
2. **Client application**: Uses AEnvironment SDK to launch environment and call `chat` tool, displaying generated HTML

## Step 1: Initialize Environment Project

**Critical**: Must use `aenv init` script, never create files manually.

```bash
cat > init_env.sh <<'EOF'
#!/usr/bin/env bash
set -e
cd /path/to/workspace
mkdir -p temp
cd temp
export ENV_IDR="html-agent-$(openssl rand -hex 4)"
aenv init "$ENV_IDR"
EOF

chmod +x init_env.sh
./init_env.sh
```

Generated structure:
- `config.json` - AEnvironment configuration
- `Dockerfile` - Docker image build file
- `requirements.txt` - Python dependencies
- `src/custom_env.py` - Agent tool implementation

## Step 2: Implement Chat Tool

Edit `src/custom_env.py` to implement `chat` tool:

### Tool Signature

```python
from typing import Dict, Any
from aenv import register_tool

@register_tool
async def chat(user_request: str) -> dict:
    """
    Generate HTML code based on user's natural language request.
    
    This tool receives a user's request and uses LLM to generate 
    corresponding HTML code that fulfills the request.
    
    Args:
        user_request: Natural language description of the HTML page to generate
        
    Returns:
        Dictionary with keys:
        - html_code: The generated HTML code as string
        - status: Success status ("success" or "error")
        - message: Status message or error description
    """
```

### LLM Integration

1. Use OpenAI SDK for global lifecycle SWE agent
2. Build appropriate system prompt to guide HTML generation
3. Call LLM and receive generated HTML code
4. Handle errors (API failures, timeouts)
5. Maintain multi-turn conversation context

### Implementation Pattern

```python
import os
import json
from openai import AsyncOpenAI
from aenv import register_tool

# Initialize OpenAI client (can be module-level)
client = None

def get_client():
    global client
    if client is None:
        client = AsyncOpenAI(
            api_key=os.getenv("OPENAI_API_KEY"),
            base_url=os.getenv("OPENAI_BASE_URL")
        )
    return client

@register_tool
async def chat(user_request: str) -> dict:
    try:
        client = get_client()
        model = os.getenv("OPENAI_MODEL", "gpt-4")
        
        system_prompt = """You are an expert HTML developer. 
        Generate clean, modern HTML code based on user requests.
        Return only valid HTML code without markdown formatting."""
        
        response = await client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_request}
            ]
        )
        
        html_code = response.choices[0].message.content.strip()
        # Remove markdown code blocks if present
        if html_code.startswith("```"):
            html_code = html_code.split("```")[1]
            if html_code.startswith("html"):
                html_code = html_code[4:]
            html_code = html_code.strip()
        
        return {
            "html_code": html_code,
            "status": "success",
            "message": "HTML generated successfully"
        }
    except Exception as e:
        return {
            "html_code": "",
            "status": "error",
            "message": f"Error generating HTML: {str(e)}"
        }
```

### File Modifications

1. **requirements.txt**: Add LLM client library
   ```
   aenvironment>=0.1.0
   openai>=1.0.0
   ```

2. **Dockerfile**: Update base image
   ```dockerfile
   FROM reg.antgroup-inc.cn/aenv/openai-base:1.0.0
   # ... rest of Dockerfile
   ```

## Step 3: Build and Deploy

Execute build script automatically:

```bash
cat > build_env.sh <<'EOF'
#!/usr/bin/env bash
set -e
cd temp/"$ENV_IDR"
# Build and push
aenv build --push -n "$ENV_IDR"
aenv push
# Pull and verify
aenv get "$ENV_IDR" -v 1.0.0
aenv push
EOF

chmod +x build_env.sh
./build_env.sh
```

**Requirements**:
- Execute scripts completely without modification
- Report errors and stop on failure
- No manual user intervention needed

## Step 4: Client Application

### File Structure

```
client/
├── client.py          # Main client code
├── requirements.txt   # Client dependencies (aenvironment>=0.1.0)
```

### Client Implementation

```python
import asyncio
import json
import os
import subprocess
from pathlib import Path
from aenv import Environment

async def main():
    # Get LLM configuration
    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4")
    
    if not api_key:
        print("Error: OPENAI_API_KEY not set")
        return
    
    env_name = "html-agent@1.0.0"  # Use your deployed environment name
    
    print("Initializing Environment...")
    async with Environment(
        env_name,
        environment_variables={
            "OPENAI_API_KEY": api_key,
            "OPENAI_BASE_URL": base_url,
            "OPENAI_MODEL": model
        },
        timeout=120
    ) as env:
        # List available tools
        tools = await env.list_tools()
        print("Environment ready. Available tools:")
        for tool in tools:
            print(f"  - {tool['name']}: {tool.get('description', '')}")
        
        # Interactive loop
        while True:
            user_request = input("\nEnter HTML generation request (or 'quit' to exit): ")
            if user_request.lower() == 'quit':
                break
            
            print("Generating HTML...")
            try:
                result = await env.call_tool("chat", {"user_request": user_request})
                
                # Parse result
                content = result.content
                response_data = None
                
                if isinstance(content, list):
                    for item in content:
                        if isinstance(item, dict) and item.get("type") == "text":
                            content_text = item.get("text", "")
                            if content_text:
                                response_data = json.loads(content_text)
                                break
                elif isinstance(content, str):
                    response_data = json.loads(content)
                else:
                    response_data = content
                
                if not isinstance(response_data, dict):
                    raise ValueError(f"Cannot parse response: {type(content)}")
                
                if response_data.get("status") == "success":
                    html_code = response_data.get("html_code", "")
                    output_file = Path("output.html")
                    output_file.write_text(html_code, encoding="utf-8")
                    print(f"✅ HTML generated and saved to: {output_file.absolute()}")
                    
                    # Open in browser
                    try:
                        subprocess.run(["open", "-a", "Google Chrome", str(output_file)])
                    except:
                        print("Note: Could not open browser automatically")
                else:
                    print(f"❌ Error: {response_data.get('message', 'Unknown error')}")
                    
            except Exception as e:
                print(f"❌ Error calling tool: {e}")

if __name__ == "__main__":
    asyncio.run(main())
```

### Client Requirements

```txt
aenvironment>=0.1.0
```

## Testing Scenarios

### Basic HTML Generation
- Input: "Create a simple welcome page with title 'Welcome' and introduction text"
- Expected: HTML with title and paragraph

### Complex HTML Generation
- Input: "Create a todo list page with form input and add button"
- Expected: HTML with interactive elements

### Error Handling
- Invalid API key should return friendly error message
- Network errors should be caught and handled

## Implementation Checklist

### Environment Side
- [ ] Use `aenv init` to create project (never create files manually)
- [ ] Register `chat` tool with `@register_tool`
- [ ] `chat` tool receives `user_request` and returns dict with `html_code`, `status`, `message`
- [ ] Read `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL` from environment variables
- [ ] Use OpenAI-compatible API to call LLM for HTML generation
- [ ] Implement comprehensive error handling
- [ ] `requirements.txt` includes `openai>=1.0.0`
- [ ] Build and push successfully with `build_env.sh`, verify new image locally

### Client Side
- [ ] Use `Environment` class to create environment instance
- [ ] Pass environment variables correctly (`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`)
- [ ] Successfully call `chat` tool and parse return result
- [ ] Save HTML code to file
- [ ] Provide user-friendly interactive interface
- [ ] Open generated HTML in Chrome browser

## Key Notes

1. **Must use `aenv init`**: Never create project structure manually
2. **Environment variable passing**: Pass LLM config via `environment_variables` parameter
3. **Return format**: `chat` tool must return dict with `html_code`, `status`, `message` fields
4. **Automatic build**: Execute `build_env.sh` automatically after code development
5. **Resource cleanup**: Use `async with` context manager for proper resource release
6. **No script modification**: Never modify `init_env.sh` and `build_env.sh` content, only execute

