---
name: environment-synthesis
description: Guide for using AEnvironment to automate agent synthesis and environment creation. Use when developers need to create, build, deploy, and manage agent environments using AEnvironment framework. Supports environment initialization, tool registration, configuration management, and client integration for agent automation workflows.
---

# AEnvironment Agent Synthesis Guide

This skill provides guidance for using AEnvironment to automate agent synthesis and environment creation.

## Overview

AEnvironment is a framework for creating, managing, and deploying agent environments. It enables developers to:

- Initialize new agent environments
- Register custom tools and capabilities
- Build and deploy environments as Docker containers
- Integrate environments with client applications
- Manage environment configurations and versions

## Quick Start

### 1. Initialize an Environment

**⚠️ IMPORTANT: Environment initialization MUST be done in your project root directory, NOT in the skill root directory.**

**⚠️ REQUIRED: You MUST use the `init_env.sh` script to initialize a brand new environments. Manual creation and existing environment reuse is not supported.**

Steps:
1. Navigate to your **project root directory** (where you want to create the agent environment)
2. Run the init script from the skill directory:

```bash
# Strictly Run the init script
./.claude/skills/environment-synthesis/scripts/init_env.sh
```

**Note:** The script will automatically:
- Create a `temp/` directory in your project root if it doesn't exist
- Generate a unique environment ID (e.g., `agent-xxxx`)
- Create a Python virtual environment
- Initialize the AEnvironment structure

This creates a new directory structure in `temp/your-agent-xxxx/`:
```
temp/
└── your-agent-xxxx/
    ├── config.json
    ├── Dockerfile
    ├── requirements.txt
    └── src/
        └── custom_env.py
```

### 2. Configure Environment

Edit `config.json` to set environment metadata. In most cases, the default config.json configuration is sufficient and no modifications are required.

### 3. Implement agent logic in chat tool

In `src/custom_env.py`, register tools using the `@register_tool` decorator:

```python
from aenv import register_tool

@register_tool
async def chat(param: str) -> Dict[str, Any]:
    """
    Tool description.
    
    Args:
        param: Parameter description
        
    Returns:
        Dictionary with results
    """
    # Agent implementation
    return {"status": "success", "result": "..."}
```

### 4. Build and Deploy

Use the build script which includes automatic verification:

```bash
# From your project root directory
./.claude/skills/environment-synthesis/scripts/build_env.sh temp/your-agent-xxxx
```

The build script will:
- Build and push the Docker image
- Verify the local Docker image exists
- Verify the environment is available in envhub via `aenv get`
- Report any failures

### 5. Create Client Application

Create a client application to interact with your environment. See [references/client-development.md](references/client-development.md) for complete client implementation patterns and examples.

Use the template script as a starting point:

```bash
# From your project root directory
cp .claude/skills/environment-synthesis/scripts/client_template.py client.py
# Edit client.py to customize for your use case
```

### 6. Run Client Application

Execute the client application to test your environment:

```bash
# Set environment variables
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="your-base-url"  # Optional
export OPENAI_MODEL="gpt-4o-mini"  # Optional

# Run the client
python3 client.py
```

The client will:
- Initialize the environment
- List available tools
- Provide an interactive interface to call tools
- Handle errors and display results

## Best Practices

1. **Environment Initialization**: 
   - **MUST** use `init_env.sh` script - manual environment creation is not supported
   - **MUST** run initialization from your project root directory, not from the skill directory
   - The script automatically generates unique environment IDs to avoid conflicts
2. **No Summary Doc**: No summary documentation should be written.
3. **Error Handling**: Always return structured error responses from tools
4. **Resource Cleanup**: Use `async with` to ensure proper resource management
5. **Tool Documentation**: Provide clear docstrings for all registered tools

## Complete Workflow

For more control, run steps individually:

**⚠️ All commands should be executed from your project root directory:**

```bash
# Step 1: Initialize environment (MUST be done in project root directory)
# Navigate to your project root directory first
cd /path/to/your/project_root

# Run the init script (REQUIRED - do not manually create environments)
./.claude/skills/environment-synthesis/scripts/init_env.sh

# Step 2-4: Configure and develop agent (manual steps)
# Edit temp/your-agent-xxxx/config.json
# Edit temp/your-agent-xxxx/src/custom_env.py

# Step 5: Build and deploy (includes verification)
./.claude/skills/environment-synthesis/scripts/build_env.sh temp/your-agent-xxxx
# This will:
#   - Build and push Docker image
#   - Verify local Docker image exists
#   - Verify environment is available in envhub

# Step 6: Create client application
cp .claude/skills/environment-synthesis/scripts/client_template.py client.py
# Edit client.py with your environment name (e.g., "your-agent-xxxx@1.0.0")

# Step 7: Run client application
python3 client.py
```

## References
- [references/tool-development.md](references/tool-development.md) - Tool registration patterns
- [references/client-development.md](references/client-development.md) - Advanced client patterns
- [scripts/init_env.sh](scripts/init_env.sh) - Environment initialization
- [scripts/build_env.sh](scripts/build_env.sh) - Build and deployment
- [scripts/client_template.py](scripts/client_template.py) - Client template

