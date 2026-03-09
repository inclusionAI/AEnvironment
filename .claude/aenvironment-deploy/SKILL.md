---
name: aenvironment-deploy
description: Deploy sandboxed environment instances and services using AEnvironment. Use when deploying agent instances, web services, or applications to AEnvironment sandbox infrastructure. Supports three workflows - (1) Build image locally and deploy, (2) Register existing image and deploy, (3) Deploy from registered environment. Handles instance deployment (temporary, IP-based access for agents) and service deployment (persistent, domain-based access with storage for apps).
---

# AEnvironment Deploy

Automate deployment of sandboxed environment instances and services using the AEnvironment platform.

## Overview

AEnvironment provides isolated sandbox environments for running agents and applications. This skill handles the complete deployment workflow:

- **Instances**: Short-lived environments for agents (IP-based access, no persistence)
- **Services**: Long-running services for apps (domain access, optional storage, multiple replicas)

## Prerequisites

Install AEnvironment CLI:

```bash
pip install aenvironment --upgrade
aenv --help
```

## Deployment Workflows

### Workflow A: Deploy with Local Image Build

Build Docker image locally, register to EnvHub, and deploy.

**When to use**: Creating a new environment from scratch with local Dockerfile.

**Requirements**: Docker installed, registry credentials configured.

**Script**: `scripts/deploy_with_local_build.py`

**Example**:

```bash
python scripts/deploy_with_local_build.py \
  --env-name myagent \
  --owner-name john \
  --api-service-url https://api.example.com \
  --envhub-url https://envhub.example.com \
  --registry-host registry.example.com \
  --registry-username user \
  --registry-password pass \
  --registry-namespace myteam \
  --deploy-type instance \
  --ttl 24h \
  --env-vars '{"API_KEY":"xxx"}'
```

### Workflow B: Deploy with Existing Image

Register existing Docker image to EnvHub and deploy.

**When to use**: You have a pre-built Docker image to deploy.

**Requirements**: Existing image accessible in registry.

**Script**: `scripts/deploy_with_existing_image.py`

**Example**:

```bash
python scripts/deploy_with_existing_image.py \
  --env-name myagent \
  --image-name registry.example.com/myteam/agent:1.0.0 \
  --owner-name john \
  --api-service-url https://api.example.com \
  --envhub-url https://envhub.example.com \
  --deploy-type instance \
  --ttl 48h
```

### Workflow C: Deploy Existing Environment

Deploy from already registered environment in EnvHub.

**When to use**: Environment is already registered, just need to deploy.

**Requirements**: Environment registered in EnvHub.

**Script**: `scripts/deploy_existing_env.py`

**Example**:

```bash
python scripts/deploy_existing_env.py \
  --env-spec myagent@1.0.0 \
  --owner-name john \
  --api-service-url https://api.example.com \
  --envhub-url https://envhub.example.com \
  --deploy-type instance
```

## Instance vs Service

| Feature | Instance | Service |
|---------|----------|---------|
| **Lifecycle** | Temporary (with TTL) | Permanent |
| **Access** | IP + port | Service domain |
| **Replicas** | Single | Multiple supported |
| **Storage** | No | Optional (requires replicas=1) |
| **Use Case** | Agents, dev/test | Production apps, APIs |

## Common Parameters

### Required (All Workflows)

- `--env-name`: Environment name
- `--owner-name`: Resource owner identifier
- `--api-service-url`: AEnvironment API URL
- `--envhub-url`: EnvHub service URL
- `--deploy-type`: `instance` or `service`

### Instance Options

- `--ttl`: Time to live (default: "24h", examples: "30m", "48h", "100h")
- `--env-vars`: JSON dict of environment variables

### Service Options

- `--replicas`: Number of replicas (default: 1)
- `--port`: Service port (default: 8080)
- `--enable-storage`: Enable persistent storage (forces replicas=1)
- `--env-vars`: JSON dict of environment variables

### Registry Options (Workflow A only)

- `--registry-host`: Registry hostname
- `--registry-username`: Registry username
- `--registry-password`: Registry password
- `--registry-namespace`: Registry namespace

## Configuration

After initialization, edit `config.json` to customize:

```json
{
  "name": "myenv",
  "version": "1.0.0",
  "artifacts": [
    {
      "type": "image",
      "content": "registry.example.com/myimage:latest"
    }
  ],
  "requirements": {
    "cpu": "1000m",
    "memory": "2Gi"
  },
  "deployConfig": {
    "service": {
      "replicas": 1,
      "port": 8080,
      "enableStorage": false,
      "storageSize": "10Gi",
      "mountPath": "/data"
    }
  }
}
```

**See**: [references/CONFIG_SCHEMA.md](references/CONFIG_SCHEMA.md) for complete schema reference.

## Accessing Deployed Resources

### Instance Access

Instances provide IP-based access:

```text
http://<instance-ip>:<port>
```

Check instance details:

```bash
python -c "from scripts.aenv_operations import AEnvOperations; \
  ops = AEnvOperations(); \
  ops.configure_cli('owner', 'api-url', 'hub-url'); \
  print(ops.list_instances())"
```

### Service Access

Services provide domain-based access:

```text
http://<service-name>.aenv-sandbox.svc.tydd-staging.alipay.net:<port>
```

Check service details:

```bash
python -c "from scripts.aenv_operations import AEnvOperations; \
  ops = AEnvOperations(); \
  ops.configure_cli('owner', 'api-url', 'hub-url'); \
  print(ops.list_services())"
```

## Management Operations

### List Resources

```python
from scripts.aenv_operations import AEnvOperations

ops = AEnvOperations()
ops.configure_cli(owner_name, api_service_url, envhub_url)

# List environments
envs = ops.list_environments()

# List instances
instances = ops.list_instances()

# List services
services = ops.list_services()
```

### Delete Resources

```python
# Delete instance
ops.delete_instance(instance_id)

# Delete service (keep storage)
ops.delete_service(service_id)

# Delete service and storage
ops.delete_service(service_id, delete_storage=True)
```

## Reference Documentation

- **[CLI_COMMANDS.md](references/CLI_COMMANDS.md)**: Quick reference for aenv CLI commands
- **[CONFIG_SCHEMA.md](references/CONFIG_SCHEMA.md)**: config.json structure and fields
- **[TROUBLESHOOTING.md](references/TROUBLESHOOTING.md)**: Common issues and solutions

## Examples

### Deploy Agent Instance

```bash
python scripts/deploy_existing_env.py \
  --env-spec stockagent@1.0.2 \
  --owner-name trader-team \
  --api-service-url https://api.aenv.example.com \
  --envhub-url https://hub.aenv.example.com \
  --deploy-type instance \
  --ttl 100h \
  --env-vars '{
    "NEWS_API_KEY":"xxx",
    "OPENAI_MODEL":"gpt-4",
    "OPENAI_API_KEY":"xxx"
  }'
```

### Deploy Web Service with Storage

```bash
python scripts/deploy_existing_env.py \
  --env-spec webapp@2.0.0 \
  --owner-name dev-team \
  --api-service-url https://api.aenv.example.com \
  --envhub-url https://hub.aenv.example.com \
  --deploy-type service \
  --replicas 1 \
  --port 8081 \
  --enable-storage \
  --env-vars '{"DB_HOST":"postgres.svc"}'
```

### Build and Deploy New Environment

```bash
python scripts/deploy_with_local_build.py \
  --env-name newagent \
  --owner-name my-team \
  --api-service-url https://api.aenv.example.com \
  --envhub-url https://hub.aenv.example.com \
  --registry-host registry.example.com \
  --registry-username user \
  --registry-password pass \
  --registry-namespace myteam \
  --deploy-type instance \
  --platform linux/amd64
```

## Error Handling

All scripts include automatic retry logic (2 retries by default). Common errors:

- **Environment not found**: Verify environment is registered with `aenv list`
- **Registry auth failed**: Check registry credentials
- **Storage with multiple replicas**: Set `--replicas 1` when using `--enable-storage`
- **Build timeout**: Optimize Dockerfile, use smaller base images

See [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) for detailed solutions.

## Best Practices

1. **Use semantic versioning** for environments (1.0.0, 1.0.1, 1.1.0, 2.0.0)
2. **Configure resources appropriately** in config.json (CPU, memory)
3. **Use Workflow C for repeated deployments** (most efficient)
4. **Set appropriate TTL for instances** (longer for long-running agents)
5. **Enable storage only when needed** (limits replicas to 1)
6. **Use environment variables** for configuration instead of hardcoding
7. **Delete unused resources** to free cluster capacity

## Core Library

The `scripts/aenv_operations.py` module provides the core operations used by all workflows. You can import and use it directly for custom workflows:

```python
from scripts.aenv_operations import AEnvOperations, AEnvError

try:
    ops = AEnvOperations(verbose=True)

    # Configure CLI
    ops.configure_cli(
        owner_name="my-team",
        api_service_url="https://api.example.com",
        envhub_url="https://hub.example.com"
    )

    # Create instance
    result = ops.create_instance(
        env_spec="myenv@1.0.0",
        ttl="24h",
        env_vars={"KEY": "value"}
    )

    print(f"Instance created: {result['instance_id']}")
    print(f"Access at: {result['ip_address']}")

except AEnvError as e:
    print(f"Deployment failed: {e}")
```
