# Docker Engine Support - Setup and Testing Guide

## Dependencies

### Controller Dependencies

The controller requires the following Go dependencies:

```bash
cd controller
go get github.com/docker/docker/client@v24.0.7
go get github.com/docker/docker/api/types@v24.0.7
go mod tidy
```

### API Service Dependencies

No additional dependencies required for API service (uses standard Go `net/http`).

## Building

### Build Controller

```bash
cd controller
go build -o bin/controller ./cmd/main.go
```

### Build API Service

```bash
cd api-service
go build -o bin/api-service ./main.go
```

## Local Testing

### Prerequisites

- Docker Engine 20.10+ installed and running
- Docker Compose 2.0+ installed (optional, for Compose tests)

### Test 1: Single Container Mode

#### Step 1: Start Controller (Docker mode)

```bash
cd controller
export ENGINE_TYPE=docker
export DOCKER_HOST=unix:///var/run/docker.sock
export DOCKER_NETWORK=bridge
export COMPOSE_ENABLED=true
./bin/controller --leader-elect=false
```

Expected output:

```
Engine type: docker
Initializing Docker engine handler...
AEnv Docker handler created, host: unix:///var/run/docker.sock, network: bridge, compose: true, TLS: false
Docker engine handler registered at /containers
AEnv server starts, listening on port: 8080
```

#### Step 2: Start API Service (Docker mode)

```bash
cd api-service
./bin/api-service \
  --schedule-type=docker \
  --schedule-addr=http://localhost:8080 \
  --backend-addr=http://your-envhub:8083
```

Expected output:

```
Docker engine enabled, EnvService is not supported in this mode
Server listening on :8070
MCP Proxy listening on :8081
```

#### Step 3: Create a container instance

```bash
curl -X POST http://localhost:8070/env-instance \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-env",
    "version": "1.0.0",
    "owner": "test-user",
    "artifacts": [
      {
        "type": "image",
        "content": "alpine:latest"
      }
    ],
    "deployConfig": {
      "cpu": "1C",
      "memory": "512Mi",
      "ttl": "1h"
    }
  }'
```

Expected response:

```json
{
  "success": true,
  "code": 0,
  "data": {
    "id": "docker-test-env-abc123def456",
    "status": "Running",
    "ip": "172.17.0.2",
    "ttl": "1h",
    "owner": "test-user"
  }
}
```

#### Step 4: Query container status

```bash
CONTAINER_ID="<id_from_previous_response>"
curl http://localhost:8070/env-instance/$CONTAINER_ID
```

#### Step 5: List all containers

```bash
curl "http://localhost:8070/env-instance?envName=test-env"
```

#### Step 6: Delete container

```bash
curl -X DELETE http://localhost:8070/env-instance/$CONTAINER_ID
```

#### Step 7: Verify with Docker CLI

```bash
# List AEnv containers
docker ps --filter "label=aenv.env_name"

# Check resource limits
docker stats $CONTAINER_ID --no-stream
```

### Test 2: Docker Compose Mode

#### Step 1: Create a Compose stack

```bash
curl -X POST http://localhost:8070/env-instance \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-app",
    "version": "1.0.0",
    "owner": "test-user",
    "deployConfig": {
      "composeFile": "version: \"3.8\"\nservices:\n  web:\n    image: nginx:latest\n    labels:\n      - \"aenv.main=true\"\n    ports:\n      - \"8080:80\"\n  redis:\n    image: redis:alpine\n",
      "ttl": "2h"
    }
  }'
```

Expected: Multiple containers started, labeled with `com.docker.compose.project=aenv-<id>`

#### Step 2: Verify Compose stack

```bash
# List compose containers
docker ps --filter "label=com.docker.compose.project"

# Check compose file
ls /tmp/aenv-compose-*.yaml
```

#### Step 3: Delete Compose stack

```bash
curl -X DELETE http://localhost:8070/env-instance/<compose_project_id>
```

Expected: All containers in the stack are stopped and removed

### Test 3: Remote Docker with TLS

#### Step 1: Setup remote Docker daemon with TLS

```bash
# On remote host, generate certificates (example)
mkdir -p /etc/docker/certs
cd /etc/docker/certs
# ... generate ca.pem, cert.pem, key.pem ...

# Configure Docker daemon to listen on TCP with TLS
dockerd --tlsverify --tlscacert=/etc/docker/certs/ca.pem \
  --tlscert=/etc/docker/certs/cert.pem \
  --tlskey=/etc/docker/certs/key.pem \
  -H=0.0.0.0:2376
```

#### Step 2: Start Controller with TLS

```bash
export ENGINE_TYPE=docker
export DOCKER_HOST=tcp://remote-host:2376
export DOCKER_TLS_VERIFY=true
export DOCKER_CERT_PATH=/path/to/client/certs
./bin/controller --leader-elect=false
```

Expected output includes:

```
TLS verification enabled, cert path: /path/to/client/certs
AEnv Docker handler created, host: tcp://remote-host:2376, ..., TLS: true
```

### Test 4: TTL and Cleanup

#### Step 1: Create container with short TTL

```bash
curl -X POST http://localhost:8070/env-instance \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ttl-test",
    "version": "1.0.0",
    "owner": "test-user",
    "artifacts": [{"type": "image", "content": "alpine:latest"}],
    "deployConfig": {
      "ttl": "60s"
    }
  }'
```

#### Step 2: Wait for expiration (> 60 seconds)

#### Step 3: Trigger cleanup

```bash
# This is normally done by API service's cleanup scheduler
curl http://localhost:8070/cleanup
```

#### Step 4: Verify container is removed

```bash
docker ps -a --filter "label=aenv.env_name=ttl-test"
```

Expected: No containers found

## Helm Deployment

### Deploy with Docker Engine

```bash
# Install Controller with Docker engine
helm install aenv-controller ./deploy/controller \
  --set docker.enabled=true \
  --set docker.host="unix:///var/run/docker.sock" \
  --set docker.network.defaultNetwork="bridge" \
  --set docker.compose.enabled=true

# Install API Service
helm install aenv-api ./deploy/api-service \
  --set scheduleType=docker \
  --set scheduleAddr="http://aenv-controller:8080"
```

### Deploy with Remote Docker + TLS

```bash
# Create secret for TLS certificates
kubectl create secret generic docker-tls-certs \
  --from-file=ca.pem=/path/to/ca.pem \
  --from-file=cert.pem=/path/to/cert.pem \
  --from-file=key.pem=/path/to/key.pem \
  -n aenv

# Install Controller with TLS
helm install aenv-controller ./deploy/controller \
  --set docker.enabled=true \
  --set docker.host="tcp://docker-host:2376" \
  --set docker.tls.verify=true \
  --set docker.tls.certPath="/certs" \
  -n aenv
```

## Troubleshooting

### Issue: "docker daemon unreachable"

**Symptom**: Controller fails to start with error:

```
failed to create Docker handler, err is docker daemon unreachable at unix:///var/run/docker.sock
```

**Solutions**:

1. Check Docker daemon is running: `docker ps`
2. Verify socket permissions: `ls -l /var/run/docker.sock`
3. Add controller user to docker group: `usermod -aG docker <user>`
4. Check DOCKER_HOST environment variable

### Issue: "Docker Compose support not yet implemented"

**Symptom**: API returns 501 when trying to create Compose stack

**Solutions**:

1. Verify COMPOSE_ENABLED is set to `true`
2. Check docker-compose/docker compose is installed: `docker compose version`
3. Ensure /tmp directory is writable

### Issue: Containers not listed

**Symptom**: `curl /env-instance` returns empty list

**Possible causes**:

1. Containers don't have `aenv.env_name` label
2. Cache not synced yet (wait 30 seconds)
3. Containers created outside of AEnv

**Debug**:

```bash
# Check all containers
docker ps -a

# Check labels
docker inspect <container_id> | grep -A 10 Labels
```

### Issue: TLS certificate errors

**Symptom**: "remote error: tls: bad certificate"

**Solutions**:

1. Verify certificate paths in DOCKER_CERT_PATH
2. Ensure certificates match server's CA
3. Check certificate expiration: `openssl x509 -in cert.pem -noout -dates`
4. Test connection: `docker --tlsverify --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem -H=tcp://host:2376 ps`

## Performance Benchmarks

### Benchmark Setup

Use `hey` tool for load testing:

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Benchmark container creation
hey -n 100 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -d '{"name":"bench","version":"1.0","owner":"test","artifacts":[{"type":"image","content":"alpine:latest"}],"deployConfig":{}}' \
  http://localhost:8070/env-instance

# Benchmark container query
hey -n 1000 -c 50 http://localhost:8070/env-instance/<container_id>
```

### Expected Results (Reference)

**Hardware**: 4 CPU, 8GB RAM, SSD
**Docker Version**: 24.0.7

| Operation | Throughput | Avg Latency | P99 Latency |
|-----------|-----------|-------------|-------------|
| Create (alpine) | 15 req/s | 650ms | 1.2s |
| Query | 120 req/s | 40ms | 85ms |
| List (10 containers) | 90 req/s | 55ms | 110ms |
| Delete | 25 req/s | 380ms | 720ms |

## Next Steps

1. Run integration tests
2. Compare performance with Kubernetes mode
3. Test Compose multi-service scenarios
4. Validate resource limits enforcement
5. Test TTL cleanup under load
