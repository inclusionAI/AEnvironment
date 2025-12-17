# All-in-One

A complete example demonstrating how to create, build, test, and deploy a custom AEnvironment with tools, functions, and rewards.

## Overview

This example shows a weather demo environment that includes:
- **Tools**: `get_weather` - Get weather information for a city
- **Functions**: `get_weather_func` - Function version of weather retrieval
- **Rewards**: `is_good_weather` - Reward function that checks if weather conditions are favorable

## Project Structure

```
all_in_one/
├── config.json              # Environment configuration
├── Dockerfile               # Container image definition
├── requirements.txt         # Python dependencies
├── run_custom_env.py        # Example usage script
└── src/
    ├── custom_env.py        # Tool, function, and reward definitions
    └── test_custom_env.py   # Unit tests
```

## Quick Start

### 1. Prerequisites

- Python 3.12+
- Docker installed and running
- AEnvironment CLI installed (`pip install aenvironment`)
- Access to AEnvironment platform (or running locally)

### 2. Define Your Environment

The environment is defined in `src/custom_env.py`:

```python
from aenv import register_tool, register_function, register_reward
from typing import Dict, Any

@register_tool
def get_weather(city: str) -> Dict[str, Any]:
    """Get weather information for a city."""
    return {
        "city": city,
        "temperature": "20",
        "description": city,
        "humidity": "conf"
    }
    
@register_function
def get_weather_func(city: str) -> Dict[str, Any]:
    """Function version of weather retrieval."""
    return {
        "city": city,
        "temperature": "20",
        "description": city,
        "humidity": "conf"
    }

@register_reward
def is_good_weather(city: str) -> bool:
    """Check if weather conditions are favorable."""
    result = get_weather(city)
    return int(result["temperature"]) > 15 and int(result["temperature"]) < 30
```

### 3. Configure Environment

Edit `config.json` to customize your environment:

```json
{
    "name": "weather-demo",
    "version": "1.0.0",
    "tags": ["swe", "python", "linux"],
    "status": "Ready",
    "codeUrl": "oss://xxx",
    "artifacts": [
        {
            "type": "image",
            "content": "docker.io/aenv/weather-demo:1.0.0"
        }
    ],
    "buildConfig": {
        "dockerfile": "./Dockerfile"
    },
    "testConfig": {
        "script": "pytest xxx"
    },
    "deployConfig": {
        "cpu": "1",
        "memory": "2G",
        "os": "linux"
    }
}
```

### 4. Build Environment

Build the Docker image locally:

```bash
cd aenv/examples/all_in_one
aenv build
```

This will:
- Build the Docker image based on `Dockerfile`
- Tag it according to `config.json`
- Make it ready for local testing or pushing

### 5. Test Locally

Run the test suite:

```bash
aenv test
```

Or run tests manually:

```bash
pytest src/test_custom_env.py
```

### 6. Push to Registry

Push your environment to the AEnvironment registry:

```bash
aenv push
```

This uploads:
- Environment metadata to EnvHub
- Docker image to the configured registry

### 7. Use Environment

#### Local Usage

Run the example script:

```bash
python run_custom_env.py
```

The script demonstrates:
- Creating an environment instance
- Listing available tools
- Calling tools and functions
- Calling reward functions
- Releasing the environment

```python
import asyncio
from aenv import Environment
import os

async def main():
    os.environ["AENV_SYSTEM_URL"] = "http://localhost:8080/"
    env = Environment("weather-demo@1.0.0", timeout=60)
    try:
        # List available tools
        print(await env.list_tools())
        
        # Call a tool
        print(await env.call_tool("get_weather", {"city": "Beijing"}))
        
        # Call a function
        print(await env.call_function("get_weather_func", {"city": "Beijing"}))
        
        # Call a reward function
        print(await env.call_reward({"city": "Beijing"}))
    finally:
        await env.release()

asyncio.run(main())
```

#### Cluster Usage

In your application code:

```python
from aenv import Environment

async def main():
    # Environment will be created in the cluster
    async with Environment("weather-demo@1.0.0") as env:
        # Use tools, functions, and rewards
        weather = await env.call_tool("get_weather", {"city": "Shanghai"})
        reward = await env.call_reward({"city": "Shanghai"})
        print(f"Weather: {weather}, Good weather: {reward}")
```

## Workflow

### Development Workflow

1. **Edit** your tools/functions/rewards in `src/custom_env.py`
2. **Build** the environment: `aenv build`
3. **Test** locally: `aenv test` or `pytest src/test_custom_env.py`
4. **Push** to registry: `aenv push`
5. **Use** in your applications

### Visual Guides

See the `images/` directory for animated GIFs demonstrating:
- `build_env_in_local.gif` - Building environment locally
- `push_env_in_local.gif` - Pushing to registry
- `run_env_in_local.gif` - Running locally
- `test_env_in_local.gif` - Testing environment
- `use_env_in_cluster.gif` - Using in cluster

## Key Concepts

### Tools vs Functions vs Rewards

- **Tools** (`@register_tool`): Executable functions that can be called by agents, return structured data
- **Functions** (`@register_function`): Similar to tools but may have different semantics in your use case
- **Rewards** (`@register_reward`): Functions that return boolean or numeric values for RL training

### Environment Lifecycle

1. **Create**: `Environment("name@version")` - Creates or connects to an instance
2. **Use**: Call tools, functions, and rewards
3. **Release**: `await env.release()` - Clean up resources

### Configuration Options

- **Template Type**: Specify in `config.json` under `deployConfig.templateType`:
  - `"singleContainer"` (default) - Single container pod
  - `"dualContainer"` - Dual container pod with sidecar

## Troubleshooting

### Build Issues

- Ensure Docker is running: `docker ps`
- Check Dockerfile syntax: `docker build -t test .`
- Verify Python dependencies: `pip install -r requirements.txt`

### Runtime Issues

- Check environment URL: Set `AENV_SYSTEM_URL` environment variable
- Verify environment exists: `aenv list` or `aenv get weather-demo@1.0.0`
- Check logs: Environment logs are available through the API service

### Testing Issues

- Ensure test environment is set: `DUMMY_INSTANCE_IP=127.0.0.1`
- Run tests with verbose output: `pytest -v src/test_custom_env.py`
- Check test dependencies: `pip install pytest pytest-asyncio`

## Next Steps

- Explore other examples in `aenv/examples/`
- Read the [Architecture Documentation](../architecture/architecture.md)
- Check the [Development Guide](../development/development.md)
- Contribute your own examples!

## See Also

- [AEnvironment Architecture](../architecture/architecture.md)
- [CLI Documentation](../guide/cli.md)
- [SDK API Reference](../guide/sdk.md)
