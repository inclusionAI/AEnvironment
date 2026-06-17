package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"envhub/models"
)

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
	imageURL := call.Image
	if len(call.Image) == 0 {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Missing environment image message")
		return
	}

	key := fmt.Sprintf("%s-%s", name, version)
	env, resourceVersion, err := ctrl.storage.Get(c.Request.Context(), key)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusNotFound, "Environment not found")
		return
	}
	if env.Status == models.EnvStatusReleased {
		models.JSONErrorWithMessage(c, http.StatusForbidden, "Cannot update released environment")
		return
	}

	haveChanged := false
	artifacts := env.Artifacts
	if artifacts == nil {
		artifacts = make([]models.Artifact, 0)
	}
	exist := false
	for idx := range artifacts {
		if artifacts[idx].Type == "image" {
			exist = true
			if artifacts[idx].Content != imageURL {
				artifacts[idx].Content = imageURL
				haveChanged = true
			}
		}
	}
	if !exist {
		artifacts = append(artifacts, models.Artifact{
			Id:      "",
			Type:    "image",
			Content: imageURL,
		})
		haveChanged = true
	}

	if !haveChanged {
		models.JSONSuccess(c, nil)
		return
	}
	env.Artifacts = artifacts
	env.UpdatedAt = time.Now()

	labels := map[string]string{
		"name":    name,
		"version": version,
	}
	if err := ctrl.storage.Update(c.Request.Context(), key, env, resourceVersion, labels); err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
}
