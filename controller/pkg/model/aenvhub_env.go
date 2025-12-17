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

package model

import (
	"encoding/json"
	"strings"
	"time"
)

// EnvStatus environment status enumeration
type EnvStatus int

const (
	EnvStatusInit EnvStatus = iota
	EnvStatusPending
	EnvStatusCreating
	EnvStatusCreated
	EnvStatusTesting
	EnvStatusVerified
	EnvStatusReady
	EnvStatusReleased
	EnvStatusFailed
)

func EnvStatusByName(name string) EnvStatus {
	var status EnvStatus
	switch strings.ToLower(name) {
	case "init":
		status = EnvStatusInit
	case "pending":
		status = EnvStatusPending
	case "creating":
		status = EnvStatusCreating
	case "created":
		status = EnvStatusCreated
	case "testing":
		status = EnvStatusTesting
	case "verified":
		status = EnvStatusVerified
	case "ready":
		status = EnvStatusReady
	case "released":
		status = EnvStatusReleased
	case "failed":
		status = EnvStatusFailed
	default:
		status = EnvStatusInit
	}
	return status
}

func EnvStatusNameByStatus(status EnvStatus) string {
	statusNames := map[EnvStatus]string{
		EnvStatusInit:     "Init",
		EnvStatusPending:  "Pending",
		EnvStatusCreating: "Creating",
		EnvStatusCreated:  "Created",
		EnvStatusTesting:  "Testing",
		EnvStatusVerified: "Verified",
		EnvStatusReady:    "Ready",
		EnvStatusReleased: "Released",
		EnvStatusFailed:   "Failed",
	}

	if name, exists := statusNames[status]; exists {
		return name
	}
	return "Init"
}

// Artifact artifact information
type Artifact struct {
	Id      string `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

// AEnvHubEnv environment information
type AEnvHubEnv struct {
	ID           string                 `json:"id"`           // Identifier ID
	Name         string                 `json:"name"`         // Environment name
	Description  string                 `json:"description"`  // Environment description
	Version      string                 `json:"version"`      // Version
	Tags         []string               `json:"tags"`         // Tags
	CodeURL      string                 `json:"code_url"`     // Code URL
	Status       EnvStatus              `json:"status"`       // Status
	Artifacts    []Artifact             `json:"artifacts"`    // Artifact information list
	BuildConfig  map[string]interface{} `json:"build_config"` // Build configuration
	TestConfig   map[string]interface{} `json:"test_config"`  // Test configuration
	DeployConfig map[string]interface{} `json:"deployConfig"` // Deployment configuration
	CreatedAt    time.Time              `json:"created_at,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at,omitempty"`
}

// Optional: Create constructor
func NewEnv(id, name, description, version, codeURL string) *AEnvHubEnv {
	return &AEnvHubEnv{
		ID:           id,
		Name:         name,
		Description:  description,
		Version:      version,
		CodeURL:      codeURL,
		Tags:         make([]string, 0),
		Status:       EnvStatusPending,
		Artifacts:    make([]Artifact, 0),
		BuildConfig:  make(map[string]interface{}),
		TestConfig:   make(map[string]interface{}),
		DeployConfig: make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// Optional: Add methods
func (e *AEnvHubEnv) AddTag(tag string) {
	e.Tags = append(e.Tags, tag)
}

func (e *AEnvHubEnv) AddArtifact(artifact Artifact) {
	e.Artifacts = append(e.Artifacts, artifact)
}

func (e *AEnvHubEnv) SetBuildConfig(key string, value interface{}) {
	e.BuildConfig[key] = value
}

func (e *AEnvHubEnv) SetTestConfig(key string, value interface{}) {
	e.TestConfig[key] = value
}

func (e *AEnvHubEnv) SetDeployConfig(key string, value interface{}) {
	e.DeployConfig[key] = value
}

func (e *AEnvHubEnv) UpdateStatus(status EnvStatus) {
	e.Status = status
	e.UpdatedAt = time.Now()
}

// Optional: JSON serialization methods
func (e *AEnvHubEnv) ToJSON() ([]byte, error) {
	// Need to import "encoding/json"
	return json.Marshal(e)
}

func (e *AEnvHubEnv) FromJSON(data []byte) error {
	// Need to import "encoding/json"
	return json.Unmarshal(data, e)
}
