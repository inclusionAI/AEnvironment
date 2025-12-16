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
	"envhub/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware Prometheus monitoring middleware
func MetricsMiddleware(metrics *models.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Record metrics
		statusCode := c.Writer.Status()
		duration := time.Since(start).Seconds()

		// Increment request count
		metrics.HttpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()

		// Record request duration
		metrics.HttpRequestDuration.WithLabelValues(method, path).Observe(duration)

		// Record environment-related operations
		if strings.HasPrefix(path, "/env/") {
			operation := getOperationFromPath(path, method)
			status := "success"
			if statusCode >= 400 {
				status = "failure"
			}
			metrics.EnvOperationsTotal.WithLabelValues(operation, status).Inc()
		}
	}
}

// getOperationFromPath gets operation name from path and method
func getOperationFromPath(path, method string) string {
	switch {
	case strings.Contains(path, "/exists"):
		return "exists"
	case strings.Contains(path, "/status"):
		return "status"
	case strings.Contains(path, "/release"):
		return "release"
	case path == "/env/" && method == "POST":
		return "create"
	case path == "/env/" && method == "GET":
		return "list"
	case strings.Count(path, "/") >= 3 && method == "PUT":
		return "update"
	case strings.Count(path, "/") >= 3 && method == "GET":
		return "get"
	default:
		return "unknown"
	}
}
