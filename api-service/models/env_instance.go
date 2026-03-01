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
	backend "envhub/models"
	"time"
)

// EnvInstanceStatus environment instance status enumeration
type EnvInstanceStatus int

const (
	EnvInstanceStatusPending EnvInstanceStatus = iota
	EnvInstanceStatusCreating
	EnvInstanceStatusRunning
	EnvInstanceStatusFailed
	EnvInstanceStatusTerminated
)

// String returns string representation of status
func (s EnvInstanceStatus) String() string {
	switch s {
	case EnvInstanceStatusPending:
		return "Pending"
	case EnvInstanceStatusCreating:
		return "Creating"
	case EnvInstanceStatusRunning:
		return "Running"
	case EnvInstanceStatusFailed:
		return "Failed"
	case EnvInstanceStatusTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

// EnvInstance environment instance object
type EnvInstance struct {
	ID        string       `json:"id"`         // Instance id, corresponds to podname
	Env       *backend.Env `json:"env"`        // Env object
	Status    string       `json:"status"`     // Instance status
	CreatedAt string       `json:"created_at"` // Creation time
	UpdatedAt string       `json:"updated_at"` // Update time
	IP        string       `json:"ip"`         // Instance IP
	DataURL   string       `json:"data_url"`   // MCP data endpoint URL (http://IP:8081/mcp)
	TTL       string       `json:"ttl"`        // time to live
	Owner     string       `json:"owner"`      // Instance owner (user who created it)
}

// NewEnvInstance creates a new environment instance object
func NewEnvInstance(id string, env *backend.Env, ip string) *EnvInstance {
	now := time.Now().Format("2006-01-02 15:04:05")
	dataURL := ""
	if ip != "" {
		dataURL = "http://" + ip + ":8081/mcp"
	}
	return &EnvInstance{
		ID:        id,
		Env:       env,
		Status:    EnvInstanceStatusPending.String(),
		CreatedAt: now,
		UpdatedAt: now,
		IP:        ip,
		DataURL:   dataURL,
		Owner:     "",
	}
}

// NewEnvInstanceWithOwner creates a new environment instance object with owner
func NewEnvInstanceWithOwner(id string, env *backend.Env, ip string, owner string) *EnvInstance {
	now := time.Now().Format("2006-01-02 15:04:05")
	dataURL := ""
	if ip != "" {
		dataURL = "http://" + ip + ":8081/mcp"
	}
	return &EnvInstance{
		ID:        id,
		Env:       env,
		Status:    EnvInstanceStatusPending.String(),
		CreatedAt: now,
		UpdatedAt: now,
		IP:        ip,
		DataURL:   dataURL,
		Owner:     owner,
	}
}

// NewEnvInstanceWithStatus creates an environment instance object with specified status
func NewEnvInstanceWithStatus(id string, env *backend.Env, status EnvInstanceStatus, ip string) *EnvInstance {
	now := time.Now().Format("2006-01-02 15:04:05")
	dataURL := ""
	if ip != "" {
		dataURL = "http://" + ip + ":8081/mcp"
	}
	return &EnvInstance{
		ID:        id,
		Env:       env,
		Status:    status.String(),
		CreatedAt: now,
		UpdatedAt: now,
		IP:        ip,
		DataURL:   dataURL,
		Owner:     "",
	}
}

// NewEnvInstanceFull creates a complete environment instance object (specify all fields)
func NewEnvInstanceFull(id string, env *backend.Env, status EnvInstanceStatus, createdAt, updatedAt, ip string) *EnvInstance {
	dataURL := ""
	if ip != "" {
		dataURL = "http://" + ip + ":8081/mcp"
	}
	return &EnvInstance{
		ID:        id,
		Env:       env,
		Status:    status.String(),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		IP:        ip,
		DataURL:   dataURL,
		Owner:     "",
	}
}

// UpdateStatus updates instance status
func (e *EnvInstance) UpdateStatus(status EnvInstanceStatus) {
	e.Status = status.String()
	e.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
}

// UpdateIP updates instance IP
func (e *EnvInstance) UpdateIP(ip string) {
	e.IP = ip
	if ip != "" {
		e.DataURL = "http://" + ip + ":8081/mcp"
	} else {
		e.DataURL = ""
	}
	e.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
}
