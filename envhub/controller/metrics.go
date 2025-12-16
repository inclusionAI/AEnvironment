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
	"envhub/models"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsController Prometheus metrics controller
type MetricsController struct {
	metrics *models.Metrics
}

// NewMetricsController creates metrics controller
func NewMetricsController(metrics *models.Metrics) *MetricsController {
	return &MetricsController{
		metrics: metrics,
	}
}

// UpdateEnvStatusMetrics updates environment status metrics
func (mc *MetricsController) UpdateEnvStatusMetrics(statusCounts map[string]int) {
	for status, count := range statusCounts {
		mc.metrics.EnvStatusTotal.WithLabelValues(status).Set(float64(count))
	}
}

// PrometheusHandler returns Prometheus metrics handler
func (mc *MetricsController) PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
