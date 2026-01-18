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

package controller

import (
	"api-service/service"
	"api-service/util"
	backendmodels "envhub/models"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// EnvServiceController handles EnvService operations
type EnvServiceController struct {
	scheduleClient *service.ScheduleClient
	backendClient  *service.BackendClient
	redisClient    *service.RedisClient
}

// NewEnvServiceController creates a new EnvService controller instance
func NewEnvServiceController(
	scheduleClient service.EnvInstanceService,
	backendClient *service.BackendClient,
	redisClient *service.RedisClient,
) *EnvServiceController {
	// Type assert to *ScheduleClient to access service methods
	sc, ok := scheduleClient.(*service.ScheduleClient)
	if !ok {
		log.Fatal("EnvServiceController requires *ScheduleClient implementation")
	}
	return &EnvServiceController{
		scheduleClient: sc,
		backendClient:  backendClient,
		redisClient:    redisClient,
	}
}

// CreateEnvServiceRequest represents the request body for creating an EnvService
type CreateEnvServiceRequest struct {
	EnvName              string            `json:"envName" binding:"required"`
	Replicas             int32             `json:"replicas"`
	EnvironmentVariables map[string]string `json:"environment_variables"`
	Owner                string            `json:"owner"`

	// Storage configuration
	PVCName   string `json:"pvc_name"`
	MountPath string `json:"mount_path"`
	// Note: storageClass is now configured in helm values.yaml, not via API
	StorageSize string `json:"storage_size"` // If specified, PVC will be created and replicas must be 1

	// Service configuration
	Port int32 `json:"port"`

	// Resource limits
	CPURequest              string `json:"cpu_request"`
	CPULimit                string `json:"cpu_limit"`
	MemoryRequest           string `json:"memory_request"`
	MemoryLimit             string `json:"memory_limit"`
	EphemeralStorageRequest string `json:"ephemeral_storage_request"`
	EphemeralStorageLimit   string `json:"ephemeral_storage_limit"`
}

// CreateEnvService creates a new EnvService (Deployment + Service + PVC)
// POST /env-service/
func (ctrl *EnvServiceController) CreateEnvService(c *gin.Context) {
	var req CreateEnvServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		backendmodels.JSONErrorWithMessage(c, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// Default replicas to 1 if not specified
	if req.Replicas == 0 {
		req.Replicas = 1
	}

	// Split name and version using SplitEnvNameVersionStrict function
	name, version, err := util.SplitEnvNameVersionStrict(req.EnvName)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 400, "Invalid EnvName format: "+err.Error())
		return
	}
	backendEnv, err := ctrl.backendClient.GetEnvByVersion(name, version)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to find environment: "+err.Error())
		return
	}
	if backendEnv == nil {
		backendmodels.JSONErrorWithMessage(c, 404, "Environment not found: "+req.EnvName)
		return
	}

	// Configure DeployConfig
	if backendEnv.DeployConfig == nil {
		backendEnv.DeployConfig = make(map[string]interface{})
	}
	if req.EnvironmentVariables != nil {
		backendEnv.DeployConfig["environmentVariables"] = req.EnvironmentVariables
	}
	backendEnv.DeployConfig["replicas"] = req.Replicas
	if req.Owner != "" {
		backendEnv.DeployConfig["owner"] = req.Owner
	}

	// Storage configuration
	if req.PVCName != "" {
		backendEnv.DeployConfig["pvcName"] = req.PVCName
	}
	if req.MountPath != "" {
		backendEnv.DeployConfig["mountPath"] = req.MountPath
	}
	// storageClass is now configured in helm values.yaml, not passed via API
	if req.StorageSize != "" {
		backendEnv.DeployConfig["storageSize"] = req.StorageSize
	}

	// Service configuration
	if req.Port > 0 {
		backendEnv.DeployConfig["port"] = req.Port
	}

	// Resource configuration
	if req.CPURequest != "" {
		backendEnv.DeployConfig["cpuRequest"] = req.CPURequest
	}
	if req.CPULimit != "" {
		backendEnv.DeployConfig["cpuLimit"] = req.CPULimit
	}
	if req.MemoryRequest != "" {
		backendEnv.DeployConfig["memoryRequest"] = req.MemoryRequest
	}
	if req.MemoryLimit != "" {
		backendEnv.DeployConfig["memoryLimit"] = req.MemoryLimit
	}
	if req.EphemeralStorageRequest != "" {
		backendEnv.DeployConfig["ephemeralStorageRequest"] = req.EphemeralStorageRequest
	}
	if req.EphemeralStorageLimit != "" {
		backendEnv.DeployConfig["ephemeralStorageLimit"] = req.EphemeralStorageLimit
	}

	// Call ScheduleClient to create Service
	envService, err := ctrl.scheduleClient.CreateService(backendEnv)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to create service: "+err.Error())
		return
	}
	envService.Env = backendEnv

	// Construct response data
	backendmodels.JSONSuccess(c, envService)
}

// GetEnvService retrieves a single EnvService
// GET /env-service/:id
func (ctrl *EnvServiceController) GetEnvService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "Missing id parameter")
		return
	}
	// Call ScheduleClient to query Service
	envService, err := ctrl.scheduleClient.GetService(id)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to query service: "+err.Error())
		return
	}
	backendmodels.JSONSuccess(c, envService)
}

// DeleteEnvService deletes an EnvService
// DELETE /env-service/:id?deleteStorage=true
func (ctrl *EnvServiceController) DeleteEnvService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "Missing id parameter")
		return
	}

	// Check if deleteStorage query parameter is set
	deleteStorage := c.Query("deleteStorage") == "true"

	// Call ScheduleClient to delete Service
	success, err := ctrl.scheduleClient.DeleteService(id, deleteStorage)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to delete service: "+err.Error())
		return
	}
	if !success {
		backendmodels.JSONErrorWithMessage(c, 500, "Service deletion returned false")
		return
	}
	backendmodels.JSONSuccess(c, "Deleted successfully")
}

// UpdateEnvServiceRequest represents the request body for updating an EnvService
type UpdateEnvServiceRequest struct {
	Replicas             *int32             `json:"replicas,omitempty"`
	Image                *string            `json:"image,omitempty"`
	EnvironmentVariables *map[string]string `json:"environment_variables,omitempty"`
}

// UpdateEnvService updates an EnvService (replicas, image, env vars)
// PUT /env-service/:id
func (ctrl *EnvServiceController) UpdateEnvService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "Missing id parameter")
		return
	}

	var req UpdateEnvServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		backendmodels.JSONErrorWithMessage(c, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// Build update request
	updateReq := &service.UpdateServiceRequest{
		Replicas:             req.Replicas,
		Image:                req.Image,
		EnvironmentVariables: req.EnvironmentVariables,
	}

	// Call ScheduleClient to update Service
	envService, err := ctrl.scheduleClient.UpdateService(id, updateReq)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to update service: "+err.Error())
		return
	}
	backendmodels.JSONSuccess(c, envService)
}

// ListEnvServices lists EnvServices
// GET /env-service/:id/list
func (ctrl *EnvServiceController) ListEnvServices(c *gin.Context) {
	id := c.Param("id")

	// Handle wildcard "*" as "list all services"
	if id == "*" {
		id = ""
	}

	// Extract envName from id or query parameter
	var envName string
	if id != "" {
		name, _ := util.SplitEnvNameVersion(id)
		envName = name
	} else {
		envName = c.Query("envName")
	}

	services, err := ctrl.scheduleClient.ListServices(envName)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to list services: "+err.Error())
		return
	}
	backendmodels.JSONSuccess(c, services)
}
