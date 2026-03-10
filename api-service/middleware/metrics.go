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
	"strings"
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
// Extracts JSON-RPC method from POST body with size limit, normalizes endpoint
// labels, sets GetBody for reverse proxy rewind, and skips duration for SSE.
func MCPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		isSSE := strings.HasSuffix(path, "/sse")

		var rpcMethod string
		if c.Request.Method == "POST" && c.Request.Body != nil {
			// Read up to 8KB+1 to detect oversized bodies; JSON-RPC method
			// field is always in the first few dozen bytes.
			const maxPeek = 8192
			limited := io.LimitReader(c.Request.Body, maxPeek+1)
			if body, err := io.ReadAll(limited); err == nil && len(body) > 0 {
				var rpc struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &rpc) == nil {
					rpcMethod = rpc.Method
				}

				// Fast path (99%+ requests): body fits within limit, skip
				// second ReadAll and append entirely.
				var fullBody []byte
				if len(body) <= maxPeek {
					fullBody = body
				} else {
					// Cap remaining read at 1MB to prevent memory exhaustion
					// from abnormally large requests.
					const maxBody = 1 << 20
					remaining, _ := io.ReadAll(io.LimitReader(c.Request.Body, maxBody))
					fullBody = append(body, remaining...)
				}

				c.Request.Body = io.NopCloser(bytes.NewReader(fullBody))
				c.Request.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(fullBody)), nil
				}
				c.Request.ContentLength = int64(len(fullBody))
			}
		}
		c.Set("_rpc_method", rpcMethod)

		endpoint := normalizeEndpoint(path)

		start := time.Now()
		c.Next()

		status := fmt.Sprintf("%d", c.Writer.Status())
		method := c.Request.Method

		metrics.MCPRequestsTotal.WithLabelValues(method, endpoint, rpcMethod, status).Inc()

		// Skip duration histogram for SSE (long-lived connections pollute buckets)
		if !isSSE {
			durationMs := float64(time.Since(start).Milliseconds())
			metrics.MCPRequestDurationMs.WithLabelValues(method, endpoint, rpcMethod, status).Observe(durationMs)
		}
	}
}

// normalizeEndpoint maps raw URL path to a bounded set of known MCP paths
// to prevent Prometheus label cardinality explosion from wildcard routes.
func normalizeEndpoint(path string) string {
	switch {
	case strings.HasSuffix(path, "/sse"):
		return "/sse"
	case strings.HasSuffix(path, "/mcp"):
		return "/mcp"
	case strings.HasSuffix(path, "/message"):
		return "/message"
	case path == "/health":
		return "/health"
	default:
		return "/other"
	}
}
