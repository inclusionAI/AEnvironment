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

var (
	// ExperimentAdmissionTotal counts admission decisions by tier.
	ExperimentAdmissionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "experiment_admission_total",
			Help:      "Total experiment admission decisions",
		},
		[]string{"decision", "tier"}, // decision: "allowed"|"rejected", tier: "p0_known"|"p1_new"|"p2_unlabeled"
	)

	// ExperimentReservedCapacity tracks total reserved CPU in milli-cores.
	ExperimentReservedCapacity = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "experiment_reserved_capacity",
			Help:      "Total reserved CPU capacity across active experiments (milli-cores)",
		},
	)

	// ExperimentCount tracks number of active experiments.
	ExperimentCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "experiment_count",
			Help:      "Number of currently active experiments",
		},
	)

	// ExperimentPeakInstances tracks per-experiment peak instance count.
	ExperimentPeakInstances = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "experiment_peak_instances",
			Help:      "Peak instance count per experiment in sliding window",
		},
		[]string{"experiment"},
	)

	// ClusterTotalCPU tracks total cluster CPU in milli-cores.
	ClusterTotalCPU = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "cluster_total_cpu",
			Help:      "Total cluster CPU capacity (milli-cores)",
		},
	)

	// ClusterUsedCPU tracks used cluster CPU in milli-cores.
	ClusterUsedCPU = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "cluster_used_cpu",
			Help:      "Used cluster CPU (milli-cores)",
		},
	)

	// ClusterUtilization tracks cluster CPU utilization ratio.
	ClusterUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "cluster_utilization",
			Help:      "Cluster CPU utilization ratio (0.0-1.0)",
		},
	)
)
