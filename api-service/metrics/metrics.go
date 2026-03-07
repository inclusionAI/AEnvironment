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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const subsystem = "aenv_api"

// BusinessLabelKeys defines the fixed business labels used for instance metrics.
// These labels are extracted from instance Labels map:
//   - env: Environment name (e.g., "terminal-0.1.0"), auto-set by system
//   - experiment: Experiment identifier (user-provided, optional)
//   - owner: Instance owner/creator (user-provided, optional)
//   - app: Application name (user-provided, optional)
//
// Missing labels will result in empty string values in metrics.
var BusinessLabelKeys = []string{"env", "experiment", "owner", "app"}

var (
	// HTTP request metrics
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDurationMs = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "request_duration_ms",
			Help:      "HTTP request duration in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 20),
		},
		[]string{"method", "endpoint", "status"},
	)

	// Business operation metrics
	InstanceOpsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "instance_operations_total",
			Help:      "Total instance operations count",
		},
		[]string{"operation", "env_name", "status"},
	)

	ServiceOpsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "service_operations_total",
			Help:      "Total service operations count",
		},
		[]string{"operation", "env_name", "status"},
	)

	// Instance state metrics (updated by periodic collector)
	ActiveInstances = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "active_instances",
			Help:      "Number of currently active instances",
		},
		BusinessLabelKeys,
	)

	InstanceUptimeSeconds = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "instance_uptime_seconds",
			Help:      "Instance uptime in seconds",
		},
		append([]string{"instance_id"}, BusinessLabelKeys...),
	)

	// MCP proxy metrics with rpc_method to distinguish JSON-RPC operations
	MCPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "mcp_requests_total",
			Help:      "Total number of MCP proxy requests",
		},
		[]string{"method", "endpoint", "rpc_method", "status"},
	)

	MCPRequestDurationMs = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "mcp_request_duration_ms",
			Help:      "MCP proxy request duration in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 20),
		},
		[]string{"method", "endpoint", "rpc_method", "status"},
	)
)
