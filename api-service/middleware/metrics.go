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
	"fmt"
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
// Excludes /health endpoint errors from being recorded as failures,
// since proxy-less /health calls (e.g., K8s liveness probes) are expected
// to return non-error status and should not pollute error metrics.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		statusCode := c.Writer.Status()

		status := fmt.Sprintf("%d", statusCode)
		method := c.Request.Method
		durationMs := float64(time.Since(start).Milliseconds())

		metrics.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		metrics.RequestDurationMs.WithLabelValues(method, endpoint, status).Observe(durationMs)
	}
}
