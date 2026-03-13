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
	"api-service/models"
	"api-service/service/faas_model"
	"time"

	log "github.com/sirupsen/logrus"
)

// InstanceLister abstracts the ability to list instances (for testability)
type InstanceLister interface {
	ListInstances(labels map[string]string) (*faas_model.InstanceListResp, error)
}

// Collector periodically polls active instances and updates Gauge metrics
type Collector struct {
	lister   InstanceLister
	interval time.Duration
	stopCh   chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(lister InstanceLister, interval time.Duration) *Collector {
	return &Collector{
		lister:   lister,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic collection loop (blocking, run in goroutine)
func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect once at startup
	c.collect()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopCh:
			return
		}
	}
}

// Stop halts the collector
func (c *Collector) Stop() {
	close(c.stopCh)
}

func (c *Collector) collect() {
	resp, err := c.lister.ListInstances(nil)
	if err != nil {
		log.Warnf("metrics collector: failed to list instances: %v", err)
		return
	}
	if resp == nil {
		return
	}

	c.CollectFromInstances(resp.Instances)
}

// CollectFromInstances updates metrics from a pre-fetched instance list.
// This allows callers to share the same ListInstances result across multiple consumers.
func (c *Collector) CollectFromInstances(instances []*faas_model.Instance) {
	// Reset gauges to avoid stale data from deleted instances
	ActiveInstances.Reset()
	InstanceUptimeSeconds.Reset()

	now := time.Now().UnixMilli()
	for _, inst := range instances {
		if inst.Labels == nil {
			inst.Labels = make(map[string]string)
		}
		env := inst.Labels["env"]
		experiment := inst.Labels["experiment"]
		owner := inst.Labels["owner"]
		app := inst.Labels["app"]

		// Increment active instance count
		ActiveInstances.WithLabelValues(env, experiment, owner, app).Inc()

		// Set uptime
		if inst.CreateTimestamp > 0 {
			uptimeSec := float64(now-inst.CreateTimestamp) / 1000.0
			InstanceUptimeSeconds.WithLabelValues(
				inst.InstanceID, env, experiment, owner, app,
			).Set(uptimeSec)
		}
	}

	log.Infof("metrics collector: updated metrics for %d instances", len(instances))
}

// CollectFromEnvInstances updates metrics from a pre-fetched EnvInstance list.
// This handles the type difference between models.EnvInstance (used by ListEnvInstances)
// and faas_model.Instance (used by FaaSClient.ListInstances).
func (c *Collector) CollectFromEnvInstances(envInstances []*models.EnvInstance) {
	ActiveInstances.Reset()
	InstanceUptimeSeconds.Reset()

	now := time.Now()
	for _, inst := range envInstances {
		labels := inst.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		env := labels["env"]
		experiment := labels["experiment"]
		owner := labels["owner"]
		app := labels["app"]

		ActiveInstances.WithLabelValues(env, experiment, owner, app).Inc()

		if inst.CreatedAt != "" {
			createdAt, err := time.Parse(time.DateTime, inst.CreatedAt)
			if err != nil {
				createdAt, err = time.Parse(time.RFC3339, inst.CreatedAt)
			}
			if err == nil {
				uptimeSec := now.Sub(createdAt).Seconds()
				InstanceUptimeSeconds.WithLabelValues(
					inst.ID, env, experiment, owner, app,
				).Set(uptimeSec)
			}
		}
	}

	log.Infof("metrics collector: updated metrics for %d env instances", len(envInstances))
}
