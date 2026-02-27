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
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Request counter
	requestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests made",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Request duration histogram
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Request size
	requestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 5),
		},
		[]string{"method", "endpoint"},
	)

	// Response size
	responseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 5),
		},
		[]string{"method", "endpoint"},
	)

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

// MetricsMiddleware metrics collection middleware
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Metrics collection after request processing completes
		statusCode := c.Writer.Status()
		duration := time.Since(start)
		responseSizeVal := float64(c.Writer.Size())
		requestSizeVal := float64(c.Request.ContentLength)

		// Record metrics
		requestCount.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
		requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

		if requestSizeVal >= 0 {
			requestSize.WithLabelValues(method, path).Observe(requestSizeVal)
		}

		if responseSizeVal >= 0 {
			responseSize.WithLabelValues(method, path).Observe(responseSizeVal)
		}

	}
}
