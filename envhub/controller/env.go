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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"envhub/models"
	"envhub/service"
)

type EnvController struct {
	storage    service.EnvStorage
	ossStorage *service.OssStorage
	ciTrigger  service.CITrigger
}

func NewEnvController(storage service.EnvStorage, ossStorage *service.OssStorage, ciTrigger service.CITrigger) *EnvController {
	return &EnvController{
		storage:    storage,
		ossStorage: ossStorage,
		ciTrigger:  ciTrigger,
	}
}

// EnvExistsResponse environment existence check response
type EnvExistsResponse struct {
	Exists bool             `json:"exists"`
	Status models.EnvStatus `json:"status,omitempty"`
}

// RegisterEnvRoutes registers routes
func (ctrl *EnvController) RegisterEnvRoutes(r *gin.Engine) {
	envGroup := r.Group("/env")
	{
		// Check if EnvName exists
		envGroup.GET("/:name/:version/exists", ctrl.EnvExists)

		// Create environment
		envGroup.POST("/", ctrl.CreateEnv)

		// Update environment
		envGroup.PUT("/:envName/:version", ctrl.UpdateEnv)

		// Get environment status
		envGroup.GET("/:name/:version/status", ctrl.GetEnvStatus)

		// Release environment
		envGroup.POST("/:name/:version/release", ctrl.ReleaseEnv)

		// Get environment by version
		envGroup.GET("/:name/:version", ctrl.GetEnvByVersion)

		// Get environment list
		envGroup.GET("/", ctrl.ListEnvs)

		// Generate environment storage URL signature
		envGroup.POST("/:name/:version/sign", ctrl.PresignEnv)

		envGroup.POST("/:name/:version/aci_trigger", ctrl.AciCallback)
	}
}

// EnvExists checks if EnvName exists
// GET /env/{name}/{version}/exists
func (ctrl *EnvController) EnvExists(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	key := fmt.Sprintf("%s-%s", name, version)

	env, _, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		// If it's a not found error, return not exists
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not exist") {
			models.JSONSuccess(c, EnvExistsResponse{
				Exists: false,
			})
			return
		}
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	models.JSONSuccess(c, EnvExistsResponse{
		Exists: true,
		Status: env.Status,
	})
}

// CreateEnv creates environment
// POST /env/
func (ctrl *EnvController) CreateEnv(c *gin.Context) {
	var req struct {
		Name         string                 `json:"name"`
		Version      string                 `json:"version"`
		Tags         []string               `json:"tags"`
		BuildConfig  map[string]interface{} `json:"buildConfig"`
		TestConfig   map[string]interface{} `json:"testConfig"`
		DeployConfig map[string]interface{} `json:"deployConfig"`
		Artifacts    []models.Artifact      `json:"artifacts"`
		Status       string                 `json:"status"`
		CodeUrl      string                 `json:"codeUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Invalid request format: "+err.Error())
		return
	}

	// Check if environment with same version already exists
	key := fmt.Sprintf("%s-%s", req.Name, req.Version)
	_, _, err := ctrl.storage.Get(c.Request.Context(), key)
	if err == nil {
		models.JSONErrorWithMessage(c, http.StatusConflict, "Environment with same name and version already exists")
		return
	}

	// Parse status
	status := models.EnvStatusByName(req.Status)

	// Create environment object
	env := models.NewEnv(key, req.Name, "", req.Version, req.CodeUrl)
	env.Tags = req.Tags
	env.BuildConfig = req.BuildConfig
	env.TestConfig = req.TestConfig
	env.DeployConfig = req.DeployConfig
	env.Status = status
	env.CodeURL = req.CodeUrl
	env.Artifacts = req.Artifacts

	// Set labels
	labels := map[string]string{
		"name":    req.Name,
		"version": req.Version,
	}

	// Store to database
	if err := ctrl.storage.Create(c.Request.Context(), key, env, labels); err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	// aci hook image build
	if ctrl.ciTrigger != nil {
		go ctrl.ciTrigger.Trigger(env)
	}

	models.JSONSuccess(c, true)
}

// UpdateEnv updates environment
// PUT /env/{envName}/{version}
func (ctrl *EnvController) UpdateEnv(c *gin.Context) {
	name := c.Param("envName")
	version := c.Param("version")

	key := fmt.Sprintf("%s-%s", name, version)

	// Check if environment exists
	env, resourceVersion, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}

	// Check environment status, if released then update is not allowed
	if env.Status == models.EnvStatusReleased {
		models.JSONErrorWithMessage(c, http.StatusForbidden, "Cannot update released environment")
		return
	}

	var req struct {
		Name         string                 `json:"name"`
		Version      string                 `json:"version"`
		Tags         []string               `json:"tags"`
		BuildConfig  map[string]interface{} `json:"buildConfig"`
		TestConfig   map[string]interface{} `json:"testConfig"`
		DeployConfig map[string]interface{} `json:"deployConfig"`
		Artifacts    []models.Artifact      `json:"artifacts"`
		Status       string                 `json:"status"`
		CodeUrl      string                 `json:"codeUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Invalid request format: "+err.Error())
		return
	}

	if req.Tags != nil {
		env.Tags = req.Tags
	}
	if req.Artifacts != nil {
		env.Artifacts = req.Artifacts
	}
	if req.BuildConfig != nil {
		env.BuildConfig = req.BuildConfig
	}
	if req.TestConfig != nil {
		env.TestConfig = req.TestConfig
	}
	if req.DeployConfig != nil {
		env.DeployConfig = req.DeployConfig
	}
	if req.CodeUrl != "" {
		env.CodeURL = req.CodeUrl
	}
	if req.Status != "" {
		env.Status = models.EnvStatusByName(req.Status)
	}
	env.UpdatedAt = time.Now()

	// Update labels
	labels := map[string]string{
		"name":    req.Name,
		"version": req.Version,
	}

	// Update to database
	if err := ctrl.storage.Update(c.Request.Context(), key, env, resourceVersion, labels); err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Code changes trigger image build
	if ctrl.ciTrigger != nil {
		go ctrl.ciTrigger.Trigger(env)
	}

	models.JSONSuccess(c, true)
}

// GetEnvStatus gets environment status
// GET /env/{name}/{version}/status
func (ctrl *EnvController) GetEnvStatus(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	key := fmt.Sprintf("%s-%s", name, version)

	env, _, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}

	models.JSONSuccess(c, map[string]string{
		"status": models.EnvStatusNameByStatus(env.Status),
	})
}

// ReleaseEnv releases environment
// POST /env/{name}/{version}/release
func (ctrl *EnvController) ReleaseEnv(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	key := fmt.Sprintf("%s-%s", name, version)

	// Get current environment
	env, resourceVersion, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}

	// Check if already in released status
	if env.Status == models.EnvStatusReleased {
		models.JSONErrorWithMessage(c, http.StatusForbidden, "Environment already released")
		return
	}

	// Update status to released
	env.Status = models.EnvStatusReleased
	env.UpdatedAt = time.Now()

	// Update labels
	labels := map[string]string{
		"name":    name,
		"version": version,
	}

	// Update to database
	if err := ctrl.storage.Update(c.Request.Context(), key, env, resourceVersion, labels); err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	models.JSONSuccess(c, true)
}

// GetEnvByVersion gets environment by version
// GET /env/{name}/{version}
func (ctrl *EnvController) GetEnvByVersion(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	key := fmt.Sprintf("%s-%s", name, version)

	env, _, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}

	models.JSONSuccess(c, env)
}

// ListEnvs gets environment list
// GET /env/
func (ctrl *EnvController) ListEnvs(c *gin.Context) {
	// Get all environment keys
	keys, err := ctrl.storage.List(c.Request.Context(), nil)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	var envs []interface{}
	for _, key := range keys {
		env, _, err := ctrl.storage.Get(c.Request.Context(), key)
		if err != nil {
			// Skip environments that failed to get
			continue
		}
		envs = append(envs, env)
	}

	models.JSONSuccess(c, envs)
}

func (ctrl *EnvController) PresignEnv(c *gin.Context) {
	if ctrl.ossStorage == nil {
		models.JSONErrorWithMessage(c, http.StatusServiceUnavailable, "OSS storage is not configured")
		return
	}
	name := c.Param("name")
	version := c.Param("version")
	style := c.Query("style")

	key := fmt.Sprintf("%s-%s", name, version)
	url, err := ctrl.ossStorage.PresignEnv(key, style)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	models.JSONSuccess(c, url)
}

func (ctrl *EnvController) AciCallback(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	type Call struct {
		Image string `json:"image"`
	}
	var call Call
	if err := c.ShouldBindJSON(&call); err != nil {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Invalid request format: "+err.Error())
	}
	imageUrl := call.Image
	if len(call.Image) == 0 {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Missing environment image message")
		return
	}

	key := fmt.Sprintf("%s-%s", name, version)
	// Check if environment exists
	env, resourceVersion, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}
	// Check environment status, if released then update is not allowed
	if env.Status == models.EnvStatusReleased {
		models.JSONErrorWithMessage(c, http.StatusForbidden, "Cannot update released environment")
		return
	}

	// If this update includes image field, build is complete and start reporting image, otherwise trigger pipeline
	haveChanged := false
	artifacts := env.Artifacts
	if artifacts == nil {
		artifacts = make([]models.Artifact, 0)
	}
	exist := false
	for idx, _ := range artifacts {
		if artifacts[idx].Type == "image" {
			exist = true
			if artifacts[idx].Content != imageUrl {
				artifacts[idx].Content = imageUrl
				haveChanged = true
			}
		}
	}
	if !exist {
		artifacts = append(artifacts, models.Artifact{
			Id:      "",
			Type:    "image",
			Content: imageUrl,
		})
		haveChanged = true
	}

	// After update, try to trigger build pipeline
	if !haveChanged {
		models.JSONSuccess(c, nil)
		return
	}
	env.Artifacts = artifacts
	env.UpdatedAt = time.Now()

	// Update labels
	labels := map[string]string{
		"name":    name,
		"version": version,
	}
	// Update to database
	if err := ctrl.storage.Update(c.Request.Context(), key, env, resourceVersion, labels); err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
}
