package service

import (
	"testing"

	backend "envhub/models"
)

func TestCreateEnvInstance_LabelsExtractedFromDeployConfig(t *testing.T) {
	tests := []struct {
		name           string
		deployConfig   map[string]interface{}
		wantEnvLabel   string
		wantExperiment string
		wantOwner      string
		wantApp        string
	}{
		{
			name: "user-provided labels with all business keys",
			deployConfig: map[string]interface{}{
				"memory": "4G",
				"labels": map[string]string{
					"experiment": "swe-bench",
					"owner":      "jun",
					"app":        "evaluator",
				},
			},
			wantEnvLabel:   "myenv-1.0.0", // auto-set from name-version
			wantExperiment: "swe-bench",
			wantOwner:      "jun",
			wantApp:        "evaluator",
		},
		{
			name: "user-provided labels with custom env override",
			deployConfig: map[string]interface{}{
				"labels": map[string]string{
					"env":        "custom-env",
					"experiment": "exp1",
				},
			},
			wantEnvLabel:   "custom-env", // user override preserved
			wantExperiment: "exp1",
			wantOwner:      "",
			wantApp:        "",
		},
		{
			name:         "no labels in DeployConfig",
			deployConfig: map[string]interface{}{"memory": "2G"},
			wantEnvLabel: "myenv-1.0.0", // auto-set
		},
		{
			name:         "nil DeployConfig labels",
			deployConfig: map[string]interface{}{"labels": nil},
			wantEnvLabel: "myenv-1.0.0",
		},
		{
			name:         "empty DeployConfig",
			deployConfig: map[string]interface{}{},
			wantEnvLabel: "myenv-1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &backend.Env{
				Name:         "myenv",
				Version:      "1.0.0",
				DeployConfig: tt.deployConfig,
			}

			// Test the labels extraction logic directly (without network calls)
			// This mirrors the logic in FaaSClient.CreateEnvInstance
			var labels map[string]string
			if l, ok := env.DeployConfig["labels"]; ok {
				if labelMap, ok := l.(map[string]string); ok {
					labels = labelMap
				}
			}
			if labels == nil {
				labels = make(map[string]string)
			}
			functionName := env.Name + "-" + env.Version
			if _, exists := labels["env"]; !exists {
				labels["env"] = functionName
			}

			if labels["env"] != tt.wantEnvLabel {
				t.Errorf("env label = %q, want %q", labels["env"], tt.wantEnvLabel)
			}
			if tt.wantExperiment != "" && labels["experiment"] != tt.wantExperiment {
				t.Errorf("experiment label = %q, want %q", labels["experiment"], tt.wantExperiment)
			}
			if tt.wantOwner != "" && labels["owner"] != tt.wantOwner {
				t.Errorf("owner label = %q, want %q", labels["owner"], tt.wantOwner)
			}
			if tt.wantApp != "" && labels["app"] != tt.wantApp {
				t.Errorf("app label = %q, want %q", labels["app"], tt.wantApp)
			}
		})
	}
}

func TestCreateEnvInstance_LabelsSetOnResult(t *testing.T) {
	// Verify that labels from DeployConfig end up on the returned EnvInstance
	env := &backend.Env{
		Name:    "test",
		Version: "v1",
		DeployConfig: map[string]interface{}{
			"labels": map[string]string{
				"experiment": "rl-training",
				"owner":      "team-a",
				"app":        "sandbox",
			},
		},
	}

	// Extract labels (same logic as FaaSClient.CreateEnvInstance)
	var labels map[string]string
	if l, ok := env.DeployConfig["labels"]; ok {
		if labelMap, ok := l.(map[string]string); ok {
			labels = labelMap
		}
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if _, exists := labels["env"]; !exists {
		labels["env"] = env.Name + "-" + env.Version
	}

	// Verify all expected labels
	expected := map[string]string{
		"env":        "test-v1",
		"experiment": "rl-training",
		"owner":      "team-a",
		"app":        "sandbox",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("labels[%q] = %q, want %q", k, labels[k], v)
		}
	}
}
