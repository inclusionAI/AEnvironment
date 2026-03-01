# Config.json Schema Reference

Environment configuration file structure and key fields.

## Basic Structure

```json
{
  "name": "myenv",
  "version": "1.0.0",
  "description": "Environment description",
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
      "storageName": "myenv",
      "storageSize": "10Gi",
      "mountPath": "/home/admin/data"
    }
  }
}
```

## Key Fields

### Basic Information

- **name**: Environment name (required)
- **version**: Semantic version (required)
- **description**: Human-readable description

### Artifacts

Defines the Docker image to use:

```json
"artifacts": [
  {
    "type": "image",
    "content": "registry.example.com/namespace/image:tag"
  }
]
```

### Requirements

Resource requirements:

```json
"requirements": {
  "cpu": "1000m",      // CPU request/limit (e.g., "500m", "2")
  "memory": "2Gi"      // Memory request/limit (e.g., "512Mi", "4Gi")
}
```

Common values:

- CPU: "100m", "500m", "1", "2"
- Memory: "128Mi", "512Mi", "1Gi", "2Gi", "4Gi"

### Deploy Configuration (Service)

Service-specific deployment settings:

```json
"deployConfig": {
  "service": {
    "replicas": 1,              // Number of replicas (1 if storage enabled)
    "port": 8080,               // Service port
    "enableStorage": false,     // Enable persistent storage
    "storageName": "myenv",     // PVC name (default: env name)
    "storageSize": "10Gi",      // Storage size
    "mountPath": "/data"        // Mount path in container
  }
}
```

## Complete Example

```json
{
  "name": "stockagent",
  "version": "1.0.2",
  "description": "Stock trading agent environment",
  "artifacts": [
    {
      "type": "image",
      "content": "registry.example.com/agents/stockagent:1.0.2"
    }
  ],
  "requirements": {
    "cpu": "2",
    "memory": "4Gi"
  },
  "deployConfig": {
    "service": {
      "replicas": 1,
      "port": 8080,
      "enableStorage": true,
      "storageName": "stockagent-data",
      "storageSize": "20Gi",
      "mountPath": "/home/admin/data"
    }
  }
}
```

## Storage Configuration Notes

- **ReadWriteOnce**: When `enableStorage: true`, `replicas` must be 1
- **storageName**: Defaults to environment name if not specified
- **storageSize**: Cannot be reduced after creation
- **mountPath**: Path where storage will be mounted in container

## Version Guidelines

Use semantic versioning:

- MAJOR: Incompatible API changes
- MINOR: Backward-compatible functionality additions
- PATCH: Backward-compatible bug fixes

Examples:

- `1.0.0`: Initial release
- `1.0.1`: Bug fix
- `1.1.0`: New feature
- `2.0.0`: Breaking change
