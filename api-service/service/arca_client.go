/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"api-service/models"
	backend "envhub/models"
)

// ArcaClient implements EnvInstanceService against the Arca sandbox OpenAPI
// (`/arca/openapi/v1/sandbox/*`).
//
// Unlike ScheduleClient / EnvInstanceClient / FaaSClient, ArcaClient does not
// assemble cpu/memory/disk/image — those are fully determined by the Arca
// template identified by DeployConfig["arcaTemplateId"]. List is not
// supported because Arca OpenAPI has no list endpoint in this iteration.
//
// Supported engines: arca.
type ArcaClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Compile-time interface compliance check.
var _ EnvInstanceService = (*ArcaClient)(nil)

// Arca OpenAPI paths (relative to baseURL).
const (
	arcaInstancesPath = "/arca/openapi/v1/sandbox/instances"
	arcaGatewayPrefix = "/arca/api/v1/sandbox"
	arcaAPIKeyHeader  = "x-agent-sandbox-api-key"
	arcaSandboxIDHdr  = "x-agent-sandbox-id"
	arcaTemplateIDHdr = "x-agent-sandbox-template-id"
	arcaPortHeader    = "x-agent-sandbox-port"
)

// DeployConfig keys consumed by ArcaClient.
const (
	deployKeyArcaTemplateID = "arcaTemplateId"
	deployKeyMountPoints    = "mountPoints"
	deployKeyEnvVars        = "environment_variables"
	deployKeyOwner          = "owner"
)

// Engine label key/value written onto returned EnvInstance.Labels.
const (
	engineLabelKey  = "engine"
	engineLabelArca = "arca"
)

// NewArcaClient constructs an ArcaClient targeting the given Arca OpenAPI base
// URL with the supplied tenant API key. The returned client is safe for
// concurrent use from multiple goroutines.
//
// Supported engines: arca.
func NewArcaClient(baseURL, apiKey string) *ArcaClient {
	return &ArcaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// arcaCreateRequest is the outbound body for
// POST /arca/openapi/v1/sandbox/instances.
//
// Supported engines: arca. Fields are chosen to match spec §3.2; notably
// `resource` and `image` are intentionally omitted because the Arca template
// fully determines them.
type arcaCreateRequest struct {
	TemplateID   string            `json:"template_id"`
	TTLInMinutes int               `json:"ttl_in_minutes,omitempty"`
	MountPoints  []interface{}     `json:"mount_points,omitempty"`
	Envs         map[string]string `json:"envs,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// arcaEnvelope matches Arca's uniform response wrapper.
type arcaEnvelope struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// arcaCreatedInstance is Arca's create response payload.
// Arca emits sandbox_id in both snake_case and camelCase; the snake form
// is historically stable and used here.
type arcaCreatedInstance struct {
	SandboxID string `json:"sandbox_id"`
}

// arcaSandboxInfo is Arca's GET /instances/{id} response payload (subset).
// Arca returns `podIp` only as camelCase (verified against stable env 2026-04).
// Supported engines: arca.
type arcaSandboxInfo struct {
	SandboxID string `json:"sandbox_id"`
	Status    string `json:"status"`
	PodIP     string `json:"podIp,omitempty"`
}

// mapArcaStatus converts Arca OpenAPI status strings into EnvInstance.Status.
// Unknown values fall back to Failed with a log warning.
//
// Supported engines: arca.
func mapArcaStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PENDING":
		return models.EnvInstanceStatusPending.String()
	case "RUNNING":
		return models.EnvInstanceStatusRunning.String()
	case "FAILED", "PAUSED":
		return models.EnvInstanceStatusFailed.String()
	case "DESTROYED":
		return models.EnvInstanceStatusTerminated.String()
	default:
		log.Warnf("arca: unknown sandbox status %q, mapping to Failed", s)
		return models.EnvInstanceStatusFailed.String()
	}
}

// ttlMinutesCeil parses a Go-style duration string (e.g. "30m", "1h", "90s")
// and returns ceil(duration / 1min). Empty input returns 0 with no error.
//
// Supported engines: arca.
func ttlMinutesCeil(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid ttl %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, nil
	}
	return int(math.Ceil(float64(d) / float64(time.Minute))), nil
}

// coerceMountPoints accepts the DeployConfig["mountPoints"] value and
// normalises it into a []interface{} ready for JSON serialisation. This
// tolerates the two common shapes: []interface{} (from JSON unmarshalling
// into map[string]interface{}) and []map[string]string (from programmatic
// controller writes).
func coerceMountPoints(raw interface{}) []interface{} {
	switch v := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		return v
	case []map[string]interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out
	case []map[string]string:
		out := make([]interface{}, len(v))
		for i, item := range v {
			copyMap := make(map[string]interface{}, len(item))
			for k, val := range item {
				copyMap[k] = val
			}
			out[i] = copyMap
		}
		return out
	default:
		log.Warnf("arca: mount_points has unexpected type %T, ignoring", raw)
		return nil
	}
}

// coerceEnvs reads DeployConfig["environment_variables"] and returns a
// map[string]string or nil. Non-string values are stringified via fmt.Sprint.
func coerceEnvs(raw interface{}) map[string]string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case map[string]string:
		if len(v) == 0 {
			return nil
		}
		return v
	case map[string]interface{}:
		if len(v) == 0 {
			return nil
		}
		out := make(map[string]string, len(v))
		for k, val := range v {
			out[k] = fmt.Sprint(val)
		}
		return out
	default:
		return nil
	}
}

// doJSON executes an HTTP request with the given method/path/body and decodes
// the Arca envelope into out. Non-2xx responses return an error carrying
// status code + body excerpt. Envelope-level `success=false` also errors.
func (c *ArcaClient) doJSON(method, path string, body interface{}, extraHeaders map[string]string, out interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("arca: marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return fmt.Errorf("arca: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(arcaAPIKeyHeader, c.apiKey)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("arca: %s %s: %w", method, path, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Warnf("arca: close response body: %v", cerr)
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("arca: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("arca: %s %s returned %d: %s", method, path, resp.StatusCode, truncateBody(raw))
	}

	var envelope arcaEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("arca: decode envelope: %w; body=%s", err, truncateBody(raw))
	}
	if !envelope.Success {
		return fmt.Errorf("arca: %s %s failed (code %d): %s", method, path, envelope.Code, envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 && !bytes.Equal(envelope.Data, []byte("null")) {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("arca: decode data: %w; body=%s", err, truncateBody(envelope.Data))
		}
	}
	return nil
}

// CreateEnvInstance creates a new Arca sandbox from the envhub Env's
// DeployConfig. Required key: `arcaTemplateId`. Returns an EnvInstance with
// `Labels[engine]="arca"`. The initial status is usually Pending; callers
// should poll GetEnvInstance until Running.
//
// Supported engines: arca.
func (c *ArcaClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	if req == nil {
		return nil, fmt.Errorf("arca: nil env")
	}
	if req.DeployConfig == nil {
		return nil, fmt.Errorf("arcaTemplateId required for arca engine (DeployConfig is nil)")
	}

	templateID, _ := req.DeployConfig[deployKeyArcaTemplateID].(string)
	if templateID == "" {
		return nil, fmt.Errorf("arcaTemplateId required for arca engine")
	}

	ttlMin, err := ttlMinutesCeil(req.GetTTL())
	if err != nil {
		return nil, fmt.Errorf("arca: %w", err)
	}

	body := arcaCreateRequest{
		TemplateID:   templateID,
		TTLInMinutes: ttlMin,
		MountPoints:  coerceMountPoints(req.DeployConfig[deployKeyMountPoints]),
		Envs:         coerceEnvs(req.DeployConfig[deployKeyEnvVars]),
	}

	owner, _ := req.DeployConfig[deployKeyOwner].(string)
	body.Metadata = map[string]string{
		"aenv_env_name":    req.Name,
		"aenv_env_version": req.Version,
	}
	if owner != "" {
		body.Metadata["aenv_owner"] = owner
	}

	var created arcaCreatedInstance
	headers := map[string]string{arcaTemplateIDHdr: templateID}
	if err := c.doJSON(http.MethodPost, arcaInstancesPath, body, headers, &created); err != nil {
		return nil, err
	}
	if created.SandboxID == "" {
		return nil, fmt.Errorf("arca: empty sandbox_id in create response")
	}

	inst := models.NewEnvInstanceWithOwner(created.SandboxID, req, "", owner)
	inst.TTL = req.GetTTL()
	inst.Labels = mergeLabelsWithEngine(req.DeployConfig)
	return inst, nil
}

// GetEnvInstance fetches sandbox detail by ID and maps it to EnvInstance with
// the arca engine label.
//
// Supported engines: arca.
func (c *ArcaClient) GetEnvInstance(id string) (*models.EnvInstance, error) {
	if id == "" {
		return nil, fmt.Errorf("arca: empty sandbox id")
	}
	path := arcaInstancesPath + "/" + id
	headers := map[string]string{arcaSandboxIDHdr: id}

	var info arcaSandboxInfo
	if err := c.doJSON(http.MethodGet, path, nil, headers, &info); err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return &models.EnvInstance{
		ID:        info.SandboxID,
		Status:    mapArcaStatus(info.Status),
		CreatedAt: now,
		UpdatedAt: now,
		IP:        info.PodIP,
		Labels:    map[string]string{engineLabelKey: engineLabelArca},
	}, nil
}

// DeleteEnvInstance releases an Arca sandbox.
//
// Supported engines: arca.
func (c *ArcaClient) DeleteEnvInstance(id string) error {
	if id == "" {
		return fmt.Errorf("arca: empty sandbox id")
	}
	path := arcaInstancesPath + "/" + id
	headers := map[string]string{arcaSandboxIDHdr: id}
	return c.doJSON(http.MethodDelete, path, nil, headers, nil)
}

// ListEnvInstances is intentionally unsupported for arca because Arca OpenAPI
// provides no list endpoint. Callers (e.g. cleanup_service) must tolerate the
// error gracefully.
//
// Supported engines: arca (always returns error).
func (c *ArcaClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	return nil, fmt.Errorf("arca: ListEnvInstances not supported")
}

// Warmup is permanently unsupported for arca (parity with ScheduleClient).
//
// Supported engines: arca (always returns error).
func (c *ArcaClient) Warmup(req *backend.Env) error {
	return fmt.Errorf("arca: Warmup not supported")
}

// arcaPresignTokenRequest is the body for POST /arca/api/v1/sandbox/{id}/presign/token.
type arcaPresignTokenRequest struct {
	ExpirationTime float64 `json:"expiration_time,omitempty"`
}

// arcaPresignTokenResponse is the unwrapped data of the presign envelope.
type arcaPresignTokenResponse struct {
	Token string `json:"token"`
}

// PresignURL acquires a short-lived URL pointing to an in-sandbox port via
// Arca's gateway. The returned URL is fully-qualified and can be used by the
// caller as a base for HTTP/MCP traffic, or by the api-service MCP proxy as
// a reverse-proxy target.
//
// Supported engines: arca.
func (c *ArcaClient) PresignURL(id string, port int, expirationMinutes float64) (string, error) {
	if id == "" {
		return "", fmt.Errorf("arca: empty sandbox id")
	}
	if port <= 0 {
		return "", fmt.Errorf("arca: port must be > 0")
	}
	path := fmt.Sprintf("%s/%s/presign/token", arcaGatewayPrefix, id)
	headers := map[string]string{
		arcaSandboxIDHdr: id,
		arcaPortHeader:   fmt.Sprintf("%d", port),
	}
	body := arcaPresignTokenRequest{ExpirationTime: expirationMinutes}

	var out arcaPresignTokenResponse
	if err := c.doJSON(http.MethodPost, path, body, headers, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("arca: empty presign token in response")
	}
	return c.baseURL + "/arca/api/v1/session/" + out.Token, nil
}

// mergeLabelsWithEngine combines user-supplied labels in DeployConfig["labels"]
// with the reserved `engine=arca` label. The engine key always wins.
func mergeLabelsWithEngine(deployConfig map[string]interface{}) map[string]string {
	out := map[string]string{engineLabelKey: engineLabelArca}
	if deployConfig == nil {
		return out
	}
	raw, ok := deployConfig["labels"]
	if !ok || raw == nil {
		return out
	}
	switch v := raw.(type) {
	case map[string]string:
		for k, val := range v {
			if k == engineLabelKey {
				continue
			}
			out[k] = val
		}
	case map[string]interface{}:
		for k, val := range v {
			if k == engineLabelKey {
				continue
			}
			out[k] = fmt.Sprint(val)
		}
	}
	return out
}
