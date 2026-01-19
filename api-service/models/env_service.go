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

// EnvServiceStatus environment service status enumeration
type EnvServiceStatus int

const (
	EnvServiceStatusPending EnvServiceStatus = iota
	EnvServiceStatusCreating
	EnvServiceStatusRunning
	EnvServiceStatusUpdating
	EnvServiceStatusFailed
	EnvServiceStatusTerminated
)

// String returns string representation of status
func (s EnvServiceStatus) String() string {
	switch s {
	case EnvServiceStatusPending:
		return "Pending"
	case EnvServiceStatusCreating:
		return "Creating"
	case EnvServiceStatusRunning:
		return "Running"
	case EnvServiceStatusUpdating:
		return "Updating"
	case EnvServiceStatusFailed:
		return "Failed"
	case EnvServiceStatusTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

// EnvService environment service object (Deployment + Service + PVC)
type EnvService struct {
	ID                   string            `json:"id"`                    // Service id, corresponds to deployment name
	Env                  *backend.Env      `json:"env"`                   // Env object
	Status               string            `json:"status"`                // Service status
	CreatedAt            string            `json:"created_at"`            // Creation time
	UpdatedAt            string            `json:"updated_at"`            // Update time
	Replicas             int32             `json:"replicas"`              // Number of replicas
	AvailableReplicas    int32             `json:"available_replicas"`    // Number of available replicas
	ServiceURL           string            `json:"service_url"`           // Service URL (internal cluster DNS)
	Owner                string            `json:"owner"`                 // Service owner (user who created it)
	EnvironmentVariables map[string]string `json:"environment_variables"` // Environment variables
	PVCName              string            `json:"pvc_name"`              // PVC name (shared by same envName)
}

// NewEnvService creates a new environment service object
func NewEnvService(id string, env *backend.Env, replicas int32, owner string, envVars map[string]string, pvcName string) *EnvService {
	now := time.Now().Format("2006-01-02 15:04:05")
	return &EnvService{
		ID:                   id,
		Env:                  env,
		Status:               EnvServiceStatusPending.String(),
		CreatedAt:            now,
		UpdatedAt:            now,
		Replicas:             replicas,
		AvailableReplicas:    0,
		Owner:                owner,
		EnvironmentVariables: envVars,
		PVCName:              pvcName,
	}
}

// NewEnvServiceWithStatus creates an environment service object with specified status
func NewEnvServiceWithStatus(id string, env *backend.Env, status EnvServiceStatus, replicas int32, availableReplicas int32, serviceURL string, owner string, envVars map[string]string, pvcName string) *EnvService {
	now := time.Now().Format("2006-01-02 15:04:05")
	return &EnvService{
		ID:                   id,
		Env:                  env,
		Status:               status.String(),
		CreatedAt:            now,
		UpdatedAt:            now,
		Replicas:             replicas,
		AvailableReplicas:    availableReplicas,
		ServiceURL:           serviceURL,
		Owner:                owner,
		EnvironmentVariables: envVars,
		PVCName:              pvcName,
	}
}

// UpdateStatus updates service status
func (s *EnvService) UpdateStatus(status EnvServiceStatus) {
	s.Status = status.String()
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
}

// UpdateReplicas updates service replicas
func (s *EnvService) UpdateReplicas(replicas int32, availableReplicas int32) {
	s.Replicas = replicas
	s.AvailableReplicas = availableReplicas
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
}

// UpdateServiceURL updates service URL
func (s *EnvService) UpdateServiceURL(url string) {
	s.ServiceURL = url
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
}
