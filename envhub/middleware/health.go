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

package middleware

import (
	"context"
	"envhub/controller"
	"envhub/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheckMiddleware health check middleware.
func HealthCheckMiddleware(metrics *models.Metrics, healthChecker controller.HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()

			if err := healthChecker.CheckHealth(ctx); err != nil {
				metrics.ServiceHealth.Set(0)
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status":  "unhealthy",
					"message": err.Error(),
				})
				return
			}

			metrics.ServiceHealth.Set(1)
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
			})
			return
		}

		c.Next()
	}
}
