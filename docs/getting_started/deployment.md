# Deployment Guide

This guide provides instructions for deploying AEnvironment in different deployment modes.

## Overview

AEnvironment supports multiple deployment modes for different use cases:

- **Kubernetes Mode**: Production deployment on Kubernetes clusters (Available)
- **Docker Compose Mode**: Single-node deployment for development and testing (Planning)
- **Other Modes**: Cloud provider managed services, serverless, edge deployment (Planning)

## Kubernetes Mode

Production deployment on Kubernetes clusters with high availability and scalability.

### Prerequisites

- Kubernetes cluster (v1.24+)
- Helm 3.x
- kubectl configured to access your cluster

### Quick Installation

```bash
helm install aenv-platform ./deploy \
  --namespace aenv \
  --create-namespace \
  --wait \
  --timeout 10m
```

### Components

| Component | Purpose |
|-----------|---------|
| **Controller** | Environment lifecycle management |
| **Redis** | State cache service |
| **EnvHub** | Environment registry |
| **API-Service** | API gateway |

### Configuration

#### Basic Configuration

Enable/disable components in `values.yaml`:

```yaml
controller:
  enabled: true
redis:
  enabled: true
envhub:
  enabled: true
api-service:
  enabled: true
```

#### Custom Values

Install with custom values:

```bash
helm install aenv-platform ./deploy \
  -f deploy/values/prod.yaml \
  --namespace aenv \
  --create-namespace
```

### Upgrade & Uninstall

```bash
# Upgrade
helm upgrade aenv-platform ./deploy \
  --namespace aenv \
  --wait \
  --timeout 10m

# Uninstall
helm uninstall aenv-platform --namespace aenv
```

### Verify Installation

```bash
# Check status
helm status aenv-platform --namespace aenv

# List resources
kubectl get all -n aenv

# Check logs
kubectl logs -n aenv -l app=controller
kubectl logs -n aenv -l app=api-service
```

### Service Endpoints

After deployment, services are available at:

- **API Service**: `http://api-service.aenv.svc.cluster.local:8080`
- **MCP Gateway**: `http://api-service.aenv.svc.cluster.local:8081`
- **EnvHub**: `http://envhub.aenv.svc.cluster.local:8083`
- **Controller**: `http://controller.aenv.svc.cluster.local:8080`

### Configure Client

```bash
# Set EnvHub endpoint
aenv config set hub_backend http://envhub.aenv.svc.cluster.local:8083/

# Set API service URL
export AENV_SYSTEM_URL=http://api-service.aenv.svc.cluster.local:8080/
```

### High Availability

Controller supports HA through Kubernetes leader election:

```yaml
controller:
  replicaCount: 3
  leaderElection:
    enabled: true
```

### Notes

- Default namespace: `aenv`
- Controller creates/uses `aenvsandbox` namespace for environment instances
- Ensure cluster has sufficient CPU/memory/storage resources
- Configure `imagePullSecrets` for private registries

## Docker Compose Mode (Planning)

Single-node deployment for development and testing using Docker Compose.

**Planned Features:**
- Quick local setup
- All components in one stack
- Suitable for development and testing
- Easy to start/stop

**Status**: Coming soon

## Other Deployment Modes (Planning)

Additional deployment options planned for future releases:

- **Cloud Provider Managed Services**: AWS ECS, Azure Container Instances, GCP Cloud Run
- **Serverless Mode**: Function-as-a-Service deployment
- **Edge Deployment**: Lightweight deployment for edge computing scenarios

**Status**: Coming soon

