# Environments

## Environment Introduction

AEnvironment is an **Environment-as-Code** development framework that enables you to define reusable environments using Python for agent construction and reinforcement learning training. The framework supports the MCP protocol and enables one-click deployment to the cloud.

> **Core Philosophy**: Abstract complex runtime environments into programmable, version-controlled, and reusable environment units. Developers can define complete agent runtime environments through declarative Python code—from toolsets and reward functions to custom functional modules—achieving one-click cloud deployment with seamless MCP protocol integration.

## Environment Configuration

### Configuration File

Each environment project contains a core configuration file: `config.json`

```json
{
  "name": "my-env",
  "version": "1.0.0",
  "description": "My custom environment",
  "tags": ["custom", "python", "ml"],
  "status": "Ready",
  "artifacts": [],
  "buildConfig": {
    "dockerfile": "./Dockerfile",
    "context": "."
  },
  "testConfig": {
    "script": "pytest tests/"
  },
  "deployConfig": {
    "cpu": "2C",
    "memory": "4G",
    "os": "linux",
    "imagePrefix": "registry.example.com/envs",
    "podTemplate": "Default"
  }
}
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique environment identifier |
| `version` | string | Yes | Semantic version number |
| `description` | string | No | Human-readable environment description |
| `tags` | string[] | No | Searchable tag collection |
| `status` | string | No | Environment status identifier |
| `artifacts` | object[] | No | Build artifacts list |
| `buildConfig` | object | Yes | Build configuration |
| `testConfig` | object | No | Test configuration |
| `deployConfig` | object | Yes | Deployment configuration |

#### Build Artifacts

Currently supports Docker images as build artifacts. After successful `aenv build` execution, artifacts content is automatically updated:

```json
"artifacts": [
  {
    "id": "",
    "type": "image",
    "content": "reg.example.com/faas-swe/image:search-1.0.0-v3"
  }
]
```

> **Extension Note**: Additional artifact types are continuously being expanded.

#### Build Configuration (BuildConfig)

Supports custom build parameters including image name, tags, etc. These parameters can also be temporarily specified via the `aenv build` command line:

```json
{
  "buildConfig": {
    "dockerfile": "./Dockerfile",
    "context": ".",
    "image_name": "helloworld",
    "image_tag": "latest"
  }
}
```

#### Deployment Configuration (DeployConfig)

```json
{
  "deployConfig": {
    "cpu": "2",
    "memory": "4G",
    "os": "linux",
    "imagePrefix": "registry.example.com/envs",
    "podTemplate": "",
    "env": {
      "MODEL_PATH": "/models/llm"
    }
  }
}
```

**Deployment Parameters:**

- **cpu**: CPU specification, defaults to 2 cores
- **memory**: Memory specification, defaults to 4GB
- **os**: Operating system, currently only supports Linux
- **imagePrefix**: Image prefix used to constrain common prefixes for multiple images associated with the current environment
- **podTemplate**: Pod template, defaults to "singleContainer" (single container mode)
- **env**: Environment variable configuration

## Environment Architecture Types

### Single Container Environment

**Architecture Features**: Integrated design combining business logic and tool execution environments in a single container.

**Core Components:**
- **Business Logic Engine**: Handles core business processes and user requests
- **Built-in Toolset**: Pre-installed all required tool scripts and command-line tools
- **Shared Runtime Environment**: Business code and tool scripts share the same runtime environment
- **Integrated Data Storage**: Local storage for business data and temporary files

```mermaid
flowchart TB
    User["External User/System"]
    
    subgraph "Single Container Workflow"
        direction TB
        
        subgraph "Environment Instance"
            A["FastMCP Service Entry"]
            B["MCP Tool Calls"]
            C["Business Process Handling"]
            D["Local Runtime Environment"]
        end
        
        User -->|HTTP/API Request| A
        A --> B
        B --> C
        C -->|Direct Call| D
        D -->|Return Result| C
        C -->|Response Data| B
        B -->|Business Response| User
        
        classDef unifiedContainer fill:#9C27B0,stroke:#4A148C,color:white
        class UnifiedContainer unifiedContainer
    end
    
    style User fill:#FF9800,stroke:#E65100,color:white
```

**Use Cases:**
- Lightweight environment requirements
- Rapid prototyping development
- Resource-constrained scenarios
- Simple tool integration

### Dual Container Environment

**Architecture Features**: Adopts dual container deployment architecture by separating business logic from execution environments, achieving highly scalable and flexible AI agent runtime environments.

#### Architecture Components

**Main Container**
- Responsible for core business logic execution
- Handles user requests and business process orchestration
- Serves as the unified entry point for the environment

**Execution Container (Second Container)**
- Acts as independent data and runtime environment carrier
- Pre-installed with various tool scripts and dependency environments
- Provides isolated execution sandbox
- Focuses on tool calls and data processing tasks

#### Collaboration Mechanism

- **On-demand Invocation**: Main container dynamically invokes execution container when specific tools need execution
- **Environment Isolation**: Tool execution environment is completely isolated from main business logic, ensuring system stability
- **Resource Sharing**: Necessary data exchange through mounted volumes or network communication
- **Security Boundary**: Execution container runs under restricted permissions, reducing security risks

```mermaid
flowchart TB
    subgraph "Dual Container Workflow"
        direction TB
        
        subgraph "Environment Instance"
            subgraph "Main Container"
                A["FastMCP Service Entry"]
                B["MCP Tool Calls"]
                C["Business Process Handling"]
                D["Execution Container Caller"]
            end
            
            subgraph "Execution Container"
                E["Tool Script Collection"]
                F["Python Virtual Environment"]
                G["Data Storage"]
                H["Execution Sandbox"]
            end
        end
        
        Rollout["Training/Agent Framework"] -->|Function Call Request| A
        A --> B
        B --> C
        C --> D
        D -->|Call Instruction + Data Parameters| H
        H --> F
        F --> E
        E --> G
        G -->|Execution Result| H
        H -->|Return Result| D
        D --> C
        C -->|Response Data| B
        B -->|Tool Call Result| Rollout
    end
    
    style Rollout fill:#FF9800,stroke:#E65100,color:white
```

#### Dual Container Configuration Guide

**1. Modify Environment Configuration File**

```json
{
  "deployConfig": {
    "podTemplate": "DualContainer",
    "imagePrefix": "registry.example.com/envs"
  }
}
```

- **podTemplate**: Select templates supporting dual containers (e.g., `DualContainer`)
- **imagePrefix**: Constrains the common prefix for second container images associated with the current environment

**2. Specify Data Source When Creating Environment Instance**

```python
env = Environment(
    env_name="swe-env",
    datasource=""
)
```

`datasource` specifies the suffix for the current environment instance's second container image. The complete second container image is: `imagePrefix + datasource`

#### Usage Scenarios

**Scenario 1: Direct Full Image Path Specification**

When `datasource` contains the complete second container image path, `imagePrefix` configuration can be omitted:

```python
env = Environment(
    env_name="swe-env",
    datasource="swebench/sweb.eval.x86_64.{instance_id}_1776_p5.js-5771:latest"
)
# Full image name: swebench/sweb.eval.x86_64.{instance_id}_1776_p5.js-5771:latest
```

**Scenario 2: Image Concatenation Logic**

Sync images to internal repository, distinguishing different instances by tags:

```python
# Configure image prefix
imagePrefix = "custom_registry/swebench/common_name:"

# Only need to pass instance ID when creating environment
env = Environment(
    env_name="swe-env",
    datasource="{instance_id}"
)
# Full image name: custom_registry/swebench/common_name:{instance_id}
```

**Advantages:**
- All instances use the same image name, distinguished by different tags
- Simplifies environment creation process, only need to pass instance identifier
- Facilitates image version management and batch deployment

## Environment Instance Lifecycle

### Create Environment Instance

```python
from aenv import Environment

# Create environment instance
env = Environment(
    env_name="my-env",
    ttl="1h",
    environment_variables={"DEBUG": "true"}
)
```

### Use Environment

Environment instances can be integrated with various training frameworks for reinforcement learning training, or configured separately for agent use. Refer to the [examples](../examples/) directory for detailed usage.

### Destroy Environment Instance

```python
# Initialize environment (create container)
await env.initialize()

# Release resources after use
await env.release()
```

**Lifecycle Management:**
- **Default TTL**: 30 minutes automatic recycling
- **Active Release**: Explicit resource release via interface
- **Resource Optimization**: Recommend timely release to avoid resource waste

### Best Practices

```python
import asyncio
from aenv import Environment

async def managed_environment():
    """Use context manager to ensure proper resource cleanup"""
    async with Environment("my-env", ttl="2h") as env:
        # Environment automatically initialized
        tools = await env.list_tools()
        
        # Execute environment tasks
        result = await env.call_tool("analyze", {"data": "sample"})
        
        # Automatic destruction on exit
        return result

# Run example
asyncio.run(managed_environment())
