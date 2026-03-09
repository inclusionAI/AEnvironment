# AEnvironment CLI Commands Reference

Quick reference for common aenv CLI commands.

## Configuration

```bash
# Set owner
aenv config set owner <owner-name>

# Set storage type (local or aenv_hub)
aenv config set storage_config.type local

# Set system URL
aenv config set system_url <api-service-url>

# Set EnvHub URL
aenv config set hub_config.hub_backend <envhub-url>

# Set registry configuration
aenv config set build_config.registry.host <registry-host>
aenv config set build_config.registry.username <username>
aenv config set build_config.registry.password <password>
aenv config set build_config.registry.namespace <namespace>

# Show current configuration
aenv config show
```

## Environment Management

```bash
# Initialize new environment
aenv init <env-name>

# Initialize config-only (for existing Dockerfile)
aenv init <env-name> --config-only

# Build Docker image
aenv build --platform linux/amd64 --push

# Register environment to EnvHub
aenv push

# List registered environments
aenv list

# Get environment details
aenv get <env-name>
```

## Instance Operations

```bash
# Create instance
aenv instance create <env-name>@<version> \
  --ttl <ttl> \
  --keep-alive \
  --skip-health \
  -e KEY=VALUE

# List instances
aenv instance list

# Get instance details
aenv instance get <instance-id>

# Delete instance
aenv instance delete <instance-id>
```

## Service Operations

```bash
# Create service
aenv service create <env-name>@<version> \
  --replicas <count> \
  --port <port> \
  --enable-storage \
  -e KEY=VALUE

# List services
aenv service list

# Get service details
aenv service get <service-id>

# Update service
aenv service update <service-id> --replicas <count>
aenv service update <service-id> -e KEY=VALUE

# Delete service
aenv service delete <service-id>
aenv service delete <service-id> --delete-storage
```

## Common Patterns

### Deploy Agent Instance

```bash
aenv instance create myagent@1.0.0 \
  --ttl 100h \
  --keep-alive \
  --skip-health \
  -e OPENAI_API_KEY="xxx" \
  -e OPENAI_MODEL="gpt-4"
```

### Deploy Web Service

```bash
aenv service create webapp@1.0.0 \
  --replicas 2 \
  --port 8080 \
  -e DATABASE_URL="postgres://..."
```

### Deploy Service with Storage

```bash
aenv service create dataapp@1.0.0 \
  --replicas 1 \
  --port 8081 \
  --enable-storage
```

## Environment Specification Format

Format: `<env-name>@<version>`

Examples:

- `myagent@1.0.0`
- `webapp@2.1.3`
- `stockagent@1.0.2`
