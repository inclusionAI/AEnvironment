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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"envhub/models"
	"envhub/service"
)

// HealthChecker health check interface
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// HealthController health check controller
type HealthController struct {
	metrics       *models.Metrics
	healthChecker HealthChecker
}

// NewHealthController creates health check controller
func NewHealthController(metrics *models.Metrics, healthChecker HealthChecker) *HealthController {
	return &HealthController{
		metrics:       metrics,
		healthChecker: healthChecker,
	}
}

// Health checks service health status
func (hc *HealthController) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := hc.healthChecker.CheckHealth(ctx); err != nil {
		hc.metrics.ServiceHealth.Set(0)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"message": err.Error(),
		})
		return
	}

	hc.metrics.ServiceHealth.Set(1)
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// EnvStorageHealthChecker mock health checker (needs to be replaced with real check logic in actual use)
type EnvStorageHealthChecker struct {
	storage service.EnvStorage
}

func NewEnvStorageHealthChecker(storage service.EnvStorage) *EnvStorageHealthChecker {
	return &EnvStorageHealthChecker{
		storage: storage,
	}
}

func (mhc *EnvStorageHealthChecker) CheckHealth(ctx context.Context) error {
	// Here you can add actual health check logic
	// For example: check database connection, check dependent services, etc.

	// Simple example: try to list environments to check if storage service is normal
	_, err := mhc.storage.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage service unavailable: %v", err)
	}

	return nil
}
