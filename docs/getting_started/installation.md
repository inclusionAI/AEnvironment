# Installation Guide

> Deploy AEnvironment development environment from scratch

## System Requirements

### Basic Dependencies

| Component | Version Requirement | Purpose |
|-----------|---------------------|---------|
| **Python** | 3.12+ | Runtime environment |
| **Package Manager** | pip ≥ 21.0 or uv | Dependency management |
| **Docker** | 20.10+ | Container runtime |

### Environment Check

```bash
# Verify Python version
python --version  # Should display 3.12+

# Verify Docker status
docker --version && docker ps

# Verify package manager
pip --version || uv --version
```

## SDK Installation

### Installation Methods Comparison

| Method | Use Case | Features | Command |
|--------|----------|----------|---------|
| **PyPI Install** | Production environment | Stable version | `pip install aenvironment` |
| **UV Install** | High performance needs | Ultra-fast dependency resolution | `uv pip install aenvironment` |
| **Source Install** | Development contribution | Latest features | `pip install -e .` |

### Standard Installation Process

#### 📦 PyPI Installation (Recommended)

```bash
# Create virtual environment (optional but recommended)
python -m venv aenv-env
source aenv-env/bin/activate  # Linux/Mac
# or
aenv-env\Scripts\activate     # Windows

# Install SDK
pip install aenvironment

# Verify installation
aenv version
```

#### ⚡ UV Installation (High Performance)

```bash
# Use UV package manager (install uv first)
uv venv aenv-env
source aenv-env/bin/activate
uv pip install aenvironment
```

#### 🔧 Source Installation (Developers)

```bash
# Clone source repository
git clone https://github.com/inclusionAI/AEnvironment.git
cd AEnvironment/aenv

# Development mode installation
pip install -e .

# Verify development environment
python -c "import aenv; print('Development environment ready')"
```

### Installation Verification

#### ✅ Basic Verification

```bash
# Check version information
aenv version
```

**Expected Output Example**:
```
AEnv SDK Version: 0.1.0
Environment: PyPI Package
Build Version: 0.1.0
Python version: 3.12.0
```

#### 🔍 Advanced Verification

```bash
# Test CLI functionality
aenv --help

# Check dependency completeness
python -c "import aenv.core; print('Core module loaded successfully')"
```

### Third-party Tool Installation

#### MCP Inspector Installation (Optional)

```bash
# Install MCP debugging tool
npm install -g @modelcontextprotocol/inspector

# Verify installation
mcp-inspector --version
```

> **💡 Use Case**: MCP Inspector serves as a visual debugging client, recommended for installation in local development environments

##### Usage

- By default, when you run `aenv run`, the MCP Inspector will be automatically launched to help you test and monitor your MCP server.
```shell
aenv run
```

## SDK Configuration

### 🛠️ Initialize Configuration

```bash
# Create configuration file
aenv config init

# View configuration path
echo "Configuration file location: $(aenv config path)"
```

### Configuration File Structure

#### Global Configuration

```json
{
  "global": {
    "global_mode": "local",
    "log_level": "INFO"
  },
  "build": {
    "type": "local",
    "build_args": {
      "socket": "unix:///var/run/docker.sock"
    },
    "registry": {
      "host": "docker.io",
      "username": "",
      "password": "",
      "namespace": "aenv"
    }
  },
  "storage": {
    "type": "local",
    "custom": {
      "prefix": "~/.aenv/envs"
    }
  },
  "hub_backend":{
    "hub_backend": "http://xxxx"
  }
}
```

### Configuration Management Operations

#### Hub Service Configuration

The Hub Service is a centralized metadata storage system for development environments. It works with `aenv list` and `aenv get` commands to manage and retrieve environment information.

##### Local Mode
No configuration required - uses default local storage.

##### Non-Local Mode

1. **Deploy Control Components**: First deploy the AEnv management components
2. **Get EnvHub Service Address**: Obtain the EnvHub service endpoint

```bash
# Configure Hub service address (required for non-local mode)
aenv config set hub_config.hub_backend http://localhost:8080

# Verify configuration
aenv config get hub_config
```

#### Build Configuration

| Configuration Item | Command Example | Description |
|-------------------|-----------------|-------------|
| **Docker Socket** | `aenv config set build.build_args.socket unix:///var/run/docker.sock` | Local Docker connection |
| **Image Registry** | `aenv config set build.registry.host docker.io` | Image push target |
| **Namespace** | `aenv config set build.registry.namespace my-team` | Image organization namespace |

#### Docker Socket Configuration Guide

<Tabs>
<TabItem value="linux" label="🐧 Linux System">

```bash
# Standard location check
ls -la /var/run/docker.sock

# Service status verification
sudo systemctl status docker | grep -i sock

# Environment variable confirmation
echo $DOCKER_HOST
```

</TabItem>
<TabItem value="macos" label="🍎 macOS System">

```bash
# Docker Desktop location
ls -la ~/.docker/run/docker.sock

# Context configuration check
docker context ls
docker context inspect default

# Detailed information view
docker info | grep -i "docker root dir"
```

</TabItem>
<TabItem value="universal" label="🔍 Universal Method">

```bash
# Global Docker socket search
sudo find / -name "docker.sock" 2>/dev/null

# Configuration check
docker version --format '{{json .Server}}' | jq -r '.DockerRootDir'

# Daemon configuration
cat /etc/docker/daemon.json | jq -r '.["hosts"]' 2>/dev/null
```

</TabItem>
</Tabs>

#### Image Registry Configuration

> **🔒 Security Note**: Image registry configuration is used for image pushing during the build process. Configure private registries according to actual needs

```bash
# View current configuration
aenv config get build_config.registry

# Configure private registry
aenv config set build_config.registry.host your-registry.com
aenv config set build_config.registry.username your-username
aenv config set build_config.registry.password your-password
aenv config set build_config.registry.namespace your-namespace
```

#### Storage Configuration

Storage configuration manages AEnvironment environment code with multi-version support:

```bash
# Configure storage path
aenv config set storage_config.custom.prefix ~/.aenv/environments

# View storage configuration
aenv config get storage_config
```

## Management Components Installation

### 🏠 Local Mode

**Features**: Zero dependencies, quick start, suitable for development debugging

```bash
# No additional components needed
aenv run

# Direct connection to local service
export DUMMY_INSTANCE_IP=http://localhost
```

### ☁️ Kubernetes Production Mode

**Features**: High availability, elastic scaling, enterprise deployment

#### Component Architecture

| Component | Purpose | Deployment Method |
|-----------|---------|-------------------|
| **Controller** | Environment lifecycle management | Deployment |
| **Redis** | State cache service | StatefulSet |
| **EnvHub** | Environment registry | Deployment |
| **API-Service** | API gateway | Deployment |

#### One-click Deployment

This repo provides an umbrella Helm chart (`deploy/`) that installs the full AEnv platform, including:

- **controller**: manages environment instance (Pod) lifecycle
- **redis**: cache/state backend
- **envhub**: environment registry
- **api-service**: API gateway

#### Prerequisites

```bash
# Verify Helm and kubectl
helm version
kubectl version --client

# Verify you can access the cluster
kubectl get nodes
```

#### Install all components

Option A: install from this repository (recommended for development / self-managed deployments):

```bash
helm install aenv-platform ./deploy \
  --namespace aenv \
  --create-namespace \
  --wait \
  --timeout 10m
```

Option B: install with a custom values file:

```bash
helm install aenv-platform ./deploy \
  -f deploy/values/prod.yaml \
  --namespace aenv \
  --create-namespace
```

#### Install from a Helm repository (optional)

If you maintain a Helm chart repository (or use an internal chart repo), you can install the umbrella chart by adding the repo first:

```bash
helm repo add aenv https://your-chart-repo.com
helm repo update

helm install aenv-platform aenv/aenv-platform \
  --namespace aenv \
  --create-namespace
```

#### Upgrade

```bash
helm upgrade aenv-platform ./deploy \
  --namespace aenv \
  --wait \
  --timeout 10m
```

#### Uninstall

```bash
helm uninstall aenv-platform --namespace aenv
```

#### Configuration

Enable/disable components in `values.yaml`:

```yaml
controller:
  enabled: true

redis:
  enabled: false
```

Customize namespace behavior:

```yaml
global:
  namespace: my-namespace
  createNamespace: true
```

Override sub-chart settings (replicas, image, resources, etc.):

```yaml
controller:
  enabled: true
  replicaCount: 3
  image: my-registry/controller:v1.0.0
  resources:
    limits:
      cpu: "4"
      memory: "8Gi"
```

#### Notes

- **Namespace**: The default namespace is `aenv`. The controller will also create/use the `aenvsandbox` namespace for environment instances.
- **Resources**: Ensure your cluster has sufficient CPU/memory/storage for your expected concurrency.
- **Image pulls**: If your images are in a private registry, configure `imagePullSecrets` accordingly.

#### Troubleshooting

```bash
# Check release status
helm status aenv-platform --namespace aenv

# List all resources
kubectl get all -n aenv

# Logs
kubectl logs -n aenv -l app=controller
kubectl logs -n aenv -l app=api-service
kubectl logs -n aenv -l app=envhub
```

#### For chart maintainers (optional)

If you are modifying the Helm chart in this repository, these commands help you manage dependencies and packaging:

```bash
# Update chart dependencies
helm dependency update deploy

# Inspect dependency list
helm dependency list deploy
```

To package the umbrella chart:

```bash
cd deploy
helm dependency update
helm package .
```

The packaging step produces an archive like `aenv-platform-0.1.0.tgz`, which can be pushed to your Helm registry.

#### Chart directory structure (deploy/)

```bash
deploy/
├── Chart.yaml
├── values.yaml
├── .helmignore
├── charts/
├── controller/
├── redis/
├── envhub/
└── api-service/
```


## Troubleshooting

### Getting Help

```bash
# View detailed help
aenv --help

# View failure details --verbose
aenv --verbose build
```

## Next Steps

After installation, we recommend continuing with:

1. **🚀 Quick Start** - {doc}`quickstart` - Create your first environment in 5 minutes
2. **📚 Deep Dive** - {doc}`concepts` - Understand core design principles
3. **🛠️ Hands-on Practice** - {doc}`../guide/sdk` - Master advanced SDK usage

---

<div align="center">

**🎯 Installation Complete!** You can now start building intelligent environments

[Get Started →](quickstart.md)

</div>

---

*This guide is released under the Apache 2.0 License*
