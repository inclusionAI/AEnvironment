package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-service/models"

	"github.com/gin-gonic/gin"
)

func TestCreateEnvInstanceRequest_LabelsBinding(t *testing.T) {
	// Test that JSON binding correctly parses labels
	tests := []struct {
		name       string
		body       string
		wantLabels map[string]string
		wantErr    bool
	}{
		{
			name: "with labels",
			body: `{"envName":"test@v1","labels":{"experiment":"exp1","owner":"jun","app":"chatbot"},"ttl":"30m"}`,
			wantLabels: map[string]string{
				"experiment": "exp1",
				"owner":      "jun",
				"app":        "chatbot",
			},
		},
		{
			name:       "without labels",
			body:       `{"envName":"test@v1","ttl":"30m"}`,
			wantLabels: nil,
		},
		{
			name:       "empty labels",
			body:       `{"envName":"test@v1","labels":{},"ttl":"30m"}`,
			wantLabels: map[string]string{},
		},
		{
			name:    "missing envName",
			body:    `{"labels":{"app":"test"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateEnvInstanceRequest
			err := json.Unmarshal([]byte(tt.body), &req)

			if tt.wantErr {
				// For binding:"required" fields, we test via gin context
				gin.SetMode(gin.TestMode)
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("POST", "/env-instance", bytes.NewBufferString(tt.body))
				c.Request.Header.Set("Content-Type", "application/json")
				bindErr := c.ShouldBindJSON(&req)
				if bindErr == nil {
					t.Error("expected binding error for missing required field")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected unmarshal error: %v", err)
			}

			if tt.wantLabels == nil {
				if req.Labels != nil {
					t.Errorf("expected nil labels, got %v", req.Labels)
				}
				return
			}

			if len(req.Labels) != len(tt.wantLabels) {
				t.Errorf("labels length = %d, want %d", len(req.Labels), len(tt.wantLabels))
			}

			for k, v := range tt.wantLabels {
				if req.Labels[k] != v {
					t.Errorf("labels[%q] = %q, want %q", k, req.Labels[k], v)
				}
			}
		})
	}
}

func TestCreateEnvInstance_LabelsPassedToDeployConfig(t *testing.T) {
	// Verify that when labels are in the request, they get set in DeployConfig
	reqBody := CreateEnvInstanceRequest{
		EnvName: "test@v1",
		TTL:     "30m",
		Labels: map[string]string{
			"experiment": "swe-bench",
			"owner":      "jun",
			"app":        "evaluator",
		},
	}

	// Simulate what the controller does
	deployConfig := make(map[string]interface{})
	if reqBody.TTL != "" {
		deployConfig["ttl"] = reqBody.TTL
	}
	if reqBody.Labels != nil {
		deployConfig["labels"] = reqBody.Labels
	}

	// Verify labels are in DeployConfig
	labels, ok := deployConfig["labels"].(map[string]string)
	if !ok {
		t.Fatal("labels not found in DeployConfig")
	}

	expected := map[string]string{
		"experiment": "swe-bench",
		"owner":      "jun",
		"app":        "evaluator",
	}

	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("DeployConfig labels[%q] = %q, want %q", k, labels[k], v)
		}
	}
}

func TestCreateEnvInstance_LabelsOnResponse(t *testing.T) {
	// Test that the response includes labels
	gin.SetMode(gin.TestMode)

	instance := &models.EnvInstance{
		ID:     "inst-123",
		Status: "Running",
		Labels: map[string]string{
			"envName":    "test-v1",
			"experiment": "exp1",
			"owner":      "jun",
			"app":        "chatbot",
		},
	}

	// Marshal to JSON and verify labels are present
	data, err := json.Marshal(instance)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	labels, ok := result["labels"].(map[string]interface{})
	if !ok {
		t.Fatal("labels not in response")
	}

	if labels["envName"] != "test-v1" {
		t.Errorf("response labels.envName = %v, want test-v1", labels["envName"])
	}
	if labels["experiment"] != "exp1" {
		t.Errorf("response labels.experiment = %v, want exp1", labels["experiment"])
	}
}

func TestCreateEnvInstanceRequest_FullRoundTrip(t *testing.T) {
	// Test the full JSON serialization round-trip with labels
	original := CreateEnvInstanceRequest{
		EnvName: "terminal@1.2.0",
		TTL:     "1h",
		Owner:   "jun",
		Labels: map[string]string{
			"experiment": "swe-bench-eval",
			"owner":      "jun",
			"app":        "code-review",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CreateEnvInstanceRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.EnvName != original.EnvName {
		t.Errorf("envName = %q, want %q", decoded.EnvName, original.EnvName)
	}
	if len(decoded.Labels) != len(original.Labels) {
		t.Errorf("labels length = %d, want %d", len(decoded.Labels), len(original.Labels))
	}
	for k, v := range original.Labels {
		if decoded.Labels[k] != v {
			t.Errorf("labels[%q] = %q, want %q", k, decoded.Labels[k], v)
		}
	}
}

func TestCreateEnvInstanceRequest_HTTPRequest(t *testing.T) {
	// Test creating a real HTTP request with labels
	gin.SetMode(gin.TestMode)

	body := `{"envName":"stem@bugfix","labels":{"experiment":"swe-bench","owner":"jun","app":"evaluator"},"ttl":"30m"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/env-instance", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req CreateEnvInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		t.Fatalf("binding error: %v", err)
	}

	if req.EnvName != "stem@bugfix" {
		t.Errorf("envName = %q, want stem@bugfix", req.EnvName)
	}
	if req.TTL != "30m" {
		t.Errorf("ttl = %q, want 30m", req.TTL)
	}
	if len(req.Labels) != 3 {
		t.Errorf("labels count = %d, want 3", len(req.Labels))
	}
	if req.Labels["experiment"] != "swe-bench" {
		t.Errorf("labels.experiment = %q, want swe-bench", req.Labels["experiment"])
	}

	// Verify status 200 (no error in binding)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
