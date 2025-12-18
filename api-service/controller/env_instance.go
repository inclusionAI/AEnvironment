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
	"api-service/models"
	backendmodels "envhub/models"

	"api-service/service"
	"api-service/util"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// EnvInstanceController handles EnvInstance operations
type EnvInstanceController struct {
	envInstanceService service.EnvInstanceService // use interface
	backendClient      *service.BackendClient
	redisClient        *service.RedisClient
}

// NewEnvInstanceController creates a new EnvInstance controller instance
func NewEnvInstanceController(
	envInstanceService service.EnvInstanceService,
	backendClient *service.BackendClient,
	redisClient *service.RedisClient,
) *EnvInstanceController {
	return &EnvInstanceController{
		envInstanceService: envInstanceService,
		backendClient:      backendClient,
		redisClient:        redisClient,
	}
}

// CreateEnvInstanceRequest represents the request body for creating an EnvInstance
type CreateEnvInstanceRequest struct {
	EnvName              string            `json:"envName" binding:"required"`
	Datasource           string            `json:"datasource"`
	EnvironmentVariables map[string]string `json:"environment_variables"`
	Arguments            []string          `json:"arguments"`
	TTL                  string            `json:"ttl"`
}

// CreateEnvInstance creates a new EnvInstance
// POST /env-instance/
func (ctrl *EnvInstanceController) CreateEnvInstance(c *gin.Context) {
	var req CreateEnvInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		backendmodels.JSONErrorWithMessage(c, 400, "Invalid request parameters: "+err.Error())
		return
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
	if req.Datasource != "" {
		if backendEnv.DeployConfig == nil {
			backendEnv.DeployConfig = make(map[string]interface{})
		}
		// Prefer imagePrefix from DeployConfig, default to empty string
		imagePrefix := "docker.io/library/aenv"
		if value, ok := backendEnv.DeployConfig["imagePrefix"]; ok {
			if str, ok2 := value.(string); ok2 {
				imagePrefix = str
			}
		}
		secondImageName := imagePrefix + ":" + req.Datasource
		backendEnv.DeployConfig["secondImageName"] = secondImageName
	}
	if req.EnvironmentVariables != nil {
		backendEnv.DeployConfig["environmentVariables"] = req.EnvironmentVariables
	}
	if req.Arguments != nil {
		backendEnv.DeployConfig["arguments"] = req.Arguments
	}
	// Set TTL for environment
	backendEnv.DeployConfig["ttl"] = req.TTL
	// Call ScheduleClient to create Pod
	envInstance, err := ctrl.envInstanceService.CreateEnvInstance(backendEnv)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to create: "+err.Error())
		return
	}
	envInstance.Env = backendEnv

	token := util.GetCurrentToken(c)
	if token != nil && ctrl.redisClient != nil {
		if result, err := ctrl.redisClient.StoreEnvInstanceToRedis(token.Token, envInstance); !result || err != nil {
			log.Warnf("failed to store EnvInstance in Redis: %v", err)
		}
	}
	// Construct response data
	backendmodels.JSONSuccess(c, envInstance)
}

// GetEnvInstance retrieves a single EnvInstance
// GET /env-instance/:id
func (ctrl *EnvInstanceController) GetEnvInstance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "Missing id parameter")
		return
	}
	// Call ScheduleClient to query Pod
	envInstance, err := ctrl.envInstanceService.GetEnvInstance(id)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to query: "+err.Error())
		return
	}
	backendmodels.JSONSuccess(c, envInstance)
}

// DeleteEnvInstance deletes an EnvInstance
// DELETE /env-instance/:id
func (ctrl *EnvInstanceController) DeleteEnvInstance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "Missing id parameter")
		return
	}

	// Call ScheduleClient to delete Pod
	err := ctrl.envInstanceService.DeleteEnvInstance(id)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "Failed to delete: "+err.Error())
		return
	}
	backendmodels.JSONSuccess(c, "Deleted successfully")
	token := util.GetCurrentToken(c)
	if token != nil && ctrl.redisClient != nil {
		if result, err := ctrl.redisClient.RemoveEnvInstanceFromRedis(token.Token, id); !result || err != nil {
			log.Warnf("failed to remove EnvInstance in Redis: %v", err)
		}
	}
}

func (ctrl *EnvInstanceController) ListEnvInstances(c *gin.Context) {
	token := util.GetCurrentToken(c)
	if token == nil {
		backendmodels.JSONErrorWithMessage(c, 403, "token required")
		return
	}
	if ctrl.redisClient != nil {
		var query = models.EnvInstance{Env: &backendmodels.Env{}}
		id := c.Param("id")
		if id != "" {
			name, version := util.SplitEnvNameVersion(id)
			query.Env.Name = name
			query.Env.Version = version
		}
		instances, err := ctrl.redisClient.ListEnvInstancesFromRedis(token.Token, &query)
		if err == nil {
			backendmodels.JSONSuccess(c, instances)
			return
		}
		log.Warnf("failed to list from redis: %v", err)
	}
	envName := c.Query("envName")
	instances, err := ctrl.envInstanceService.ListEnvInstances(envName)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, err.Error())
		return
	}
	backendmodels.JSONSuccess(c, instances)
}

func (ctrl *EnvInstanceController) Warmup(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		backendmodels.JSONErrorWithMessage(c, 400, "env id required")
		return
	}
	name, version, err := util.SplitEnvNameVersionStrict(id)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 400, "invalid env format, should be name@version")
		return
	}
	backendEnv, err := ctrl.backendClient.GetEnvByVersion(name, version)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, "failed to warm up env instance: "+err.Error())
		return
	}
	if backendEnv == nil {
		backendmodels.JSONErrorWithMessage(c, 404, "can not find env by id: "+id)
		return
	}

	err = ctrl.envInstanceService.Warmup(backendEnv)
	if err != nil {
		backendmodels.JSONErrorWithMessage(c, 500, err.Error())
		return
	}
	backendmodels.JSONSuccess(c, backendEnv)
}
