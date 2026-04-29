/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"api-service/models"
	backend "envhub/models"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// arcaMock captures every request the ArcaClient makes, for assertions.
type arcaMock struct {
	server       *httptest.Server
	requests     []*capturedRequest
	nextResponse func(req *http.Request) (status int, body string)
}

type capturedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func newArcaMock(t *testing.T, respond func(*http.Request) (int, string)) *arcaMock {
	t.Helper()
	m := &arcaMock{nextResponse: respond}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.requests = append(m.requests, &capturedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Body:    body,
		})
		status, resp := m.nextResponse(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, resp)
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *arcaMock) client() *ArcaClient {
	return NewArcaClient(m.server.URL, "test-key")
}

// okResponse wraps data into the standard Arca envelope.
func okResponse(dataJSON string) string {
	return fmt.Sprintf(`{"success":true,"code":200,"message":"success","data":%s}`, dataJSON)
}

// failResponse returns a success=false envelope.
func failResponse(code int, msg string) string {
	return fmt.Sprintf(`{"success":false,"code":%d,"message":%q,"data":null}`, code, msg)
}

// lastRequest returns the most recent captured request or fails the test.
func (m *arcaMock) lastRequest(t *testing.T) *capturedRequest {
	t.Helper()
	if len(m.requests) == 0 {
		t.Fatal("no requests captured")
	}
	return m.requests[len(m.requests)-1]
}

// decodeBody unmarshals the captured request body into a generic map.
func decodeBody(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode body: %v; raw=%s", err, raw)
	}
	return out
}

// sampleEnv constructs a backend.Env with the provided DeployConfig overrides.
func sampleEnv(dc map[string]interface{}) *backend.Env {
	if dc == nil {
		dc = map[string]interface{}{}
	}
	// caller must explicitly set arcaTemplateId when needed
	return &backend.Env{
		Name:         "my-env",
		Version:      "1.0",
		DeployConfig: dc,
	}
}

// ---------------------------------------------------------------------------
// skeleton-level tests (preserved from Task 1)
// ---------------------------------------------------------------------------

// TestNewArcaClient_Defaults verifies constructor wiring.
//
// Supported engines: arca.
func TestNewArcaClient_Defaults(t *testing.T) {
	c := NewArcaClient("http://example:8080", "test-key")
	if c == nil {
		t.Fatal("NewArcaClient returned nil")
	}
	if c.baseURL != "http://example:8080" {
		t.Errorf("baseURL = %q, want http://example:8080", c.baseURL)
	}
	if c.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want test-key", c.apiKey)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 30s", c.httpClient.Timeout)
	}
}

// TestNewArcaClient_TrimsTrailingSlash ensures we don't double-slash when
// users supply a trailing slash in --arca-base-url.
//
// Supported engines: arca.
func TestNewArcaClient_TrimsTrailingSlash(t *testing.T) {
	c := NewArcaClient("http://example:8080/", "k")
	if c.baseURL != "http://example:8080" {
		t.Errorf("baseURL = %q, want http://example:8080", c.baseURL)
	}
}

// TestArcaClient_SatisfiesEnvInstanceService is a compile-time check via
// `var _ EnvInstanceService = (*ArcaClient)(nil)` in arca_client.go.
//
// Supported engines: arca.
func TestArcaClient_SatisfiesEnvInstanceService(t *testing.T) {
	var _ EnvInstanceService = (*ArcaClient)(nil)
}

// ---------------------------------------------------------------------------
// FC-API-01: Create happy path
// ---------------------------------------------------------------------------

func TestArcaCreate_HappyPath(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-123"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"ttl":            "30m",
	})

	inst, err := c.CreateEnvInstance(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst == nil || inst.ID != "sb-123" {
		t.Fatalf("want sandbox id sb-123, got %+v", inst)
	}
	if inst.Status != models.EnvInstanceStatusPending.String() {
		t.Errorf("status = %q, want Pending", inst.Status)
	}
	if inst.Labels["engine"] != "arca" {
		t.Errorf("engine label = %q, want arca", inst.Labels["engine"])
	}

	req := m.lastRequest(t)
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/arca/openapi/v1/sandbox/instances" {
		t.Errorf("path = %s", req.Path)
	}
	if req.Headers.Get("x-agent-sandbox-template-id") != "tpl1" {
		t.Errorf("x-agent-sandbox-template-id = %q, want tpl1", req.Headers.Get("x-agent-sandbox-template-id"))
	}

	body := decodeBody(t, req.Body)
	if body["template_id"] != "tpl1" {
		t.Errorf("template_id = %v", body["template_id"])
	}
	if fmt.Sprint(body["ttl_in_minutes"]) != "30" {
		t.Errorf("ttl_in_minutes = %v", body["ttl_in_minutes"])
	}
	if _, ok := body["resource"]; ok {
		t.Errorf("body unexpectedly contains resource: %v", body["resource"])
	}
	if _, ok := body["image"]; ok {
		t.Errorf("body unexpectedly contains image: %v", body["image"])
	}
}

// ---------------------------------------------------------------------------
// FC-API-02: missing arcaTemplateId
// ---------------------------------------------------------------------------

func TestArcaCreate_MissingTemplateID_Returns400(t *testing.T) {
	called := int32(0)
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		atomic.AddInt32(&called, 1)
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-999"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{}) // no arcaTemplateId
	inst, err := c.CreateEnvInstance(env)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "arcaTemplateId required for arca engine") {
		t.Errorf("error = %q, want substring 'arcaTemplateId required for arca engine'", err)
	}
	if inst != nil {
		t.Errorf("expected nil instance, got %+v", inst)
	}
	if atomic.LoadInt32(&called) != 0 {
		t.Error("expected no HTTP call on validation failure")
	}
}

// ---------------------------------------------------------------------------
// FC-API-03: mount_points passthrough
// ---------------------------------------------------------------------------

func TestArcaCreate_MountPointsPassthrough(t *testing.T) {
	mp := []interface{}{
		map[string]interface{}{
			"id":         "OSS_bucket_ak",
			"remote_dir": "/data/oss",
			"local_dir":  "/workspace/oss",
		},
	}
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-mp"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"mountPoints":    mp,
	})
	if _, err := c.CreateEnvInstance(env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeBody(t, m.lastRequest(t).Body)
	got, _ := json.Marshal(body["mount_points"])
	want, _ := json.Marshal(mp)
	if string(got) != string(want) {
		t.Errorf("mount_points = %s, want %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// FC-API-05: TTL conversion (table-driven)
// ---------------------------------------------------------------------------

func TestArcaCreate_TTLConversion(t *testing.T) {
	cases := []struct {
		raw        string
		wantMin    int
		wantOmit   bool
		wantErrSub string
	}{
		{"30m", 30, false, ""},
		{"90s", 2, false, ""},
		{"1h", 60, false, ""},
		{"", 0, true, ""},
		{"bad", 0, false, "invalid ttl"},
	}
	for _, tc := range cases {
		t.Run("ttl="+tc.raw, func(t *testing.T) {
			m := newArcaMock(t, func(r *http.Request) (int, string) {
				return http.StatusOK, okResponse(`{"sandbox_id":"sb-ttl"}`)
			})
			c := m.client()

			env := sampleEnv(map[string]interface{}{
				"arcaTemplateId": "tpl1",
				"ttl":            tc.raw,
			})
			_, err := c.CreateEnvInstance(env)
			if tc.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			body := decodeBody(t, m.lastRequest(t).Body)
			if tc.wantOmit {
				if _, ok := body["ttl_in_minutes"]; ok {
					t.Errorf("ttl_in_minutes should be omitted, got %v", body["ttl_in_minutes"])
				}
				return
			}
			got := fmt.Sprint(body["ttl_in_minutes"])
			want := fmt.Sprint(tc.wantMin)
			if got != want {
				t.Errorf("ttl_in_minutes = %s, want %s", got, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FC-API-06: metadata injection
// ---------------------------------------------------------------------------

func TestArcaCreate_MetadataInjected(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-md"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"owner":          "alice",
	})
	env.Name = "my-env"
	env.Version = "1.0"
	if _, err := c.CreateEnvInstance(env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeBody(t, m.lastRequest(t).Body)
	md, ok := body["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("metadata = %v, want map", body["metadata"])
	}
	if md["aenv_env_name"] != "my-env" {
		t.Errorf("aenv_env_name = %v", md["aenv_env_name"])
	}
	if md["aenv_env_version"] != "1.0" {
		t.Errorf("aenv_env_version = %v", md["aenv_env_version"])
	}
	if md["aenv_owner"] != "alice" {
		t.Errorf("aenv_owner = %v", md["aenv_owner"])
	}
}

// ---------------------------------------------------------------------------
// FC-API-07/08: no resource, no image in body (spec forbids)
// ---------------------------------------------------------------------------

func TestArcaCreate_NoResourceOrImageInBody(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-nores"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"cpu":            "2",
		"memory":         "4",
		"disk":           "25",
	})
	env.Artifacts = []backend.Artifact{{Type: "docker-image", Content: "irrelevant"}}
	if _, err := c.CreateEnvInstance(env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := string(m.lastRequest(t).Body)
	if strings.Contains(raw, `"resource"`) {
		t.Errorf("body contains resource field: %s", raw)
	}
	if strings.Contains(raw, `"image"`) {
		t.Errorf("body contains image field: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// FC-API-09: Get status mapping
// ---------------------------------------------------------------------------

func TestArcaGet_StatusMapping(t *testing.T) {
	cases := []struct {
		arca string
		want string
	}{
		{"PENDING", models.EnvInstanceStatusPending.String()},
		{"RUNNING", models.EnvInstanceStatusRunning.String()},
		{"FAILED", models.EnvInstanceStatusFailed.String()},
		{"DESTROYED", models.EnvInstanceStatusTerminated.String()},
		{"PAUSED", models.EnvInstanceStatusFailed.String()},
		{"UNKNOWN_VALUE", models.EnvInstanceStatusFailed.String()}, // default branch
	}
	for _, tc := range cases {
		t.Run(tc.arca, func(t *testing.T) {
			m := newArcaMock(t, func(r *http.Request) (int, string) {
				return http.StatusOK, okResponse(fmt.Sprintf(`{"sandbox_id":"sb-1","status":%q}`, tc.arca))
			})
			c := m.client()
			inst, err := c.GetEnvInstance("sb-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inst.Status != tc.want {
				t.Errorf("status = %q, want %q", inst.Status, tc.want)
			}
			if inst.Labels["engine"] != "arca" {
				t.Errorf("engine label missing")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FC-API-10: Get populates pod_ip (Arca emits `podIp` in camelCase)
// ---------------------------------------------------------------------------

func TestArcaGet_PodIPPopulated(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-ip","status":"RUNNING","podIp":"10.1.2.3"}`)
	})
	c := m.client()
	inst, err := c.GetEnvInstance("sb-ip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.IP != "10.1.2.3" {
		t.Errorf("IP = %q, want 10.1.2.3", inst.IP)
	}
}

// ---------------------------------------------------------------------------
// FC-API-11: Delete success
// ---------------------------------------------------------------------------

func TestArcaDelete_Success(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse("1")
	})
	c := m.client()

	if err := c.DeleteEnvInstance("sb-del"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := m.lastRequest(t)
	if req.Method != http.MethodDelete {
		t.Errorf("method = %s", req.Method)
	}
	if !strings.HasSuffix(req.Path, "/sb-del") {
		t.Errorf("path = %s", req.Path)
	}
	if req.Headers.Get("x-agent-sandbox-id") != "sb-del" {
		t.Errorf("x-agent-sandbox-id = %q", req.Headers.Get("x-agent-sandbox-id"))
	}
}

// ---------------------------------------------------------------------------
// FC-API-12: Delete 404 returns error, no panic
// ---------------------------------------------------------------------------

func TestArcaDelete_NotFound_ReturnsError(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusNotFound, `{"success":false,"code":404,"message":"sandbox not found","data":null}`
	})
	c := m.client()

	err := c.DeleteEnvInstance("missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// FC-API-13: List returns not-supported
// ---------------------------------------------------------------------------

func TestArcaList_ReturnsNotSupported(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		t.Fatalf("list should not hit Arca: %s %s", r.Method, r.URL.Path)
		return 0, ""
	})
	c := m.client()

	instances, err := c.ListEnvInstances("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %q, want substring 'not supported'", err)
	}
	if instances != nil {
		t.Errorf("expected nil slice, got %v", instances)
	}
}

// ---------------------------------------------------------------------------
// FC-API-15: Warmup not supported
// ---------------------------------------------------------------------------

func TestArcaWarmup_NotSupported(t *testing.T) {
	c := NewArcaClient("http://127.0.0.1:1", "k")
	err := c.Warmup(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %q, want 'not supported'", err)
	}
}

// ---------------------------------------------------------------------------
// FC-API-16: Arca 5xx passthrough
// ---------------------------------------------------------------------------

func TestArcaCreate_Arca500_PassthroughError(t *testing.T) {
	count := int32(0)
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		atomic.AddInt32(&count, 1)
		return http.StatusInternalServerError, failResponse(500, "boom")
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{"arcaTemplateId": "tpl1"})
	_, err := c.CreateEnvInstance(env)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want contains 500", err)
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("requests = %d, want exactly 1 (no retry)", atomic.LoadInt32(&count))
	}
}

// ---------------------------------------------------------------------------
// FC-API-17: Create timeout
// ---------------------------------------------------------------------------

func TestArcaCreate_Timeout(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		time.Sleep(300 * time.Millisecond)
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-slow"}`)
	})
	c := m.client()
	c.httpClient.Timeout = 50 * time.Millisecond

	env := sampleEnv(map[string]interface{}{"arcaTemplateId": "tpl1"})
	_, err := c.CreateEnvInstance(env)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Timeout") && !strings.Contains(msg, "deadline") {
		t.Errorf("error = %q, want timeout/deadline", err)
	}
}

// ---------------------------------------------------------------------------
// FC-API-18: API key header present on every call
// ---------------------------------------------------------------------------

func TestArca_APIKeyHeader(t *testing.T) {
	seenKeys := []string{}
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		seenKeys = append(seenKeys, r.Header.Get("x-agent-sandbox-api-key"))
		switch {
		case r.Method == http.MethodPost:
			return http.StatusOK, okResponse(`{"sandbox_id":"sb-hdr"}`)
		case r.Method == http.MethodGet:
			return http.StatusOK, okResponse(`{"sandbox_id":"sb-hdr","status":"RUNNING"}`)
		case r.Method == http.MethodDelete:
			return http.StatusOK, okResponse("1")
		}
		return http.StatusNotFound, ""
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{"arcaTemplateId": "tpl1"})
	if _, err := c.CreateEnvInstance(env); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := c.GetEnvInstance("sb-hdr"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := c.DeleteEnvInstance("sb-hdr"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(seenKeys) != 3 {
		t.Fatalf("expected 3 keys captured, got %d", len(seenKeys))
	}
	for i, k := range seenKeys {
		if k != "test-key" {
			t.Errorf("request #%d key = %q, want test-key", i, k)
		}
	}
}

// ---------------------------------------------------------------------------
// engine label merge preserves user labels
// ---------------------------------------------------------------------------

func TestArcaCreate_MergesUserLabels(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-lab"}`)
	})
	c := m.client()

	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"labels": map[string]string{
			"owner":  "alice",
			"engine": "ignored-by-client", // engine must always be arca
		},
	})
	inst, err := c.CreateEnvInstance(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Labels["engine"] != "arca" {
		t.Errorf("engine label = %q, want arca", inst.Labels["engine"])
	}
	if inst.Labels["owner"] != "alice" {
		t.Errorf("owner label = %q, want alice", inst.Labels["owner"])
	}
}

// ---------------------------------------------------------------------------
// FC-API-19: PresignURL happy path + error envelope
// ---------------------------------------------------------------------------

func TestArcaPresignURL_HappyPath(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		// Verify path / headers / body shape.
		if !strings.HasSuffix(r.URL.Path, "/arca/api/v1/sandbox/sb-1/presign/token") {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-agent-sandbox-id") != "sb-1" {
			t.Errorf("missing x-agent-sandbox-id header")
		}
		if r.Header.Get("x-agent-sandbox-port") != "8080" {
			t.Errorf("port header = %q, want 8080", r.Header.Get("x-agent-sandbox-port"))
		}
		return http.StatusOK, okResponse(`{"token":"abc123"}`)
	})
	c := m.client()

	url, err := c.PresignURL("sb-1", 8080, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSuffix := "/arca/api/v1/session/abc123"
	if !strings.HasSuffix(url, wantSuffix) {
		t.Errorf("url = %q, want suffix %q", url, wantSuffix)
	}
}

func TestArcaPresignURL_EmptyToken(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"token":""}`)
	})
	c := m.client()
	if _, err := c.PresignURL("sb-1", 8080, 5); err == nil {
		t.Errorf("expected error on empty token")
	}
}

func TestArcaPresignURL_RejectsZeroPort(t *testing.T) {
	c := (&ArcaClient{}).withDummy()
	if _, err := c.PresignURL("sb-1", 0, 5); err == nil {
		t.Errorf("expected error on port=0")
	}
}

// withDummy makes the receiver usable without a server (input-validation tests).
func (c *ArcaClient) withDummy() *ArcaClient {
	c.baseURL = "http://example.invalid"
	c.apiKey = "k"
	c.httpClient = &http.Client{}
	return c
}
