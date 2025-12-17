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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	namespace = "aenv"
)

var (
	k8sApiCallLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "k8s_api_call_latency",
			Help:      "the latency for calling k8s api",
			Buckets:   []float64{5, 10, 30, 60, 100, 300, 600, 1000, 3000, 6000, 10000, 30000, 60000, 600000},
		},
		[]string{"method", "object_type"},
	)
)

func init() {
	metrics.Registry.MustRegister(k8sApiCallLatency)
}

// RecordK8sApiCallLatency records latency
// get the time since the specified start in milliseconds.
func RecordK8sApiCallLatency(method string, objectType string, startTime time.Time) {
	k8sApiCallLatency.WithLabelValues(method, objectType).Observe(float64(time.Since(startTime).Nanoseconds() / time.Millisecond.Nanoseconds()))
}
