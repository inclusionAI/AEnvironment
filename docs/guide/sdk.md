# AEnvironment Python SDK Guide

In reinforcement learning (RL) training phases, the environment is an indispensable key factor. High-quality, rapidly extensible environments can significantly improve the efficiency and quality of RL training. To help developers quickly build required environments, we provide a complete development SDK that enables efficient environment creation and management.

> **Environment Concept**: An environment is a containerized sandbox that can integrate any form of tools or code, providing standardized tool and reward services externally to achieve rapid delivery of production-grade environments.

## Quick Start

### Install SDK

```bash
pip install aenvironment
```

### Verify Installation

```python
import aenv
print(aenv.__version__)
```

### Run Environment Instance

#### Runtime Mode Selection

##### Local Mode

In local mode, you need to start the environment execution sandbox via `aenv run` and set environment variables:

```bash
export DUMMY_INSTANCE_IP=http://localhost
```

##### Remote Mode

In remote mode, you need to deploy components first, obtain the service address and configure:

```bash
export AENV_SYSTEM_URL=http://service
```

#### Start Environment Instance

<details>
<summary>mini-terminal code example</summary>

```python
import asyncio
from aenv import Environment

async def main():
    # Create environment instance
    mini_terminal = Environment("mini-terminal@1.0.1", ttl="60m")

    try:
        # Get available tools list
        tools = await mini_terminal.list_tools()
        print("Successfully obtained tool list:", tools)
        assert tools is not None
    except Exception as e:
        print(
            "Test completed - environment created successfully, but tool list may be empty:",
            str(e),
        )

    # Interactive command execution
    while True:
        try:
            user_input = input(">>> ").strip()
            if user_input.lower() in ("exit", "quit"):
                print("Exiting interactive mode")
                break

            # Call remote tool to execute command
            result = await mini_terminal.call_tool(
                "mini-terminal@1.0.1/execute_command",
                {"command": user_input, "timeout": 5}
            )
            print("Execution result:\n", result)
            print("-" * 60)

        except KeyboardInterrupt:
            print("\nInterrupt detected, enter 'exit' to quit or continue input")
            await mini_terminal.release()
            print("Environment successfully released")
if __name__ == "__main__":
    asyncio.run(main())
```

</details>

#### Environment Execution Demo

![Start Environment Instance](../images/guide/aenv_instance_log.gif)

## Environment Development

### Tool Registration

Use the `@register_tool` decorator to quickly register existing Python methods as environment tools. Registered tools will ultimately be provided as MCP Server services.

#### How It Works

1. Start corresponding containers when creating environment instances
2. Launch FastMCP Server within containers
3. Register all tools to MCP Server
4. Unified invocation through MCP standard protocol

#### Development Example

Example code:

```python
from typing import Dict, Any
from aenv import register_tool

@register_tool
def my_custom_echo_env(content: str) -> Dict[str, Any]:
    """
    Custom environment tool description.

    This tool receives any string content and returns it as-is output, used to verify environment integration effects.

    Args:
        content: Any content to echo

    Returns:
        Dictionary containing results in format {"result": "original content"}
    """
    return {"result": f"{content}"}
```

#### Parameter Specification

##### Input Parameters

Input parameter Schema must be consistent with MCP Tool input Schema, mainly supporting basic data types and avoiding complex nested structures.

**Supported Types:**

- Basic types: `str`, `int`, `float`, `bool`
- Lists: `list[str]`, `list[int]`, etc.
- Simple Pydantic models
- Optional types: `Optional[str]`
- Default parameter values

**Not Recommended or Limited Types:**

- Deep nested structures
- Complex generic types
- Overly complex custom validation logic
- Dynamically generated models

**Example:**

```python
from aenv import register_tool

mcp = FastMCP("Example")

# ✅ Recommended type usage
@register_tool
def simple_types(
    name: str,           # string
    count: int,          # integer
    price: float,        # float
    enabled: bool,       # boolean
    tags: list[str]      # string list
) -> str:
    return "OK"

# ✅ Using Pydantic models
class UserQuery(BaseModel):
    query: str
    max_results: int = 10
    include_details: bool = False

@register_tool
def search_users(params: UserQuery) -> str:
    return f"Searching: {params.query}"
```

##### Return Value Processing

When calling environment instances, pay attention to correct parsing of return values. We have transformed the original `mcp.types.CallToolResult` type, retaining only core data.

**Data Transformation Logic:**

```python
# MCP original result format
class CallToolResult(Result):
    is_error: Boolean = False
    content: List[Dict[str, Any]]

# AEnvironment transformed format
class ToolResult:
    is_error: Boolean = False
    content: List[Dict[str, Any]]

# Transformation process
content: List[Dict[str, Any]] = []

for item in mcp_call_results.content:
    if hasattr(item, "text") and item.text:
        content.append({"type": "text", "text": item.text})
    elif hasattr(item, "type") and hasattr(item, "data"):
        content.append({"type": item.type, "data": item.data})
    else:
        content.append({"type": "text", "text": str(item)})

return ToolResult(content=content, is_error=result.isError)
```

**Calling Example:**

```python
result = await mini_terminal.call_tool(
    "env_name@1.0.1/execute_command",
    {"input": input, "timeout": 5}
)

# Execution log
20251212-14:47:57.174 aenv.environment INFO: [ENV:mini-terminal-f3e28p] Executing tool: execute_command in environment mini-terminal@1.0.1

# Return result
Execution result:
ToolResult(content=[{
    'type': 'text',
    'text': '{"output":["helloworld\n",true],"returncode":0}'
}], is_error=False)
```

##### Tool Discovery

Query available tools in the current environment via the `list_tools` interface:

```python
# 1. Get all tools
tools = await mini_terminal.list_tools()

# 2. Tool details example
[
    {
        "name": "mini-terminal@1.0.1/execute_command",
        "description": null,
        "inputSchema": {
            "properties": {
                "command": {"type": "string"},
                "timeout": {"default": 60, "title": "Timeout"}
            },
            "required": ["command"],
            "type": "object"
        }
    }
]
```

### Reward Functions

Reward functions are mainly used in reinforcement learning training processes to evaluate whether the impact of agent actions on environment states meets expectations. They are typically divided into positive rewards and negative rewards.

Since they need to perceive environmental changes, reward functions are closely coupled with the environment and therefore implemented as part of the environment. Use the `@register_reward` decorator to register Python methods as reward functions.

```python
from aenv import register_reward

@register_reward
def evaluate_code_quality(code: str, test_results: dict) -> dict:
    """
    Reward function for evaluating code quality.

    Comprehensive scoring based on test results, code length, and code standards.

    Args:
        code: Code string to evaluate
        test_results: Test result dictionary containing pass/fail statistics

    Returns:
        Dictionary containing score, feedback, and detailed information
    """
    score = 0.0
    feedback = []

    # Score based on test results
    if test_results.get("passed", 0) > 0:
        score += 0.5
        feedback.append("Tests passed")

    # Score based on code length
    if len(code) < 1000:
        score += 0.3
        feedback.append("Code concise")

    # Score based on code standards
    if "def " in code and "import " in code:
        score += 0.2
        feedback.append("Good structure")

    return {
        "score": score,
        "feedback": "; ".join(feedback),
        "details": test_results
    }

# Use reward function
reward = await env.call_reward({
    "code": "def hello(): return 'world'",
    "test_results": {"passed": 5, "failed": 0}
})
```

### Function Registration

Functions serve as extension points for the environment. Use the `@register_function` decorator to register any Python function to the environment. Registered functions essentially become endpoints of HTTP services within environment-associated containers, providing services externally via HTTP.

```python
from aenv import register_function

@register_function
def custom_endpoint(data: dict) -> dict:
    """Custom HTTP endpoint function"""
    return {"status": "success", "data": data}
```

### Health Checks

You can customize environment health check logic according to specific scenarios. Use the `@register_health` decorator to implement.

```python
from aenv import register_health

@register_health
def system_health_check() -> dict:
    """
    System health check function.

    Check CPU, memory, disk usage, and system running status.

    Returns:
        Dictionary containing various health indicators
    """
    import psutil

    return {
        "status": "healthy",
        "cpu_percent": psutil.cpu_percent(),
        "memory_percent": psutil.virtual_memory().percent,
        "disk_usage": psutil.disk_usage("/").percent,
        "uptime": "Running normally"
    }

# Execute health check
health = await env.check_health()
```

## Environment Usage

### Basic Usage

```python
import asyncio
from aenv import Environment

async def basic_example():
    # Create environment instance
    env = Environment("my-python-env")

    # Initialize environment
    await env.initialize()

    # Use environment
    tools = await env.list_tools()
    print(f"Available tools: {len(tools)}")

    # Destroy environment
    await env.destroy()

# Run example
asyncio.run(basic_example())
```

### Execution Flow

1. **Instantiate Environment**
   - User requests to create specific type environment (e.g., "trading-env")
   - Environment scheduler starts corresponding Docker containers based on preset image mapping
   - FastMCP service automatically starts within containers, loading environment-specific tool sets and reward functions

2. **Tool Invocation**
   - User calls via environment instance: `env.call_tool("analyze_market", data)`

3. **Internal Routing**
   - Environment manager forwards requests to container corresponding endpoints (e.g., `http://localhost:8000/tools/analyze_market`)

4. **Service Execution**
   - FastMCP service within container receives and parses requests
   - Execute corresponding tool logic or reward calculation

5. **Result Return**
   - Execution results return via HTTP response

### Constructor Parameters

| Parameter Name | Type | Default | Description | Use Case |
|----------------| ------ |---------|-------------|----------|
| `env_name` | str | Required | Environment name | Identify environment instance |
| `datasource` | str | `""` | Data source path | Mount data volumes |
| `ttl` | str | `"30m"` | Lifecycle | Auto-destruction time |
| `environment_variables` | dict | `None` | Environment variables | Configure runtime environment |
| `arguments` | list | `None` | Startup arguments | Container startup parameters |
| `aenv_url` | str | `None` | Service address | AEnvironment platform |
| `timeout` | float | `30.0` | Request timeout | Network request timeout |
| `max_retries` | int | `10` | Max retries | Failure retry count |
| `api_key` | str | `None` | API key | Authentication |

### Recommended Usage: Context Manager

Using context managers ensures proper initialization and automatic cleanup of environment resources:

```python
import asyncio
from aenv import Environment

async def recommended_usage():
    # Recommended: automatic initialization and destruction
    async with Environment("safe-env") as env:
        # Environment automatically initialized
        tools = await env.list_tools()

        # Execute tools
        result = await env.call_tool("python", {
            "code": "print('Hello from AEnvironment!')"
        })

        print(result.content)
        # Automatically destroys environment on exit

asyncio.run(recommended_usage())
```

### Advanced Configuration

#### Complete Configuration Example

```python
env = Environment(
    env_name="data-analysis-env",      # Environment name
    datasource="/data/workspace",      # Data source path
    ttl="2h",                          # Lifecycle 2 hours
    environment_variables={
        "PYTHONPATH": "/app",
        "MODEL_PATH": "/models/bert",
        "API_KEY": "sk-xxx"
    },
    arguments=["--verbose", "--gpu"],  # Startup arguments
    aenv_url="http://localhost:8080",  # AEnvironment service address
    timeout=60.0,                      # Timeout (seconds)
    max_retries=5,                     # Max retry count
    api_key="your-api-key"            # API key
)
```

#### Dataset Specification

```python
env = Environment(
    env_name="swe-env",
    datasource="/path/to/dataset"
)
```

#### Environment Variable Injection

Pass environment variables to containers:

```python
env = Environment(
    env_name="my-env",
    environment_variables={
        "API_KEY": "secret-key",
        "DEBUG": "true",
        "LOG_LEVEL": "info"
    }
)
```

#### Startup Parameter Configuration

Specify startup arguments for custom environments:

```python
env = Environment(
    env_name="my-env",
    arguments=["--config", "/app/config.yaml", "--verbose"]
)
```

#### Lifecycle Management

Environment instances have a default lifecycle of 30 minutes, after which the system automatically recycles them. Can be configured as follows:

##### 1. Parameter Configuration (Highest Priority)

```python
# 30 minutes (default)
env = Environment("my-env", ttl="30m")

# 2 hours
env = Environment("my-env", ttl="2h")

# 1 day
env = Environment("my-env", ttl="24h")
```

##### 2. Global Configuration

Configure uniformly in `config.json`, affecting all instances created by this environment:

```json
{
    "deployConfig": {
        "ttl": "2h"
    }
}
```

**Configuration Priority**: Parameter Configuration > Global Configuration > System Default

## FAQ

### Q1: Do environment instances need to be actively released after creation?

**Recommendation**: Environment instances should be actively released after use to avoid resource waste.

**Best Practices**:

- Short-term tasks: Set reasonable TTL for automatic recycling
- Long-term tasks: Use context managers to ensure resource cleanup
- Interactive use: Explicitly call `await env.release()`

**Evaluation Principle**: Assess reasonable TTL values based on specific usage scenarios, balancing resource utilization and user experience.
