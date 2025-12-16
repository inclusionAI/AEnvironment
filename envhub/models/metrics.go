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

package models

import "github.com/prometheus/client_golang/prometheus"

// Metrics defines monitoring metrics
type Metrics struct {
	// HTTP request counter
	HttpRequestsTotal *prometheus.CounterVec

	// HTTP request duration
	HttpRequestDuration *prometheus.HistogramVec

	// Service health status
	ServiceHealth prometheus.Gauge

	// Environment operation counter
	EnvOperationsTotal *prometheus.CounterVec

	// Environment status counter
	EnvStatusTotal *prometheus.GaugeVec
}

// NewMetrics creates new monitoring metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		// HTTP request counter
		HttpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),

		// HTTP request duration
		HttpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		// Service health status
		ServiceHealth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "service_health_status",
				Help: "Service health status (1 = healthy, 0 = unhealthy)",
			},
		),

		// Environment operation counter
		EnvOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "env_operations_total",
				Help: "Total number of environment operations",
			},
			[]string{"operation", "status"},
		),

		// Environment status counter
		EnvStatusTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "env_status_count",
				Help: "Count of environments by status",
			},
			[]string{"status"},
		),
	}

	// Register metrics
	prometheus.MustRegister(
		m.HttpRequestsTotal,
		m.HttpRequestDuration,
		m.ServiceHealth,
		m.EnvOperationsTotal,
		m.EnvStatusTotal,
	)

	// Initialize health status as healthy
	m.ServiceHealth.Set(1)

	return m
}
