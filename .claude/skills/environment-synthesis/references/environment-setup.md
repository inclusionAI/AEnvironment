# Environment Setup Guide

## Directory Structure

A typical AEnvironment project has the following structure:

```
agent-name/
├── config.json          # Environment configuration
├── Dockerfile           # Container build instructions
├── requirements.txt     # Python dependencies
└── src/
    └── custom_env.py   # Tool definitions
```

## config.json Structure

```json
{
    "name": "agent-name",
    "version": "1.0.0",
    "tags": ["swe", "python", "linux"],
    "status": "Ready",
    "codeUrl": "oss://xxx",
    "artifacts": [
        {
            "type": "image",
            "content": "registry/agent-name:1.0.0"
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

## Dockerfile Template

```dockerfile
FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY src/ ./src/

CMD ["python", "-m", "aenv"]
```

## requirements.txt

Minimum requirements:

```
aenv>=0.1.0
openai>=1.0.0
```

Additional dependencies as needed for your tools.

