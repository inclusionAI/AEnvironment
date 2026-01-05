---
name: aenvironment-agent
description: Comprehensive guide for creating, developing, and deploying AI agent environments using AEnvironment framework. Use when developers need to: (1) Initialize and configure agent environments with proper structure, (2) Implement agent tools/functions/rewards using decorators (@register_tool, @register_function, @register_reward), (3) Build, deploy and verify agent environments, (4) Develop robust client applications using AEnvironment SDK, (5) Debug and troubleshoot environment issues. This skill provides automation scripts, best practices, and complete workflow guidance.
---

# AEnvironment Agent Development Guide

Comprehensive guide for building production-ready AI agent environments using the AEnvironment framework. This skill provides step-by-step workflows, automation scripts, and best practices for stable and efficient agent development.

## Prerequisites

**IMPORTANT**: Before using this skill, ensure all prerequisites are installed and configured.

### Required Software

1. **Python 3.10 or later** (3.12+ recommended)
2. **Docker** (installed and running)
3. **AEnvironment SDK** with CLI tools
4. **pip** (Python package manager)

### Installation Steps

#### 1. Install Python 3.10+

Check your Python version:
```bash
python3 --version  # Should show 3.10 or later
```

If needed, download from [python.org](https://www.python.org/downloads/)

#### 2. Install and Verify Docker

```bash
# Check Docker is installed
docker --version

# Verify Docker daemon is running
docker ps  # Should not show connection errors

# If not running, start Docker Desktop (macOS/Windows) or:
# Linux: sudo systemctl start docker
```

#### 3. Install AEnvironment SDK

**Option A: From Source** (recommended for development)
```bash
git clone https://github.com/inclusionAI/AEnvironment
cd AEnvironment/aenv
pip install -e .
```

**Option B: From PyPI** (when published)
```bash
pip install aenvironment
```

#### 4. Verify Installation

```bash
# Check aenv command is available
aenv --help

# Expected output:
# Commands:
#   init     Initialize aenv project
#   build    Build Docker images
#   push     Push to environment hub
#   get      Get environment details
#   test     Test locally
#   ...
```

### Quick Prerequisites Check

Run the automated checker to verify all dependencies:

```bash
bash .claude/skills/aenvironment-agent/scripts/check_prerequisites.sh
```

**Expected output if all OK:**
```
✅ Python 3.12.4 (>= 3.10 required)
✅ pip installed
✅ aenv command found
✅ Docker installed
✅ Docker daemon is running
✅ All required prerequisites are met! ✨
```

**If prerequisites are missing**, the script will provide installation instructions.

### Common Setup Issues

| Issue | Solution |
|-------|----------|
| `aenv: command not found` | Install AEnvironment SDK (see step 3 above) |
| `Docker daemon is not running` | Start Docker Desktop or `sudo systemctl start docker` |
| `Python version too old` | Upgrade to Python 3.10+ from python.org |
| `permission denied` on scripts | Run `chmod +x .claude/skills/aenvironment-agent/scripts/*.sh` |

### Backend Configuration

**CRITICAL**: Configure the AEnvironment Hub backend URL before using any commands:

```bash
# Set the Hub backend URL (required for all operations)
export HUB_BACKEND=http://your-hub-address:8080

# For client SDK, also set (automatically used by Environment class):
export AENV_SYSTEM_URL=http://your-hub-address:8080
```

Add to your `~/.bashrc` or `~/.zshrc` for persistence:
```bash
echo 'export HUB_BACKEND=http://your-hub-address:8080' >> ~/.bashrc
echo 'export AENV_SYSTEM_URL=http://your-hub-address:8080' >> ~/.bashrc
```

**Why both variables?**
- `HUB_BACKEND`: Used by `aenv` CLI commands (init, build, push, get, list)
- `AENV_SYSTEM_URL`: Used by `Environment` SDK class for runtime connections

---

## Core Concepts

**AEnvironment** treats "Everything as Environment" - from simple tools to complex agent systems. Key abstractions:

- **Tool** (`@register_tool`): Callable functions exposed via MCP protocol for external use
- **Function** (`@register_function`): Internal helper functions for environment logic
- **Reward** (`@register_reward`): Evaluation functions for RL training scenarios
- **Environment**: Unified interface for deploying and accessing agent capabilities

## Quick Start Workflow

### 1. Initialize Agent Environment

**CRITICAL**: Always use the initialization script - manual creation is not supported.

```bash
# Run from your project root directory
cd /path/to/your/project

# Execute initialization script
bash .claude/skills/aenvironment-agent/scripts/init_env.sh

# Or with custom parameters:
# bash .claude/skills/aenvironment-agent/scripts/init_env.sh /custom/path my-agent-prefix
```

This generates a complete project structure:
- `config.json` - Environment metadata and configuration
- `Dockerfile` - Container build specification
- `requirements.txt` - Python dependencies (includes aenvironment)
- `src/custom_env.py` - Tool implementation entry point

**Output**: `temp/agent-XXXX/` directory with all required files

### 2. Implement Agent Logic

Edit `src/custom_env.py` to implement your agent's capabilities using decorators.

**IMPORTANT**: For LLM-powered agents with reasoning capabilities, reference the complete template at [`.claude/skills/aenvironment-agent/assets/example_custom_env.py`](assets/example_custom_env.py). This includes:
- ✅ LLM client initialization with lazy loading
- ✅ Multi-turn conversation support with session management
- ✅ Proper error handling for API calls
- ✅ Memory management for conversation history
- ✅ Multiple tool examples (chat, reset, session info)

#### Complete LLM Agent Template

**Copy this template to your `src/custom_env.py`:**

```python
import os
import logging
from typing import Dict, Any, Optional
from collections import defaultdict
from openai import AsyncOpenAI
from aenv import register_tool

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Module-level client (singleton pattern for efficiency)
_client: Optional[AsyncOpenAI] = None

# Conversation history storage (for multi-turn conversations)
_conversations = defaultdict(list)


def get_llm_client() -> AsyncOpenAI:
    """
    Get or create OpenAI client instance (lazy initialization).

    Reads configuration from environment variables:
    - OPENAI_API_KEY: Required API key
    - OPENAI_BASE_URL: Optional custom base URL

    Returns:
        AsyncOpenAI client instance
    """
    global _client
    if _client is None:
        api_key = os.getenv("OPENAI_API_KEY")
        base_url = os.getenv("OPENAI_BASE_URL")

        if not api_key:
            raise ValueError("OPENAI_API_KEY environment variable not set")

        _client = AsyncOpenAI(
            api_key=api_key,
            base_url=base_url
        )
        logger.info("Initialized OpenAI client")

    return _client


@register_tool
async def chat(user_request: str, session_id: str = "default") -> Dict[str, Any]:
    """
    Process user requests using LLM and generate responses.

    This tool supports multi-turn conversations by maintaining session history.
    Each session maintains separate conversation context.

    Args:
        user_request: User's input message or request
        session_id: Session identifier for maintaining conversation context

    Returns:
        Dictionary with standardized format:
        - response: LLM-generated response text
        - status: "success" or "error"
        - message: Human-readable status message
        - session_id: Session identifier used
    """
    try:
        # Get LLM client
        client = get_llm_client()
        model = os.getenv("OPENAI_MODEL", "gpt-4")

        # Get conversation history for this session
        messages = _conversations[session_id]

        # Add system message if this is first message in session
        if not messages:
            system_prompt = """You are a helpful AI assistant.
            Provide clear, accurate, and helpful responses to user questions.
            Be concise but thorough in your explanations."""
            messages.append({"role": "system", "content": system_prompt})

        # Add user message
        messages.append({"role": "user", "content": user_request})

        # Call LLM
        logger.info(f"Calling LLM with model: {model}")
        response = await client.chat.completions.create(
            model=model,
            messages=messages,
            temperature=0.7,
            max_tokens=2000
        )

        # Extract response
        assistant_message = response.choices[0].message.content.strip()

        # Store assistant response in history
        messages.append({"role": "assistant", "content": assistant_message})

        # Keep last 20 messages to manage memory (10 exchanges)
        if len(messages) > 21:  # 1 system + 20 conversation messages
            # Keep system message and last 20 messages
            _conversations[session_id] = [messages[0]] + messages[-20:]

        logger.info(f"Successfully generated response for session: {session_id}")

        return {
            "response": assistant_message,
            "status": "success",
            "message": "Response generated successfully",
            "session_id": session_id
        }

    except ValueError as e:
        logger.error(f"Configuration error: {e}")
        return {
            "response": "",
            "status": "error",
            "message": f"Configuration error: {str(e)}",
            "session_id": session_id
        }

    except Exception as e:
        logger.exception("Unexpected error in chat tool")
        return {
            "response": "",
            "status": "error",
            "message": f"Error generating response: {str(e)}",
            "session_id": session_id
        }


@register_tool
def reset_conversation(session_id: str = "default") -> Dict[str, Any]:
    """
    Reset conversation history for a specific session.

    Args:
        session_id: Session identifier to reset

    Returns:
        Dictionary with status information
    """
    try:
        if session_id in _conversations:
            del _conversations[session_id]
            logger.info(f"Reset conversation for session: {session_id}")
            message = f"Conversation history cleared for session: {session_id}"
        else:
            message = f"No conversation history found for session: {session_id}"

        return {
            "status": "success",
            "message": message,
            "session_id": session_id
        }

    except Exception as e:
        logger.exception("Error resetting conversation")
        return {
            "status": "error",
            "message": f"Error resetting conversation: {str(e)}",
            "session_id": session_id
        }
```

**See the complete example with additional tools at**: [`.claude/skills/aenvironment-agent/assets/example_custom_env.py`](assets/example_custom_env.py)

#### Key Implementation Points

- ✅ **Always return consistent dict format** with `status` and `message` keys
- ✅ **Use `async def`** for LLM calls and I/O-bound operations
- ✅ **Handle all exceptions** - never let them propagate to framework
- ✅ **Read configuration from environment variables** (OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL)
- ✅ **Lazy client initialization** - create client only when first needed
- ✅ **Session management** - maintain separate conversation history per session
- ✅ **Memory management** - limit conversation history to prevent memory issues
- ✅ **Comprehensive logging** - log all operations for debugging

### 3. Configure Dependencies

Update files based on your requirements:

**requirements.txt** - Add necessary dependencies:
```txt
aenvironment>=0.1.0
openai>=1.0.0          # For OpenAI integration
anthropic>=0.18.0      # For Claude integration
# Add other dependencies as needed
```

**Dockerfile** - Update base image if needed:
```dockerfile
# For LLM-based agents
FROM reg.antgroup-inc.cn/aenv/openai-base:1.0.0

# For general agents
FROM python:3.12-slim
```

**config.json** - Usually no changes needed (default config is sufficient)

### 3a. Configure Environment Variables (For LLM-Based Agents)

If your agent uses LLM services (OpenAI, Claude, etc.), you need to configure API keys and endpoints.

#### For Local Testing

Set environment variables before running tests:

```bash
# OpenAI Configuration
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # Optional
export OPENAI_MODEL="gpt-4"  # Optional, can set default in code

# Anthropic/Claude Configuration
export ANTHROPIC_API_KEY="your-api-key-here"

# Test locally
cd temp/agent-XXXX
aenv test
```

#### For Deployed Agents

Pass environment variables via the client `Environment` constructor:

```python
import os
from aenv import Environment

# Read from your local environment
api_key = os.getenv("OPENAI_API_KEY")
base_url = os.getenv("OPENAI_BASE_URL")
model = os.getenv("OPENAI_MODEL", "gpt-4")

# Pass to deployed agent
async with Environment(
    "agent-name@1.0.0",
    environment_variables={
        "OPENAI_API_KEY": api_key,
        "OPENAI_BASE_URL": base_url,
        "OPENAI_MODEL": model
    },
    timeout=120
) as env:
    # Your code here
    result = await env.call_tool("your_tool", {...})
```

**Security Best Practices:**
- ✅ Always use environment variables - never hardcode API keys
- ✅ Store keys in `.env` files (add to `.gitignore`)
- ✅ Use secret management in production (AWS Secrets Manager, etc.)
- ❌ Never commit API keys to version control
- ❌ Never log or print API keys

### 4. Build and Deploy

Use the automated build script (recommended):

```bash
# Run from project root directory
bash .claude/skills/aenvironment-agent/scripts/build_env.sh temp/agent-XXXX

# Or with custom version:
# bash .claude/skills/aenvironment-agent/scripts/build_env.sh temp/agent-XXXX 2.0.0
```

**What the script does**:
1. ✅ Checks prerequisites (aenv, Docker)
2. 🔨 Validates environment structure
3. 🐳 Builds Docker image with `aenv build --push` (builds AND pushes to registry)
4. 📤 Registers environment with `aenv push` (registers in AEnvironment hub)
5. ✅ Verifies deployment with `aenv get`
6. 📊 Reports success or failure

**Note**: `aenv build --push` automatically pushes the Docker image to your configured Docker registry, then `aenv push` registers the environment metadata in the AEnvironment hub.

**Alternative: Manual Build** (if you need more control):

```bash
# Navigate to environment directory
cd temp/agent-XXXX

# Build Docker image only (no push)
aenv build

# Build and push to Docker registry
aenv build --push

# Register in AEnvironment hub
aenv push

# Verify deployment
aenv get agent-XXXX -v 1.0.0
```

**Verification**: Check output for "✅ Build and deployment completed successfully!"

## Client Application Development

### 5. Develop Client Application

Use AEnvironment SDK to interact with deployed environments.

#### Environment Configuration

**IMPORTANT**: The `Environment` class automatically reads the hub URL from the `AENV_SYSTEM_URL` environment variable. You do NOT need to pass it as a parameter.

```bash
# Set once in your shell or .bashrc/.zshrc
export AENV_SYSTEM_URL=http://your-hub-address:8080

# Then your client code automatically uses this URL
```

#### Basic Client Pattern

```python
import asyncio
import os
from aenv import Environment

async def main():
    # Read LLM configuration from environment
    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4")

    if not api_key:
        print("❌ Error: OPENAI_API_KEY not set")
        print("   Set with: export OPENAI_API_KEY='your-key'")
        return

    # IMPORTANT: No need to specify aenv_url parameter
    # The Environment class automatically reads from AENV_SYSTEM_URL env var
    async with Environment(
        "agent-XXXX@1.0.0",  # Replace with your environment name
        environment_variables={
            # Pass these to the agent environment
            "OPENAI_API_KEY": api_key,
            "OPENAI_BASE_URL": base_url,
            "OPENAI_MODEL": model
        },
        timeout=120  # Adjust based on expected response time
    ) as env:
        # List available tools
        tools = await env.list_tools()
        print("Available tools:", [t["name"] for t in tools])

        # Call a tool
        result = await env.call_tool("chat", {"user_request": "Hello!"})

        # Parse result
        data = parse_tool_result(result)
        print(f"Status: {data.get('status')}")
        print(f"Response: {data.get('response')}")

asyncio.run(main())
```

**Key Points**:
- ✅ Set `AENV_SYSTEM_URL` environment variable once (not in code)
- ✅ Pass LLM credentials via `environment_variables` parameter
- ✅ These variables are injected into the agent's runtime environment
- ✅ The agent reads them using `os.getenv()` in its tools

#### Result Parsing Helper

```python
import json

def parse_tool_result(result):
    """
    Parse ToolResult.content into structured data.
    Handles list, string, and dict content types.
    """
    content = result.content

    # List content (most common from MCP)
    if isinstance(content, list):
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                text = item.get("text", "")
                if text:
                    try:
                        return json.loads(text)
                    except json.JSONDecodeError:
                        return {"raw": text}

    # String content
    elif isinstance(content, str):
        try:
            return json.loads(content)
        except json.JSONDecodeError:
            return {"raw": content}

    # Dict content
    elif isinstance(content, dict):
        return content

    raise ValueError(f"Cannot parse content type: {type(content)}")
```

#### Interactive Client Pattern

```python
async def interactive_client():
    """Interactive CLI for agent interaction."""
    async with Environment("agent-XXXX@1.0.0", ...) as env:
        print("🤖 Agent ready. Type 'quit' to exit.\n")

        while True:
            user_input = input("You: ").strip()

            if user_input.lower() in ['quit', 'exit']:
                break

            if not user_input:
                continue

            try:
                result = await env.call_tool("chat", {"user_request": user_input})
                data = parse_tool_result(result)

                if data.get("status") == "success":
                    print(f"Agent: {data.get('response')}\n")
                else:
                    print(f"❌ Error: {data.get('message')}\n")

            except Exception as e:
                print(f"❌ Error: {e}\n")
```

**Key Best Practices**:
- **Always use `async with`**: Ensures proper resource cleanup
- **Set appropriate timeout**: Long-running operations need longer timeouts (default: 30s)
- **Pass environment variables**: Use `environment_variables` parameter for LLM config
- **Handle all exceptions**: Network errors, timeouts, tool errors
- **Parse results consistently**: Use helper function for all tool calls

## Common Patterns and Best Practices

### Tool Implementation Patterns

#### Multi-turn Conversation with Context

```python
from collections import defaultdict
from aenv import register_tool

# Store conversation history per session
_conversations = defaultdict(list)

@register_tool
async def chat(message: str, session_id: str = "default") -> dict:
    """Chat with conversation context."""
    messages = _conversations[session_id]

    # Add user message
    messages.append({"role": "user", "content": message})

    # Get LLM response
    client = get_llm_client()
    response = await client.chat.completions.create(
        model=os.getenv("OPENAI_MODEL"),
        messages=messages
    )

    # Store assistant response
    assistant_msg = response.choices[0].message.content
    messages.append({"role": "assistant", "content": assistant_msg})

    # Keep last 20 messages to manage memory
    if len(messages) > 20:
        _conversations[session_id] = messages[-20:]

    return {
        "response": assistant_msg,
        "status": "success",
        "session_id": session_id
    }
```

#### Multiple Tool Types

```python
from aenv import register_tool, register_function, register_reward

# Exposed tool for external use
@register_tool
def search(query: str) -> dict:
    """Search for information."""
    results = internal_search(query)
    return {"results": results, "status": "success"}

# Internal function for environment logic
@register_function
def internal_search(query: str) -> list:
    """Internal search implementation."""
    return [...]  # Search logic

# Reward function for RL training
@register_reward
def evaluate_search(results: dict) -> float:
    """Evaluate search quality for RL."""
    return len(results.get("results", [])) / 10.0
```

### Error Handling Best Practices

1. **Always catch exceptions**: Never let exceptions propagate from tools
2. **Return consistent format**: Every tool should return dict with `status` and `message`
3. **Provide helpful messages**: Include specific error information
4. **Log for debugging**: Use logging for troubleshooting

```python
import logging

logger = logging.getLogger(__name__)

@register_tool
async def robust_tool(param: str) -> dict:
    """Tool with comprehensive error handling."""
    try:
        # Input validation
        if not param or len(param) > 1000:
            return {
                "status": "error",
                "message": "Invalid input: must be 1-1000 characters"
            }

        # Main logic
        result = await process(param)

        return {
            "result": result,
            "status": "success",
            "message": "Operation completed"
        }

    except ValueError as e:
        logger.warning(f"Validation error: {e}")
        return {"status": "error", "message": f"Invalid input: {e}"}

    except ConnectionError as e:
        logger.error(f"Connection error: {e}")
        return {"status": "error", "message": "Service unavailable"}

    except Exception as e:
        logger.exception("Unexpected error")
        return {"status": "error", "message": f"Unexpected error: {e}"}
```

## Troubleshooting Guide

### Common Issues and Solutions

#### 1. Environment Initialization Fails

**Symptoms**: `aenv init` command fails or creates incomplete structure

**Solutions**:
- Ensure `aenv` is installed: `pip install aenvironment`
- Verify you're in project root directory
- Check disk space and write permissions
- Try with explicit parameters: `bash scripts/init_env.sh $(pwd) my-agent`

#### 2. Build Fails

**Symptoms**: `aenv build` fails during Docker build

**Solutions**:
- Check Docker is running: `docker ps`
- Verify Dockerfile syntax
- Ensure all dependencies in requirements.txt are valid
- Check for syntax errors in `src/custom_env.py`
- Review build logs for specific errors

#### 3. Deployment Verification Fails

**Symptoms**: `aenv get` cannot find environment after push

**Solutions**:
- Wait 30-60 seconds for registry sync
- Verify push completed successfully (check logs)
- Try explicit version: `aenv get agent-XXXX -v 1.0.0`
- Check network connectivity to registry

#### 4. Client Connection Timeout

**Symptoms**: Client hangs or times out when connecting

**Solutions**:
- Increase timeout: `Environment(..., timeout=300)`
- Check environment is deployed: `aenv get agent-XXXX`
- Verify network connectivity
- Check environment hub status

#### 5. Tool Returns Error

**Symptoms**: Tool calls fail with errors

**Solutions**:
- Verify environment variables are passed correctly
- Check tool implementation for bugs
- Test locally with `aenv run` before deploying
- Review tool logs for specific errors
- Ensure LLM API keys are valid

#### 6. Result Parsing Fails

**Symptoms**: Cannot parse tool result content

**Solutions**:
- Use provided `parse_tool_result()` helper function
- Check tool returns proper dict format with `status` key
- Handle all content types (list, str, dict)
- Add defensive checks for missing keys

## Complete Workflow Checklist

### ✅ Environment Development

**Initialization**:
- [ ] Run init script from project root: `bash .claude/skills/aenvironment-agent/scripts/init_env.sh`
- [ ] Verify generated structure in `temp/agent-XXXX/`
- [ ] Never create files manually - always use init script

**Implementation**:
- [ ] Edit `src/custom_env.py` and implement tools with `@register_tool`
- [ ] Add type hints to all function parameters and returns
- [ ] Write clear docstrings for every tool
- [ ] Return consistent dict format with `status` and `message` keys
- [ ] Handle all exceptions - never let them propagate
- [ ] Use `async def` for I/O-bound operations

**Configuration**:
- [ ] Update `requirements.txt` with dependencies (e.g., `openai>=1.0.0`)
- [ ] Update `Dockerfile` base image if needed
- [ ] Verify `config.json` is appropriate (usually no changes needed)

**Build & Deploy**:
- [ ] Run build script: `bash .claude/skills/aenvironment-agent/scripts/build_env.sh temp/agent-XXXX`
- [ ] Verify build completes without errors
- [ ] Confirm deployment with `aenv get agent-XXXX -v 1.0.0`
- [ ] Test locally with `aenv run` before deploying if needed

### ✅ Client Development

**Setup**:
- [ ] Install SDK: `pip install aenvironment`
- [ ] Set environment variables: `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`
- [ ] Verify environment is deployed and accessible

**Implementation**:
- [ ] Use `Environment` class with `async with` context manager
- [ ] Pass LLM config via `environment_variables` parameter
- [ ] Set appropriate timeout (default 30s, increase for long operations)
- [ ] Use `parse_tool_result()` helper for consistent parsing
- [ ] Handle all exceptions (timeout, connection, tool errors)

**Testing**:
- [ ] Test basic connection and tool listing
- [ ] Verify tool calls work correctly
- [ ] Test error handling (invalid inputs, timeouts)
- [ ] Verify result parsing for all response types

## Key Principles

1. **Always use automation scripts**:
   - Init: Use `scripts/init_env.sh` - never create files manually
   - Build: Use `scripts/build_env.sh` - includes automatic verification

2. **Consistent return format**:
   - All tools must return dict with `status`, `message`, and result fields
   - Status should be "success" or "error"
   - Message should be human-readable

3. **Environment variables for configuration**:
   - Read API keys and config from environment variables
   - Pass via `environment_variables` parameter in client
   - Never hardcode sensitive data

4. **Resource cleanup**:
   - Always use `async with` context manager for Environment
   - Ensures proper cleanup even on errors

5. **Comprehensive error handling**:
   - Catch all exceptions in tools
   - Return structured error responses
   - Never let exceptions propagate to MCP layer

6. **Type safety**:
   - Use type hints for all functions
   - Helps catch errors early and improves IDE support

7. **Testing before deployment**:
   - Test locally with `aenv run` when possible
   - Verify tools work as expected before deploying

## Advanced Topics

For detailed patterns and examples, see the reference documentation:

- **[complete-workflow.md](references/complete-workflow.md)** - End-to-end HTML generation agent example
- **[tool-patterns.md](references/tool-patterns.md)** - Common tool implementation patterns
- **[client-patterns.md](references/client-patterns.md)** - Advanced SDK usage patterns

## Additional Resources

### Example Code
- **[assets/example_custom_env.py](assets/example_custom_env.py)** - Complete tool implementation example
- **[assets/example_client.py](assets/example_client.py)** - Complete client application example

### Automation Scripts
- **[scripts/init_env.sh](scripts/init_env.sh)** - Environment initialization script
- **[scripts/build_env.sh](scripts/build_env.sh)** - Build and deployment script

### Quick Command Reference

```bash
# Initialize new environment
bash .claude/skills/aenvironment-agent/scripts/init_env.sh

# Build and deploy
bash .claude/skills/aenvironment-agent/scripts/build_env.sh temp/agent-XXXX

# Verify deployment
aenv get agent-XXXX -v 1.0.0

# Test locally (run from environment directory)
cd temp/agent-XXXX && aenv run

# List available environments
aenv list

# Get environment details
aenv get agent-XXXX --verbose
```

## Summary

This skill provides a complete, production-ready workflow for AEnvironment agent development:

1. **Init** → Use automation scripts for proper project structure
2. **Implement** → Write tools with consistent patterns and error handling
3. **Build** → Automated build and verification
4. **Deploy** → Push to environment hub with validation
5. **Use** → Robust client applications with proper resource management

Follow the checklists and best practices to ensure stable, reliable agent environments.