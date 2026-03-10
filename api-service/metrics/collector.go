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

	// Reset gauges to avoid stale data from deleted instances
	ActiveInstances.Reset()
	InstanceUptimeSeconds.Reset()

	now := time.Now().UnixMilli()
	for _, inst := range resp.Instances {
		if inst.Labels == nil {
			inst.Labels = make(map[string]string)
		}
		env := inst.Labels["envName"]
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

	log.Infof("metrics collector: updated metrics for %d instances", len(resp.Instances))
}
