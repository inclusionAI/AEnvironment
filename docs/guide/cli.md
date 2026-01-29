# AEnvironment CLI User Guide

This guide provides comprehensive documentation for all AEnvironment CLI commands and features, helping you efficiently manage AEnvironment projects and environments.

## 📋 Command Overview

```text
Usage: aenv [OPTIONS] COMMAND [ARGS]...

  Aenv CLI helps build your custom aenv

Options:
  --debug
  -v, --verbose  Enable verbose output
  --help         Show this message and exit.

Commands:
  build      Build Docker images with real-time progress display.
  config     Manage CLI configuration.
  get        Get specified environment details
  init       Initialize aenv project using environmental scaffolding tools
  instance   Manage environment instances (create, list, get, delete)
  instances  List running environment instances (legacy command)
  list       List all environments with pagination
  push       Push current aenv project to remote backend aenv hub
  release    Release current aenv project
  service    Manage environment services (create, list, get, delete, update)
  test       Test current aenv project by running local directly
  version    Display version number and corresponding build/commit information
```

| Command Category | Command | Description |
|---|---|---|
| **Project Lifecycle** | `init` | Initialize new project |
| | `build` | Build container images |
| | `run` | Validate environment configuration |
| | `push` | Push environment to repository |
| **Environment Management** | `list` | List all environments |
| | `get` | Get environment details |
| **Instance Management** | `instance` | Manage environment instances |
| | `instances` | List running instances (legacy) |
| **Service Management** | `service` | Manage environment services |
| **Configuration Management** | `config` | Manage CLI configuration |
| **System Information** | `version` | Display version information |

## 🚀 Quick Start

### Typical Workflow

```bash
# 1. Create new project
aenv init my-search-env --template default

# 2. Validate configuration
aenv run --work-dir ./my-search-env

# 3. Build image
aenv build --work-dir ./my-search-env --image-tag v1.0.0

# 4. Push to repository
aenv push --work-dir ./my-search-env --version 1.0.0

# 5. Release environment
aenv release --work-dir ./my-search-env
```

## 🛠️ Project Lifecycle Commands

### `aenv init` - Initialize Project

Create a new AEnvironment project with support for multiple templates.

#### Basic Usage

```text
Usage: aenv init [OPTIONS] NAME

  Initialize aenv project using scaffolding tools

  NAME: aenv name

  Examples:
    aenv init myproject --version 1.0.0
    aenv init myproject --template default --work-dir ./myproject --force
    aenv init myproject --version 1.0.0 --config-only

Options:
  -v, --version TEXT        Specify aenv version number
  -t, --template [default]  Scaffolding template selection
  -w, --work-dir TEXT       Working directory for initialization
  --force                   Force overwrite existing directory
  --config-only             Only create config.json file, skip other files and directories
  --help                    Show this message and exit.
```

#### Available Templates

| Template Name | Description | Use Cases |
|---|---|---|
| `default` | Basic template | General environments |

#### Project Structure

```text
my-project/
├── config.json          # Environment configuration
├── Dockerfile          # Container definition
├── requirements.txt    # Python dependencies
├── src/               # Source code
│   ├── __init__.py
│   └── custom_env.py
├── test/              # Test files
└── README.md          # Project documentation
```

- **config.json**: Defines environment metadata including build configuration, test scripts, deployment resource requirements, and version information

<details>
<summary>Configuration Example</summary>

```json
{
  "name": "aenv",
  "version": "1.0.0",
  "tags": ["linux"],
  "status": "Ready",
  "codeUrl": "",
  "artifacts": [],
  "buildConfig": {
    "dockerfile": "./Dockerfile"
  },
  "testConfig": {
    "script": ""
  },
  "deployConfig": {
    "cpu": "1",
    "memory": "2Gi",
    "os": "linux",
    "ephemeralStorage": "5Gi",
    "environmentVariables": {},
    "service": {
      "replicas": 1,
      "port": 8081,
      "enableStorage": false,
      "storageName": "aenv",
      "storageSize": "10Gi",
      "mountPath": "/home/admin/data"
    }
  }
}
```

> **Note**: This is the default template structure. Additional fields like `imagePrefix` and `podTemplate` can be added as needed for specific deployment scenarios.

</details>

- **Dockerfile**

The final deliverable is primarily a container image, with the Dockerfile defining base configuration, dependency installation, and environment variable setup.

- **src**

Environment business logic code is placed in the src directory. Use `@register` decorators to register code as corresponding tools, functions, and rewards.

#### Config-Only Mode

Use `--config-only` flag to create only the `config.json` file without generating other project files and directories. This is useful when you already have a project structure and only need the configuration file.

```bash
# Create only config.json in current directory
aenv init myproject --version 1.0.0 --config-only

# Create config.json in specified directory
aenv init myproject --version 1.0.0 --config-only --work-dir ./myproject

# Force overwrite existing config.json
aenv init myproject --version 1.0.0 --config-only --force
```

> **Note**: When using `--config-only`, the `config.json` content is loaded from the template file, ensuring consistency with the full initialization process.

### `aenv run` - Validate Environment

After code development, use the run command for local validation.

#### Run Basic Usage

<details>
<summary>Usage Details</summary>

```text
Usage: aenv run [OPTIONS]

  Test current aenv project by running local directly

  Examples:
    aenv run --work-dir /tmp/aenv/search

Options:
  --work-dir TEXT           Specify aenv development root directory
  --inspector-port INTEGER  MCP Inspector port
  --help                    Show this message and exit.
```

</details>

```bash
# Validate current directory
aenv run

# Specify working directory
aenv run --work-dir ./my-project
```

<details>
<summary>Output Example</summary>

```text
ℹ️  🔍 Starting aenv project validation...
   Working Directory: /AEnvironment/aenv/hello
   Inspector Port: 6274

ℹ️  📁 Validating working environment...
✅ ✅ Working environment validation passed
ℹ️  🔧 Checking dependencies...
✅ ✅ Dependency check passed
ℹ️  📦 Installing MCP Inspector...
MCP Inspector Is Installed...!
✅ ✅ MCP Inspector installation completed
ℹ️  🚀 Starting MCP server and Inspector...
   Press Ctrl+C to stop services

2025-12-09 17:18:21,875 - mcp_manager - INFO - 🚀 Starting MCP server and Inspector...
2025-12-09 17:18:21,875 - mcp_manager - INFO - Starting task: mcp_server - python -m aenv.
```

</details>

#### Validation Checks

- ✅ Configuration file format validation
- ✅ Code correctness verification
- ✅ Dependency completeness check
- ✅ MCP service functionality verification

### `aenv build` - Build Images

Build Docker container images with real-time progress display.

#### Build Basic Usage

<details>
<summary>Usage Details</summary>

```text
Usage: aenv build [OPTIONS]

  Build Docker images with real-time progress display.

  This command builds Docker images from your project and provides real-time
  progress updates with beautiful UI components.

  Examples:
    aenv build
    aenv build --image-name myapp --image-tag v1.0
    aenv build --work-dir ./myproject --registry myregistry.com
    aenv build --work-dir ./build --dockerfile ./Dockerfile.prod

Options:
  -w, --work-dir DIRECTORY  Docker build context directory (defaults to current directory)
  -n, --image-name TEXT     Name for the Docker image
  -t, --image-tag TEXT      Tags for the Docker image (can be used multiple
                            times)
  -r, --registry TEXT       Docker registry URL
  -s, --namespace TEXT      Namespace for the Docker image
  --push / --no-push        Push image to registry after build
  -p, --platform TEXT       Platform for the Docker image
  -f, --dockerfile TEXT     Path to the Dockerfile (relative to work-dir, defaults to Dockerfile)
  --help                    Show this message and exit.
```

</details>

```bash
# Basic build
aenv build

# Specify working directory
aenv build --work-dir ./my-project

# Custom image name
aenv build --image-name my-custom-env

# Multiple tags
aenv build --image-tag v1.0.0 --image-tag latest

# Specify platform
aenv build --platform linux/amd64,linux/arm64

# Specify custom Dockerfile
aenv build --work-dir ./build --dockerfile ./Dockerfile.prod
```

#### Advanced Options

| Option | Short | Description | Example |
|---|---|---|---|
| `--work-dir` | `-w` | Docker build context directory | `--work-dir ./project` |
| `--image-name` | `-n` | Image name | `--image-name search-tool` |
| `--image-tag` | `-t` | Image tags | `--image-tag v1.0.0` |
| `--registry` | `-r` | Registry URL | `--registry registry.company.com` |
| `--namespace` | `-s` | Namespace | `--namespace myteam` |
| `--platform` | `-p` | Target platform | `--platform linux/amd64` |
| `--push` |  | Push after build | `--push` |
| `--dockerfile` | `-f` | Path to Dockerfile (relative to work-dir) | `--dockerfile ./Dockerfile.prod` |

#### Important Notes

- **config.json location**: The `config.json` file must be in the current working directory (where you run the command), not in the `--work-dir` directory.
- **work-dir purpose**: The `--work-dir` option specifies the Docker build context directory, which is used as the base path for Dockerfile and all files referenced in it (COPY, ADD, etc.).
- **Dockerfile path**: The `--dockerfile` path is relative to `--work-dir`. If not specified, defaults to `Dockerfile` in the `--work-dir` directory.

#### Build Output Example

![aenv build output log](../images/cli/aenv_build_log.png)

### `aenv push` - Push Environment

Push the built environment to a remote repository.

#### Push Basic Usage

```bash
# Push current environment
aenv push

# Specify version
aenv push --version 1.0.0

# Specify working directory
aenv push --work-dir ./my-project

# Push to specific registry
aenv push --registry registry.company.com
```

#### Push Options

| Option | Description | Example |
|---|---|---|
| `--version` | Specify version number | `--version 1.0.0` |
| `--registry` | Target registry | `--registry registry.company.com` |
| `--namespace` | Namespace | `--namespace myteam` |
| `--force` | Force overwrite | `--force` |

## 📊 Environment Management Commands

### `aenv list` - List Environments

Display all available AEnvironment environments.

#### List Basic Usage

```bash
# List all environments
aenv list

# Table format display
aenv list --format table

# JSON format output
aenv list --format json

# Pagination display
aenv list --limit 20 --offset 0

# Filter by name
aenv list --name search

# Filter by tags
aenv list --tags python,search
```

#### Output Format

```bash
$ aenv list --format table
+---------------------------+------------+---------------+-------------------------------------+
| Name                      | Version    | Description   | Created At                          |
+===========================+============+===============+=====================================+
| swebench-env              | 1.0.2      |               | 2025-12-08T14:21:04.143332845+08:00 |
+---------------------------+------------+---------------+-------------------------------------+
| test-search-env           | 1.0.0      |               | 2025-11-27T11:41:42.212945201+08:00 |
+---------------------------+------------+---------------+-------------------------------------+
| mini-swe-agent-env        | 1.1.1      |               | 2025-11-24T23:21:35.882220449+08:00 |
+---------------------------+------------+---------------+-------------------------------------+
```

### `aenv instances` - List Running Instances

List running environment instances with detailed information.

#### Instances Basic Usage

```bash
# List all running instances
aenv instances

# List instances for a specific environment
aenv instances --name my-env

# List instances for a specific environment and version
aenv instances --name my-env --version 1.0.0

# Output as JSON
aenv instances --format json

# Use custom system URL
aenv instances --system-url http://api.example.com:8080
```

#### Instances Options

| Option | Short | Description | Example |
|---|---|---|---|
| `--name` | `-n` | Filter by environment name | `--name test01` |
| `--version` | `-v` | Filter by environment version (requires --name) | `--version 1.0.0` |
| `--format` | `-f` | Output format (table or json) | `--format json` |
| `--system-url` |  | AEnv system URL (defaults to AENV_SYSTEM_URL env var) | `--system-url http://localhost:8080` |

#### Instances Output Format

```bash
$ aenv instances
+---------------+---------------+-----------+----------+------+----------------------+
| Instance ID   | Environment   | Version   | Status   | IP   | Created At           |
+===============+===============+===========+==========+======+======================+
| test01-tb9gp8 | test01        | 1.0.0     | Running  | -    | 2026-01-12T03:25:46Z |
+---------------+---------------+-----------+----------+------+======================+
| test01-ccp5v4 | test01        | 1.0.0     | Running  | -    | 2026-01-12T03:25:50Z |
+---------------+---------------+-----------+----------+------+----------------------+
```

#### Environment Variables

You can configure the API URL using environment variables:

```bash
# Set API URL (must include port 8080)
export AENV_SYSTEM_URL=http://api-service:8080

# Set API key (if authentication is enabled)
export AENV_API_KEY=your-token-here

# Then run the command
aenv instances
```

#### Instances Notes

- **API URL**: The CLI automatically uses port 8080 for the system URL. If you provide a URL with a different port, it will be adjusted.
- **Authentication**: If token authentication is enabled, set the `AENV_API_KEY` environment variable
- **Instance ID Format**: Instance IDs typically follow the pattern `environment-name-randomid` (e.g., `test01-tb9gp8`)
- **Filtering**: Use `--name` to filter by environment name, optionally combined with `--version` for more specific filtering

### `aenv instance` - Manage Environment Instances

Unified interface for managing environment instances with full lifecycle control.

> **Note**: `aenv instance` is the new command group that replaces and extends the legacy `aenv instances` command with additional capabilities like creating and deleting instances.

#### Instance Subcommands

| Subcommand | Description |
|---|---|
| `instance create` | Create new environment instance |
| `instance list` | List running instances |
| `instance get` | Get instance details by ID |
| `instance delete` | Delete an instance |
| `instance info` | Get instance information (requires active instance) |

#### `instance create` - Create Instance

Create and deploy a new environment instance.

**Basic Usage:**

```bash
# Create using config.json in current directory
aenv instance create

# Create with explicit environment name
aenv instance create flowise-xxx@1.0.2

# Create with custom TTL and environment variables
aenv instance create flowise-xxx@1.0.2 --ttl 1h -e DB_HOST=localhost -e DB_PORT=5432

# Create with arguments and skip health check
aenv instance create flowise-xxx@1.0.2 --arg --debug --arg --verbose --skip-health

# Create and keep alive (doesn't auto-release)
aenv instance create flowise-xxx@1.0.2 --keep-alive
```

**Options:**

| Option | Short | Description | Default |
|---|---|---|---|
| `--datasource` | `-d` | Data source for mounting | - |
| `--ttl` | `-t` | Time to live (e.g., 30m, 1h) | 30m |
| `--env` | `-e` | Environment variables (KEY=VALUE) | - |
| `--arg` | `-a` | Command line arguments | - |
| `--system-url` | | AEnv system URL | from env/config |
| `--timeout` | | Request timeout in seconds | 60.0 |
| `--startup-timeout` | | Startup timeout in seconds | 500.0 |
| `--max-retries` | | Maximum retry attempts | 10 |
| `--api-key` | | API key for authentication | from env |
| `--skip-health` | | Skip health check | false |
| `--output` | `-o` | Output format (table/json) | table |
| `--keep-alive` | | Keep instance running | false |
| `--owner` | | Owner of the instance | from config |

**Important Notes:**

- By default, instances are automatically released when the command exits
- Use `--keep-alive` to keep the instance running after deployment
- Without `--keep-alive`, the instance lifecycle is tied to the command execution

#### `instance list` - List Instances

List running environment instances with filtering options.

```bash
# List all running instances
aenv instance list

# List instances for a specific environment
aenv instance list --name my-env

# List instances for a specific environment and version
aenv instance list --name my-env --version 1.0.0

# Output as JSON
aenv instance list --output json
```

#### `instance get` - Get Instance Details

Retrieve detailed information about a specific instance by its ID.

```bash
# Get instance information
aenv instance get flowise-xxx-abc123

# Get in JSON format
aenv instance get flowise-xxx-abc123 --output json
```

#### `instance delete` - Delete Instance

Delete a running environment instance.

```bash
# Delete an instance (with confirmation)
aenv instance delete flowise-xxx-abc123

# Delete without confirmation
aenv instance delete flowise-xxx-abc123 --yes
```

### `aenv service` - Manage Environment Services

Manage long-running environment services with persistent deployments, multiple replicas, and storage support.

> **Service vs Instance**: Services are persistent deployments without TTL, supporting multiple replicas, persistent storage, and cluster DNS service URLs. Instances are temporary with TTL and auto-cleanup.

#### Service Subcommands

| Subcommand | Description |
|---|---|
| `service create` | Create new service deployment |
| `service list` | List running services |
| `service get` | Get service details by ID |
| `service delete` | Delete a service |
| `service update` | Update service configuration |

#### `service create` - Create Service

Create a long-running service with Deployment, Service, and optionally persistent storage.

**Basic Usage:**

```bash
# Create using config.json in current directory
aenv service create

# Create with explicit environment name
aenv service create myapp@1.0.0

# Create with custom service name
aenv service create myapp@1.0.0 --service-name my-custom-service

# Create with 3 replicas and custom port (no storage)
aenv service create myapp@1.0.0 --replicas 3 --port 8000

# Create with storage enabled (storageSize must be in config.json)
aenv service create myapp@1.0.0 --enable-storage

# Create with environment variables
aenv service create myapp@1.0.0 -e DB_HOST=postgres -e CACHE_SIZE=1024
```

**Options:**

| Option | Short | Description | Default |
|---|---|---|---|
| `--service-name` | `-s` | Custom service name (must follow Kubernetes DNS naming conventions) | auto-generated as `{envName}-svc-{random}` |
| `--replicas` | `-r` | Number of replicas | 1 or from config |
| `--port` | `-p` | Service port | 8080 or from config |
| `--env` | `-e` | Environment variables (KEY=VALUE) | - |
| `--enable-storage` | | Enable persistent storage | false |
| `--output` | `-o` | Output format (table/json) | table |

**Configuration Priority:**

1. CLI parameters (`--replicas`, `--port`, `--enable-storage`)
2. `config.json`'s `deployConfig.service` (new structure)
3. `config.json`'s `deployConfig` (legacy flat structure)
4. System defaults

**Storage Configuration:**

Storage settings are read from `config.json`'s `deployConfig.service`:

- `storageSize`: Storage size like "10Gi", "20Gi" (required when `--enable-storage` is used)
- `storageName`: Storage name (default: environment name)
- `mountPath`: Mount path (default: /home/admin/data)

**Important Notes:**

- When storage is enabled, replicas must be 1 (enforced by backend)
- Services run indefinitely without TTL
- Services get cluster DNS service URLs for internal access
- **Service Name**: Custom service names must follow Kubernetes DNS naming conventions:
  - Use only lowercase letters, numbers, hyphens, and dots
  - Start and end with an alphanumeric character
  - Be no longer than 253 characters
  - Example: `my-service`, `app-v1`, `web-frontend-prod`
  - If not specified, auto-generated as `{envName}-svc-{random}` (e.g., `myapp-svc-abc123`)
- The service name becomes:
  - Kubernetes Deployment name
  - Kubernetes Service name
  - Service URL prefix: `{serviceName}.{namespace}.{domain}:{port}`
  - Unique identifier for all operations (get, update, delete)

#### `service list` - List Services

List running environment services.

```bash
# List all services
aenv service list

# List services for specific environment
aenv service list --name myapp

# Output as JSON
aenv service list --output json
```

**Output Example:**

```bash
$ aenv service list
+------------------+-------------+---------+--------+----------+----------+--------------+----------------------+
| Service ID       | Environment | Owner   | Status | Replicas | Storage  | Service URL  | Created At           |
+==================+=============+=========+========+==========+==========+==============+======================+
| myapp-svc-abc123 | myapp@1.0.0 | user1   | Ready  | 3/3      | myapp-pvc| myapp-svc... | 2026-01-19T10:30:00Z |
+------------------+-------------+---------+--------+----------+----------+--------------+----------------------+
```

#### `service get` - Get Service Details

Retrieve detailed information about a specific service.

```bash
# Get service information
aenv service get myapp-svc-abc123

# Get in JSON format
aenv service get myapp-svc-abc123 --output json
```

#### `service delete` - Delete Service

Delete a running service. By default, keeps storage for reuse.

```bash
# Delete a service (with confirmation), keep storage
aenv service delete myapp-svc-abc123

# Delete without confirmation
aenv service delete myapp-svc-abc123 --yes

# Delete service and storage
aenv service delete myapp-svc-abc123 --delete-storage
```

**Options:**

| Option | Description |
|---|---|
| `--yes`, `-y` | Skip confirmation prompt |
| `--delete-storage` | Also delete associated storage (WARNING: permanent data loss) |

**Important Notes:**

- By default, storage is kept for reuse when deleting a service
- Use `--delete-storage` to permanently delete storage and all data

#### `service update` - Update Service

Update a running service's configuration.

```bash
# Scale to 5 replicas
aenv service update myapp-svc-abc123 --replicas 5

# Update environment variables
aenv service update myapp-svc-abc123 -e DB_HOST=newhost -e DB_PORT=3306

# Update multiple things at once
aenv service update myapp-svc-abc123 --replicas 3 -e DB_HOST=newhost
```

**Options:**

| Option | Short | Description |
|---|---|---|
| `--replicas` | `-r` | Update number of replicas |
| `--env` | `-e` | Environment variables (KEY=VALUE) |
| `--output` | `-o` | Output format (table/json) |

**Important Notes:**

- At least one of `--replicas` or `--env` must be provided
- Environment variable updates merge with existing variables

### `aenv get` - Get Environment Details

Retrieve detailed information about a specific environment.

#### Get Basic Usage

```bash
# Get environment details
aenv get search-env

# Specify version
aenv get search-env --version 1.0.0

# JSON format output
aenv get search-env --format json
```

#### Get Output Example

```json
{
  "id": "search-1.0.0",
  "name": "search",
  "description": "",
  "version": "1.0.0",
  "tags": [
    "swe",
    "python",
    "linux"
  ],
  "code_url": "~/.aenv/search-code.tar.gz",
  "status": 6,
  "artifacts": [
    {
      "id": "",
      "type": "image",
      "content": "docker.io/xxxx/image:search-1.0.0-v3"
    }
  ],
  "build_config": {
    "dockerfile": "./Dockerfile"
  },
  "test_config": {
    "script": "pytest xxx"
  },
  "deploy_config": {
    "cpu": "1C",
    "memory": "2G",
    "os": "linux"
  }
}
```

> **Note**: Requires non-local mode to use.

## ⚙️ Configuration Management Commands

### `aenv config` - Configuration Management

Manage CLI configuration and settings.

#### Subcommands

##### `config show` - Display Configuration

```bash
# Show all configurations
aenv config show

# JSON format
aenv config show --format json
```

##### `config get` - Get Configuration

```bash
# Get specific value
aenv config get global_mode
```

##### `config init` - Initialize Configuration

```bash
# Create default configuration
aenv config init
```

## 🔍 System Information Commands

### `aenv version` - Version Information

Display CLI version and build information.

#### Version Basic Usage

```bash
# Show version only
aenv version -s

# JSON format
aenv version --format json
```

#### Version Output Example

```bash
$ aenv version
AEnv CLI Version: 0.1.0
Build Date: 2024-01-15
Git Commit: abc123def
Go Version: go1.21.0
Platform: linux/amd64
Python Version: 3.12.0
```

## 🎯 Practical Usage Examples

### Scenario 1: Developing New Environment

```bash
# 1. Create search tool environment
aenv init search-tool --template search

# 2. Modify configuration
cd search-tool
# Edit config.json and Dockerfile

# 3. Implement business logic
@register_tool
@register_function
@register_reward
```

### Scenario 2: CI/CD Integration

```bash
# Use in CI scripts
#!/bin/bash
set -e

# Validate environment
aenv run --work-dir ./project

# Build and push
aenv build --work-dir ./project \
           --image-tag ${BUILD_NUMBER} \
           --registry registry.company.com \
           --push

# Publish
aenv push
```

## 📚 Quick Reference

### Common Command Combinations

```bash
# Complete workflow
aenv init → aenv run → aenv build → aenv push

# Environment management
aenv list → aenv get → aenv push

# Instance management
aenv instances → aenv instances --name <env> → aenv instances --format json

# Configuration management
aenv config path → aenv config set → aenv config show
```

### Debug Commands

```bash
# Verbose output
aenv --verbose <command>
```

## 🔗 Related Resources

- [Installation Guide](../getting_started/installation.md) - Detailed installation steps
- [Quick Start](../getting_started/quickstart.md) - Get started in 5 minutes
