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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"api-service/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Auto cleanup metrics
	cleanupSuccessCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auto_cleanup_success_total",
			Help: "Total number of successfully auto-cleaned instances",
		},
	)

	cleanupFailureCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auto_cleanup_failure_total",
			Help: "Total number of failed auto-cleanup attempts",
		},
	)
)

// IncrementCleanupSuccess increments the cleanup success counter
func IncrementCleanupSuccess() {
	cleanupSuccessCount.Inc()
}

// IncrementCleanupFailure increments the cleanup failure counter
func IncrementCleanupFailure() {
	cleanupFailureCount.Inc()
}

// MetricsMiddleware records HTTP request metrics.
// Uses Gin's FullPath() for endpoint label, suitable for routers with named routes.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		status := fmt.Sprintf("%d", c.Writer.Status())
		method := c.Request.Method
		durationMs := float64(time.Since(start).Milliseconds())

		metrics.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		metrics.RequestDurationMs.WithLabelValues(method, endpoint, status).Observe(durationMs)
	}
}

// MCPMetricsMiddleware records MCP proxy request metrics.
// Uses actual URL path and extracts JSON-RPC method from POST body.
func MCPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var rpcMethod string
		if c.Request.Method == "POST" && c.Request.Body != nil {
			if body, err := io.ReadAll(c.Request.Body); err == nil && len(body) > 0 {
				var rpc struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &rpc) == nil {
					rpcMethod = rpc.Method
				}
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}

		start := time.Now()
		c.Next()

		endpoint := c.Request.URL.Path
		status := fmt.Sprintf("%d", c.Writer.Status())
		method := c.Request.Method
		durationMs := float64(time.Since(start).Milliseconds())

		metrics.MCPRequestsTotal.WithLabelValues(method, endpoint, rpcMethod, status).Inc()
		metrics.MCPRequestDurationMs.WithLabelValues(method, endpoint, rpcMethod, status).Observe(durationMs)
	}
}
