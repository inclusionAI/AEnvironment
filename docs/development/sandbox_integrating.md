# Custom Sandbox Engine Integration Guide

This document explains how to integrate a custom sandbox engine into your system using two supported approaches. Follow
the instructions and code examples below based on your requirements.

---

## Overview

The system provides two integration paths for custom sandbox engines:

- **Option 1 (Standard)**: Adapt your existing sandbox API to the predefined `EnvInstanceClient` interface.
- **Option 2 (Custom)**: Implement the `EnvInstanceService` interface directly in your own client, allowing full control
  over API interactions.

Both methods require implementing CRUD operations and lifecycle management for sandbox instances. The core interface is
defined as:

```go
type EnvInstanceService interface {
GetEnvInstance(id string) (*models.EnvInstance, error)
CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error)
DeleteEnvInstance(id string) error
ListEnvInstances(envName string) ([]*models.EnvInstance, error)
Warmup(req *backend.Env) error
Cleanup() error
}
```

Configuration is controlled by the `schedule_type` flag in your `main` function:

- `schedule_type: standard` → Uses `EnvInstanceClient`
- `schedule_type: custom` → Uses your implementation of `EnvInstanceService`

---

## Option 1: Adapt Existing API via `EnvInstanceClient`

Use this approach if your sandbox API can be adapted to match the expected request/response format of
`EnvInstanceClient`.

### Step 1: Configure Base URL

Initialize the client with your sandbox engine’s base URL, which can be specified by `schedule_addr` flag in
`api-service` startup command:

```go
client := service.NewEnvInstanceClient("https://your-sandbox-api.com")
```

### Step 2: Ensure API Compatibility

Your sandbox service must expose the following endpoints:

| Method   | URI Pattern                                | Description                        |
|----------|--------------------------------------------|------------------------------------|
| `POST`   | `/aenvironment/instance`                   | Create a new environment instance  |
| `GET`    | `/aenvironment/instance/{id}`              | Get instance by ID                 |
| `DELETE` | `/aenvironment/instance/{id}`              | Delete instance by ID              |
| `GET`    | `/aenvironment/instance?envName={envName}` | List instances by environment name |
| `PUT`    | `/aenvironment/instance/action/warmup`     | Warm up environment resources      |
| `PUT`    | `/aenvironment/instance/action/cleanup`    | Cleanup unused resources           |

---

### API Details & Examples

#### 1. Create Environment Instance

**Endpoint**: `POST /aenvironment/instance`  
**Request Body**:

```json
{
  "id": "env-123",
  "name": "Python3.9-RL",
  "description": "Reinforcement Learning sandbox",
  "version": "v1.2",
  "tags": [
    "ml",
    "rl"
  ],
  "code_url": "https://github.com/user/rl-env.git",
  "build_config": {
    "image": "python:3.9-rl",
    "timeout": 600
  },
  "test_config": {
    "command": "pytest tests/"
  },
  "deploy_config": {
    "replicas": 1,
    "resources": {
      "cpu": "2",
      "memory": "4Gi"
    }
  }
}
```

**Success Response (200/201)**:

```json
{
  "success": true,
  "code": 200,
  "message": "Instance created",
  "data": {
    "id": "inst-789",
    "env": {
      /* same as request env */
    },
    "status": "Running",
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z",
    "ip": "10.244.1.10",
    "ttl": "2h"
  }
}
```

---

#### 2. Get Environment Instance

**Endpoint**: `GET /aenvironment/instance/{id}`

**Response**:

```json
{
  "success": true,
  "code": 200,
  "data": {
    "id": "inst-789",
    "env": {
      /* env config */
    },
    "status": "Running",
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:30:00Z",
    "ip": "10.244.1.10",
    "ttl": "2h"
  }
}
```

---

#### 3. Delete Environment Instance

**Endpoint**: `DELETE /aenvironment/instance/{id}`  
**Response**:

```json
{
  "success": true,
  "code": 200,
  "data": true
}
```

---

#### 4. List Instances by Environment Name

**Endpoint**: `GET /aenvironment/instance?envName=Python3.9-RL`  
**Response**:

```json
{
  "success": true,
  "code": 200,
  "data": [
    {
      "id": "inst-789",
      "env": {
        "name": "Python3.9-RL"
      },
      "status": "Running",
      "ip": "10.244.1.10",
      "created_at": "2025-01-15T10:00:00Z"
    }
  ]
}
```

---

#### 5. Warmup (Prepare Resources)

**Endpoint**: `PUT /aenvironment/instance/action/warmup`  
**Request**: Same body format as `CreateEnvInstance` (sent in body or inferred by service).  
**Response**: Same as `GetEnvInstance` (containing warmed-up instance).

---

#### 6. Cleanup (Release Unused Resources)

**Endpoint**: `PUT /aenvironment/instance/action/cleanup`  
**Response**: Success or error message. Implementation may scan and delete expired instances.

---

### Step 3: Initialize with Standard Mode

In your `main.go`:

```go
func main() {
// Load config where schedule_type is set
config := loadConfig()

var envSvc service.EnvInstanceService

if config.ScheduleType == "standard" {
envSvc = service.NewEnvInstanceClient(config.SandboxBaseURL)
} else if config.ScheduleType == "custom" {
envSvc = NewCustomSandboxClient(config) // See Option 2
}

// Pass envSvc to scheduler or manager
scheduler := NewScheduler(envSvc)
scheduler.Start()
}
```

> ✅ **Pros**: Quick integration if API matches.  
> ⚠️ **Cons**: Requires adapting your API to expected URL, payload, and response formats.

---

## Option 2: Implement Custom Client via `EnvInstanceService`

Use this if your sandbox API differs significantly in endpoints, parameters, or response structure.

### Step 1: Define Your Custom Client

Implement all methods of `EnvInstanceService`:

```go
type CustomSandboxClient struct {
baseURL    string
httpClient *http.Client
apiKey     string // Example: custom auth
}

func NewCustomSandboxClient(baseURL, apiKey string) *CustomSandboxClient {
return &CustomSandboxClient{
baseURL: baseURL,
apiKey:  apiKey,
httpClient: &http.Client{
Timeout: 45 * time.Second,
},
}
}
```

---

### Step 2: Implement Required Methods (Example: Create)

```go
func (c *CustomSandboxClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
// Custom endpoint and payload
url := fmt.Sprintf("%s/v2/envs/launch", c.baseURL)

payload := map[string]interface{}{
"template": req.Name,
"git":      req.CodeURL,
"resources": req.DeployConfig,
}

jsonData, _ := json.Marshal(payload)
httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
httpReq.Header.Set("X-API-Key", c.apiKey)
httpReq.Header.Set("Content-Type", "application/json")

resp, err := c.httpClient.Do(httpReq)
if err != nil {
return nil, err
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
if resp.StatusCode != 201 {
return nil, fmt.Errorf("custom create failed: %s", body)
}

// Parse custom response format
var result struct {
SandboxID string `json:"sandbox_id"`
IP        string `json:"endpoint_ip"`
Status    string `json:"state"`
}
if err := json.Unmarshal(body, &result); err != nil {
return nil, err
}

return &models.EnvInstance{
ID:     result.SandboxID,
Env:    req,
Status: result.Status,
IP:     result.IP,
}, nil
}
```

> You **must** implement all 6 interface methods.

---

### Step 3: Wire Custom Client in Main

```go
func main() {
var scheduleClient service.EnvInstanceService
if scheduleType == "k8s" {
scheduleClient = service.NewScheduleClient(scheduleAddr)
} else if scheduleType == "standard" {
scheduleClient = service.NewEnvInstanceClient(scheduleAddr)
} else if scheduleType == "custom" {
scheduleClient = service.NewCustomSandboxClient(scheduleAddr, "your-api-key")
} else {
log.Fatalf("unsupported schedule type: %v", scheduleType)
}

}
```

> ✅ **Pros**: Full flexibility in API design, headers, auth, and response parsing.  
> ⚠️ **Requirement**: You must handle all error cases, timeouts, and data mapping.

---

## Required Data Structures

### `backend.Env` (Input for Creation/Warmup)

```json
{
  "id": "env-123",
  "name": "Python3.9",
  "description": "Standard Python env",
  "version": "v1.0",
  "tags": [
    "ai",
    "stable"
  ],
  "code_url": "https://github.com/org/repo.git",
  "build_config": {
    "image": "python:3.9"
  },
  "test_config": {
    "command": "make test"
  },
  "deploy_config": {
    "cpu": "2",
    "memory": "4Gi"
  }
}
```

### `models.EnvInstance` (Return Type)

```json
{
  "id": "inst-abc123",
  "env": {
    /* Env object */
  },
  "status": "Running",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:05:00Z",
  "ip": "192.168.1.100",
  "ttl": "1h"
}
```

### `models.ClientResponse[T]`

All responses **must** follow this envelope format:

```json
{
  "success": true,
  "code": 200,
  "message": "OK",
  "data": {
    /* T payload */
  }
}
```

> If your API uses a different envelope, transform it in your custom client methods.

---

## Recommendations & Best Practices

1. **Error Handling**  
   Always return descriptive errors including HTTP status and response body (truncated if large).

2. **Timeout Configuration**  
   Set reasonable timeouts (e.g., 30–60s) based on sandbox startup time.

3. **Idempotency**  
   Ensure `DeleteEnvInstance` is idempotent—returning success if resource is already gone is acceptable.

4. **Logging**  
   Log key operations (e.g., `Creating sandbox for env=Python3.9`), especially in `Cleanup`.

5. **Health Checks**  
   Consider adding `/health` endpoint validation during client initialization.

6. **Testing**  
   Provide a mock implementation of `EnvInstanceService` for unit testing business logic.

---

## Troubleshooting

| Symptom                | Likely Cause                                  | Solution                                                   |
|------------------------|-----------------------------------------------|------------------------------------------------------------|
| `404 Not Found`        | Incorrect base URL or endpoint path           | Verify full URL construction in client                     |
| `400 Bad Request`      | Mismatched request payload                    | Ensure `backend.Env.ToJSON()` matches expected schema      |
| `unmarshal error`      | Response format differs                       | In custom client, map your response to `ClientResponse[T]` |
| `Cleanup does nothing` | `FilterPods` not implemented or returns empty | Implement filtering logic or return mock data              |

---

## Summary

- **Standard Integration**: Fast path if your API matches expected routes/payloads.
- **Custom Integration**: Implement `EnvInstanceService` fully for maximum control.
- Set `schedule_type: standard` or `custom` in config to select mode.
- All responses **must** conform to `ClientResponse[T]` structure or be adapted.

For further details, refer to:

- `env_instance.go` – Standard HTTP client implementation
- `schedule_client.go` – Adapter example (`ScheduleClient` wraps pod ops into `EnvInstanceService`)
- `models/` – Shared data structures

Contact the platform team if you need assistance adapting your sandbox API.
