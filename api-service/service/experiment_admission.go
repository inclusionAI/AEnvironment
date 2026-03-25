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

package service

import (
	"api-service/metrics"
	"api-service/models"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// ExperimentAdmission implements first-come-first-served resource protection.
// Earlier experiments are guaranteed resources (historical peak instance count);
// only the newest experiments get rejected when resources are tight.
type ExperimentAdmission struct {
	mu             sync.RWMutex
	experiments    map[string]*ExperimentState
	clusterTotal   int64 // total CPU in milli-cores
	clusterUsed    int64 // used CPU in milli-cores
	hasClusterData bool  // whether we have received at least one cluster resource update

	// instanceExperiments maps instanceID → experiment name.
	// FaaS backend doesn't return user labels in ListInstances, so we track
	// the experiment assignment internally when instances are created.
	instanceExperiments map[string]string

	perInstanceCPU    int64         // CPU per instance in milli-cores
	peakWindow        time.Duration // sliding window for peak calculation
	schedulerEndpoint string        // faas-api-service HTTP API base URL (aggregated cluster info)
	pollInterval      time.Duration // cluster resource poll interval
	watermark         float64       // cluster utilization threshold (0.0-1.0), default 0.7
	requiredLabels    []string      // labels that must be present, default ["experiment"]

	httpClient    *http.Client
	pollFailCount int // consecutive poll failures (for log suppression)
}

// AdmissionResult contains the outcome of an admission decision.
type AdmissionResult struct {
	Allowed bool
	Reason  string
	Tier    string // "p0_known", "p1_new", "p2_unlabeled"
}

// ExperimentState tracks per-experiment instance counts and peak history.
type ExperimentState struct {
	FirstSeen    time.Time
	CurrentCount int
	PeakCount    int
	PeakSamples  []PeakSample // ring buffer for sliding window
}

// PeakSample records an instance count observation at a point in time.
type PeakSample struct {
	Timestamp time.Time
	Count     int
}

// clusterInfoData matches the "data" field of faas-api-service /clusterinfo response.
type clusterInfoData struct {
	TotalCPU          int64 `json:"totalCPU"`
	UsedCPU           int64 `json:"usedCPU"`
	FreeCPU           int64 `json:"freeCPU"`
	TotalMemory       int64 `json:"totalMemory"`
	UsedMemory        int64 `json:"usedMemory"`
	FreeMemory        int64 `json:"freeMemory"`
	HealthyPartitions int   `json:"healthyPartitions"`
	TotalPartitions   int   `json:"totalPartitions"`
}

// clusterInfoResponse matches the faas-api-service /clusterinfo JSON response.
type clusterInfoResponse struct {
	Success bool            `json:"success"`
	Data    clusterInfoData `json:"data"`
}

// NewExperimentAdmission creates a new admission controller.
func NewExperimentAdmission(schedulerEndpoint string, perInstanceCPU int64, peakWindow time.Duration, watermark float64, requiredLabels []string) *ExperimentAdmission {
	if watermark <= 0 || watermark > 1.0 {
		watermark = 0.7
	}
	if len(requiredLabels) == 0 {
		requiredLabels = []string{"experiment"}
	}
	return &ExperimentAdmission{
		experiments:         make(map[string]*ExperimentState),
		instanceExperiments: make(map[string]string),
		perInstanceCPU:      perInstanceCPU,
		peakWindow:          peakWindow,
		schedulerEndpoint:   schedulerEndpoint,
		pollInterval:        10 * time.Second,
		watermark:           watermark,
		requiredLabels:      requiredLabels,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// StartClusterResourcePoller runs a blocking loop that polls the scheduler
// for cluster resource data. Should be called in a goroutine.
func (ea *ExperimentAdmission) StartClusterResourcePoller() {
	log.Infof("Experiment admission: starting cluster info poller (faas-api-service=%s, interval=%v)", ea.schedulerEndpoint, ea.pollInterval)

	// Poll immediately on start
	ea.pollClusterResource()

	ticker := time.NewTicker(ea.pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		ea.pollClusterResource()
	}
}

func (ea *ExperimentAdmission) pollClusterResource() {
	url := ea.schedulerEndpoint + "/hapis/faas.hcs.io/v1/clusterinfo"
	resp, err := ea.httpClient.Get(url)
	if err != nil {
		ea.logPollFailure("failed to poll cluster info from faas-api-service: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ea.logPollFailure("faas-api-service returned status %d", resp.StatusCode)
		return
	}

	var cr clusterInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		ea.logPollFailure("failed to decode cluster info: %v", err)
		return
	}

	if !cr.Success {
		ea.logPollFailure("faas-api-service returned success=false")
		return
	}

	if cr.Data.HealthyPartitions == 0 && cr.Data.TotalPartitions > 0 {
		ea.logPollFailure("no healthy scheduler partitions (%d total)", cr.Data.TotalPartitions)
		return
	}

	ea.mu.Lock()
	ea.clusterTotal = cr.Data.TotalCPU
	ea.clusterUsed = cr.Data.UsedCPU
	ea.hasClusterData = true
	ea.pollFailCount = 0
	ea.mu.Unlock()

	// Update Prometheus gauges
	metrics.ClusterTotalCPU.Set(float64(cr.Data.TotalCPU))
	metrics.ClusterUsedCPU.Set(float64(cr.Data.UsedCPU))
	if cr.Data.TotalCPU > 0 {
		metrics.ClusterUtilization.Set(float64(cr.Data.UsedCPU) / float64(cr.Data.TotalCPU))
	}

	log.Debugf("Experiment admission: cluster resource updated (total=%d, used=%d, partitions=%d/%d)",
		cr.Data.TotalCPU, cr.Data.UsedCPU, cr.Data.HealthyPartitions, cr.Data.TotalPartitions)
}

// logPollFailure logs poll failures with exponential backoff suppression.
// Logs on 1st, 6th, 60th, 360th failure, etc. to avoid log spam.
func (ea *ExperimentAdmission) logPollFailure(format string, args ...interface{}) {
	ea.mu.Lock()
	ea.pollFailCount++
	count := ea.pollFailCount
	ea.mu.Unlock()

	if count == 1 || count == 6 || count%360 == 0 {
		msg := fmt.Sprintf(format, args...)
		log.Warnf("Experiment admission: %s (failure #%d, suppressing repeated warnings)", msg, count)
	}
}

// RegisterInstance records the experiment assignment for an instance.
// Called by middleware after a successful CreateEnvInstance.
func (ea *ExperimentAdmission) RegisterInstance(instanceID, experiment string) {
	if experiment == "" {
		experiment = "default"
	}
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.instanceExperiments[instanceID] = experiment
	log.Debugf("Experiment admission: registered instance %s → experiment %s (total tracked: %d)",
		instanceID, experiment, len(ea.instanceExperiments))
}

// UnregisterInstance removes the experiment assignment for an instance.
func (ea *ExperimentAdmission) UnregisterInstance(instanceID string) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	delete(ea.instanceExperiments, instanceID)
}

// UpdateExperimentCounts updates per-experiment instance counts from the
// periodic ListEnvInstances result. Called from startUnifiedPeriodicTask.
// Uses the internal instanceExperiments map to resolve experiment labels,
// since FaaS backend doesn't return user labels in ListInstances.
func (ea *ExperimentAdmission) UpdateExperimentCounts(instances []*models.EnvInstance) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Count instances per experiment
	counts := make(map[string]int)
	activeInstanceIDs := make(map[string]bool)
	for _, inst := range instances {
		if inst.Status == "Terminated" || inst.Status == "Failed" {
			continue
		}
		activeInstanceIDs[inst.ID] = true
		// Try internal map first (for FaaS mode where labels aren't returned)
		exp := ea.getInstanceExperimentLocked(inst)
		counts[exp]++
	}

	now := time.Now()

	// Update existing experiments and add new ones
	for exp, count := range counts {
		state, exists := ea.experiments[exp]
		if !exists {
			state = &ExperimentState{
				FirstSeen: now,
			}
			ea.experiments[exp] = state
		}
		state.CurrentCount = count
		state.PeakSamples = append(state.PeakSamples, PeakSample{
			Timestamp: now,
			Count:     count,
		})

		// Evict samples outside the sliding window
		ea.evictOldSamples(state, now)

		// Recalculate peak from remaining samples
		state.PeakCount = ea.calculatePeak(state)
	}

	// Remove experiments with zero active instances
	for exp, state := range ea.experiments {
		if _, active := counts[exp]; !active {
			state.CurrentCount = 0
			ea.evictOldSamples(state, now)
			state.PeakCount = ea.calculatePeak(state)

			// Remove if all historical samples have expired (peak decayed to 0)
			if len(state.PeakSamples) == 0 || state.PeakCount == 0 {
				delete(ea.experiments, exp)
			}
		}
	}

	// Clean up stale entries in instanceExperiments (instances no longer active)
	for id := range ea.instanceExperiments {
		if !activeInstanceIDs[id] {
			delete(ea.instanceExperiments, id)
		}
	}

	// Update Prometheus gauges
	metrics.ExperimentCount.Set(float64(len(ea.experiments)))
	metrics.ExperimentReservedCapacity.Set(float64(ea.reservedCapacity()))
	for exp, state := range ea.experiments {
		metrics.ExperimentPeakInstances.WithLabelValues(exp).Set(float64(state.PeakCount))
	}
}

// getInstanceExperimentLocked resolves the experiment name for an instance.
// Must be called with ea.mu held.
func (ea *ExperimentAdmission) getInstanceExperimentLocked(inst *models.EnvInstance) string {
	if exp, ok := ea.instanceExperiments[inst.ID]; ok {
		return exp
	}
	if exp := inst.Labels["experiment"]; exp != "" {
		return exp
	}
	return "default"
}

func (ea *ExperimentAdmission) evictOldSamples(state *ExperimentState, now time.Time) {
	cutoff := now.Add(-ea.peakWindow)
	i := 0
	for i < len(state.PeakSamples) && state.PeakSamples[i].Timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		state.PeakSamples = state.PeakSamples[i:]
	}
}

func (ea *ExperimentAdmission) calculatePeak(state *ExperimentState) int {
	peak := 0
	for _, s := range state.PeakSamples {
		if s.Count > peak {
			peak = s.Count
		}
	}
	return peak
}

// CheckRequiredLabels checks whether the given labels contain all required labels.
// Returns missing label names. Empty return means all labels are present.
func (ea *ExperimentAdmission) CheckRequiredLabels(labels map[string]string) []string {
	var missing []string
	for _, required := range ea.requiredLabels {
		if labels[required] == "" {
			missing = append(missing, required)
		}
	}
	return missing
}

// ShouldAdmit is a convenience wrapper returning (bool, string).
// Use ShouldAdmitWithResult for full tier information.
func (ea *ExperimentAdmission) ShouldAdmit(experimentID string) (bool, string) {
	result := ea.ShouldAdmitWithResult(experimentID)
	return result.Allowed, result.Reason
}

// ShouldAdmitWithResult decides whether a CreateEnvInstance request should be admitted.
// Returns an AdmissionResult with tier classification:
//   - p0_known: existing experiment, always admitted
//   - p1_new: new experiment, subject to watermark + capacity check
//   - p2_unlabeled: handled by middleware (CheckRequiredLabels)
func (ea *ExperimentAdmission) ShouldAdmitWithResult(experimentID string) AdmissionResult {
	if experimentID == "" {
		experimentID = "default"
	}

	ea.mu.RLock()
	defer ea.mu.RUnlock()

	// Fail-open: no cluster data yet (startup, scheduler unreachable)
	if !ea.hasClusterData {
		return AdmissionResult{Allowed: true, Tier: "p1_new"}
	}

	// P0: Known experiment — always admit
	if _, exists := ea.experiments[experimentID]; exists {
		return AdmissionResult{Allowed: true, Tier: "p0_known"}
	}

	// P1: New experiment — dual gate: watermark + reserved capacity

	// Gate 1: Cluster utilization watermark
	if ea.clusterTotal > 0 {
		utilization := float64(ea.clusterUsed) / float64(ea.clusterTotal)
		if utilization >= ea.watermark {
			return AdmissionResult{
				Allowed: false,
				Tier:    "p1_new",
				Reason: fmt.Sprintf(
					"Experiment admission denied: cluster utilization %.1f%% exceeds watermark %.1f%% for new experiment %q "+
						"(total=%d, used=%d milli-CPU)",
					utilization*100, ea.watermark*100, experimentID, ea.clusterTotal, ea.clusterUsed,
				),
			}
		}
	}

	// Gate 2: Reserved capacity check
	reserved := ea.reservedCapacity()
	available := ea.clusterTotal - reserved
	if available > ea.perInstanceCPU {
		return AdmissionResult{Allowed: true, Tier: "p1_new"}
	}

	return AdmissionResult{
		Allowed: false,
		Tier:    "p1_new",
		Reason: fmt.Sprintf(
			"Experiment admission denied: insufficient cluster capacity for new experiment %q "+
				"(total=%d, reserved=%d, available=%d, required=%d milli-CPU)",
			experimentID, ea.clusterTotal, reserved, available, ea.perInstanceCPU,
		),
	}
}

// reservedCapacity returns the total CPU reserved by all active experiments.
// Must be called with ea.mu held (at least RLock).
func (ea *ExperimentAdmission) reservedCapacity() int64 {
	var total int64
	for _, state := range ea.experiments {
		total += int64(state.PeakCount) * ea.perInstanceCPU
	}
	return total
}

// GetMetrics returns current admission state for debugging/observability.
func (ea *ExperimentAdmission) GetMetrics() map[string]interface{} {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	experiments := make(map[string]interface{})
	for exp, state := range ea.experiments {
		experiments[exp] = map[string]interface{}{
			"current_count": state.CurrentCount,
			"peak_count":    state.PeakCount,
			"first_seen":    state.FirstSeen,
			"samples":       len(state.PeakSamples),
		}
	}

	return map[string]interface{}{
		"cluster_total":     ea.clusterTotal,
		"cluster_used":      ea.clusterUsed,
		"has_cluster_data":  ea.hasClusterData,
		"per_instance_cpu":  ea.perInstanceCPU,
		"reserved_capacity": ea.reservedCapacity(),
		"watermark":         ea.watermark,
		"required_labels":   ea.requiredLabels,
		"experiment_count":  len(ea.experiments),
		"experiments":       experiments,
	}
}
