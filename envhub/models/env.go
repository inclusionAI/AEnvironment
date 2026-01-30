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

package models

import (
	"encoding/json"
	"fmt"
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

// Env environment information
type Env struct {
	ID           string                 `json:"id"`            // Identifier id
	Name         string                 `json:"name"`          // Environment name
	Description  string                 `json:"description"`   // Environment description
	Version      string                 `json:"version"`       // Version
	Tags         []string               `json:"tags"`          // Tags
	CodeURL      string                 `json:"code_url"`      // Code
	Status       EnvStatus              `json:"status"`        // Status
	Artifacts    []Artifact             `json:"artifacts"`     // Artifact information list
	BuildConfig  map[string]interface{} `json:"build_config"`  // Build configuration
	TestConfig   map[string]interface{} `json:"test_config"`   // Test configuration
	DeployConfig map[string]interface{} `json:"deploy_config"` // Deploy configuration
	CreatedAt    time.Time              `json:"created_at,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at,omitempty"`
}

// Optional: create constructor
func NewEnv(id, name, description, version, codeURL string) *Env {
	return &Env{
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
func (e *Env) AddTag(tag string) {
	e.Tags = append(e.Tags, tag)
}

func (e *Env) AddArtifact(artifact Artifact) {
	e.Artifacts = append(e.Artifacts, artifact)
}

func (e *Env) SetBuildConfig(key string, value interface{}) {
	e.BuildConfig[key] = value
}

func (e *Env) SetTestConfig(key string, value interface{}) {
	e.TestConfig[key] = value
}

func (e *Env) SetDeployConfig(key string, value interface{}) {
	e.DeployConfig[key] = value
}

func (e *Env) UpdateStatus(status EnvStatus) {
	e.Status = status
	e.UpdatedAt = time.Now()
}

// Optional: JSON serialization method
func (e *Env) ToJSON() ([]byte, error) {
	// Need to import "encoding/json"
	envMap := make(map[string]interface{})
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &envMap)
	if err != nil {
		return nil, err
	}
	envMap["codeUrl"] = e.CodeURL
	envMap["buildConfig"] = e.BuildConfig
	envMap["testConfig"] = e.TestConfig
	envMap["deployConfig"] = e.DeployConfig
	envMap["createdAt"] = e.CreatedAt
	envMap["updatedAt"] = e.UpdatedAt
	return json.Marshal(envMap)
}

func (e *Env) FromJSON(data []byte) error {
	envMap := make(map[string]interface{})
	err := json.Unmarshal(data, &envMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, e)
	if err != nil {
		return err
	}
	if e.CodeURL == "" && envMap["codeUrl"] != nil {
		e.CodeURL = envMap["codeUrl"].(string)
	}
	if e.BuildConfig == nil && envMap["buildConfig"] != nil {
		e.BuildConfig = envMap["buildConfig"].(map[string]interface{})
	}
	if e.TestConfig == nil && envMap["testConfig"] != nil {
		e.TestConfig = envMap["testConfig"].(map[string]interface{})
	}
	if e.DeployConfig == nil && envMap["deployConfig"] != nil {
		e.DeployConfig = envMap["deployConfig"].(map[string]interface{})
	}
	if e.CreatedAt.IsZero() && envMap["createdAt"] != nil {
		e.CreatedAt, err = time.Parse(time.RFC3339, envMap["createdAt"].(string))
		if err != nil {
			return err
		}
	}
	if e.UpdatedAt.IsZero() && envMap["updatedAt"] != nil {
		e.UpdatedAt, err = time.Parse(time.RFC3339, envMap["updatedAt"].(string))
		if err != nil {
			return err
		}
	}
	return nil
}

// GetImage finds the artifact of type "docker-image" from Artifacts and returns its Content (the image address)
func (e *Env) GetImage() string {
	for _, artifact := range e.Artifacts {
		if strings.EqualFold(artifact.Type, "image") {
			return artifact.Content
		}
	}
	return ""
}

// GetMemory retrieves the memory configuration from DeployConfig, such as "2G"
func (e *Env) GetMemory() string {
	if val, exists := e.DeployConfig["memory"]; exists {
		if s, ok := val.(string); ok {
			return s
		}
		// If it's another type (such as float64), try to convert to string
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// GetCPU retrieves the cpu configuration from DeployConfig, such as "1C"
func (e *Env) GetCPU() string {
	if val, exists := e.DeployConfig["cpu"]; exists {
		if s, ok := val.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", val)
	}
	return ""
}
