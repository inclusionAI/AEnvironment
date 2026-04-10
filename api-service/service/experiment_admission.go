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
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

// experimentFormatRegex validates the {owner}/{name} format.
var experimentFormatRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+$`)

const experimentCountsKey = "exp:counts"

// ExperimentAdmission implements resource protection with three rules:
//  1. experiment label required, format {owner}/{name}
//  2. per-experiment instance count <= maxInstances
//  3. cluster utilization >= watermark → reject new experiments
//
// State is stored in Redis (exp:counts hash) for multi-instance consistency.
// A local cache (experiments map) serves the hot read path (ShouldAdmit)
// with zero Redis calls. Redis is updated atomically on create/delete,
// and a periodic sync reconciles any drift.
type ExperimentAdmission struct {
	mu             sync.RWMutex
	experiments    map[string]*ExperimentState
	clusterTotal   int64
	clusterUsed    int64
	hasClusterData bool

	// instanceExperiments maps instanceID → experiment name.
	// FaaS backend doesn't return user labels in ListInstances, so we track
	// the experiment assignment internally when instances are created.
	instanceExperiments map[string]string

	redisClient       *RedisClient
	maxInstances      int
	watermark         float64
	schedulerEndpoint string
	pollInterval      time.Duration

	httpClient    *http.Client
	pollFailCount int
}

// AdmissionResult contains the outcome of an admission decision.
type AdmissionResult struct {
	Allowed bool
	Reason  string
	Tier    string // "p0_known", "p1_new"
}

// ExperimentState tracks per-experiment instance count.
type ExperimentState struct {
	FirstSeen    time.Time
	CurrentCount int
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
// redisClient may be nil for single-instance deployments (local-only mode).
func NewExperimentAdmission(schedulerEndpoint string, maxInstances int, watermark float64, redisClient *RedisClient) *ExperimentAdmission {
	if watermark <= 0 || watermark > 1.0 {
		watermark = 0.5
	}
	if maxInstances <= 0 {
		maxInstances = 1000
	}
	ea := &ExperimentAdmission{
		experiments:         make(map[string]*ExperimentState),
		instanceExperiments: make(map[string]string),
		redisClient:         redisClient,
		maxInstances:        maxInstances,
		watermark:           watermark,
		schedulerEndpoint:   schedulerEndpoint,
		pollInterval:        10 * time.Second,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	ea.warmupFromRedis()
	return ea
}

// ValidateExperimentFormat checks whether the experiment label matches {owner}/{name}.
func ValidateExperimentFormat(experiment string) error {
	if !experimentFormatRegex.MatchString(experiment) {
		return fmt.Errorf("invalid experiment format %q, expected {owner}/{name} (e.g. zhangsan/swe-bench-lite)", experiment)
	}
	return nil
}

// warmupFromRedis loads experiment counts from Redis into the local cache on startup.
func (ea *ExperimentAdmission) warmupFromRedis() {
	if ea.redisClient == nil {
		return
	}
	result, err := ea.redisClient.client.HGetAll(ea.redisClient.ctx, experimentCountsKey).Result()
	if err != nil {
		log.Warnf("Experiment admission: failed to warmup from Redis: %v", err)
		return
	}
	now := time.Now()
	for exp, countStr := range result {
		count, err := strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			continue
		}
		ea.experiments[exp] = &ExperimentState{
			FirstSeen:    now,
			CurrentCount: count,
		}
	}
	log.Infof("Experiment admission: warmed up %d experiments from Redis", len(ea.experiments))
}

// StartClusterResourcePoller runs a blocking loop that polls the scheduler
// for cluster resource data. Should be called in a goroutine.
func (ea *ExperimentAdmission) StartClusterResourcePoller() {
	log.Infof("Experiment admission: starting cluster info poller (faas-api-service=%s, interval=%v)", ea.schedulerEndpoint, ea.pollInterval)

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

	metrics.ClusterTotalCPU.Set(float64(cr.Data.TotalCPU))
	metrics.ClusterUsedCPU.Set(float64(cr.Data.UsedCPU))
	if cr.Data.TotalCPU > 0 {
		metrics.ClusterUtilization.Set(float64(cr.Data.UsedCPU) / float64(cr.Data.TotalCPU))
	}

	log.Debugf("Experiment admission: cluster resource updated (total=%d, used=%d, partitions=%d/%d)",
		cr.Data.TotalCPU, cr.Data.UsedCPU, cr.Data.HealthyPartitions, cr.Data.TotalPartitions)
}

// logPollFailure logs poll failures with exponential backoff suppression.
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

// RegisterInstance records the experiment assignment for an instance and
// atomically increments the experiment count in Redis.
func (ea *ExperimentAdmission) RegisterInstance(instanceID, experiment string) {
	ea.mu.Lock()
	ea.instanceExperiments[instanceID] = experiment

	// Update local cache
	state, exists := ea.experiments[experiment]
	if !exists {
		state = &ExperimentState{FirstSeen: time.Now()}
		ea.experiments[experiment] = state
	}
	state.CurrentCount++
	ea.mu.Unlock()

	// Atomic Redis increment (non-blocking on failure)
	if ea.redisClient != nil {
		newCount, err := ea.redisClient.client.HIncrBy(ea.redisClient.ctx, experimentCountsKey, experiment, 1).Result()
		if err != nil {
			log.Warnf("Experiment admission: Redis HINCRBY failed for %s: %v (local-only mode)", experiment, err)
			return
		}
		// Refresh local cache with authoritative Redis count
		ea.mu.Lock()
		if s, ok := ea.experiments[experiment]; ok {
			s.CurrentCount = int(newCount)
		}
		ea.mu.Unlock()
	}

	log.Debugf("Experiment admission: registered instance %s → experiment %s", instanceID, experiment)
}

// UnregisterInstance removes the experiment assignment for an instance and
// atomically decrements the experiment count in Redis.
func (ea *ExperimentAdmission) UnregisterInstance(instanceID string) {
	ea.mu.Lock()
	experiment, ok := ea.instanceExperiments[instanceID]
	if !ok {
		ea.mu.Unlock()
		return
	}
	delete(ea.instanceExperiments, instanceID)

	// Update local cache
	if state, exists := ea.experiments[experiment]; exists {
		state.CurrentCount--
		if state.CurrentCount <= 0 {
			delete(ea.experiments, experiment)
		}
	}
	ea.mu.Unlock()

	// Atomic Redis decrement
	if ea.redisClient != nil {
		newCount, err := ea.redisClient.client.HIncrBy(ea.redisClient.ctx, experimentCountsKey, experiment, -1).Result()
		if err != nil {
			log.Warnf("Experiment admission: Redis HINCRBY(-1) failed for %s: %v", experiment, err)
			return
		}
		if newCount <= 0 {
			ea.redisClient.client.HDel(ea.redisClient.ctx, experimentCountsKey, experiment)
		}
		// Refresh local cache with authoritative Redis count
		ea.mu.Lock()
		if newCount <= 0 {
			delete(ea.experiments, experiment)
		} else if s, ok := ea.experiments[experiment]; ok {
			s.CurrentCount = int(newCount)
		}
		ea.mu.Unlock()
	}

	log.Debugf("Experiment admission: unregistered instance %s (experiment %s)", instanceID, experiment)
}

// UpdateExperimentCounts updates per-experiment instance counts from the
// periodic ListEnvInstances result. Reconciles both Redis and local cache.
func (ea *ExperimentAdmission) UpdateExperimentCounts(instances []*models.EnvInstance) {
	ea.mu.Lock()

	counts := make(map[string]int)
	activeInstanceIDs := make(map[string]bool)
	for _, inst := range instances {
		if inst.Status == "Terminated" || inst.Status == "Failed" {
			continue
		}
		activeInstanceIDs[inst.ID] = true
		exp := ea.getInstanceExperimentLocked(inst)
		counts[exp]++
	}

	now := time.Now()

	// Rebuild local cache from actual counts
	newExperiments := make(map[string]*ExperimentState, len(counts))
	for exp, count := range counts {
		state, exists := ea.experiments[exp]
		if !exists {
			state = &ExperimentState{FirstSeen: now}
		}
		state.CurrentCount = count
		newExperiments[exp] = state
	}
	ea.experiments = newExperiments

	// Clean up stale entries in instanceExperiments
	for id := range ea.instanceExperiments {
		if !activeInstanceIDs[id] {
			delete(ea.instanceExperiments, id)
		}
	}

	ea.mu.Unlock()

	// Sync to Redis: full overwrite via pipeline
	if ea.redisClient != nil {
		ea.syncCountsToRedis(counts)
	}

	metrics.ExperimentCount.Set(float64(len(counts)))
}

// syncCountsToRedis overwrites the Redis hash with authoritative counts.
func (ea *ExperimentAdmission) syncCountsToRedis(counts map[string]int) {
	pipe := ea.redisClient.client.Pipeline()
	pipe.Del(ea.redisClient.ctx, experimentCountsKey)
	if len(counts) > 0 {
		fields := make(map[string]interface{}, len(counts))
		for exp, count := range counts {
			fields[exp] = count
		}
		pipe.HSet(ea.redisClient.ctx, experimentCountsKey, fields)
	}
	if _, err := pipe.Exec(ea.redisClient.ctx); err != nil && err != redis.Nil {
		log.Warnf("Experiment admission: Redis sync pipeline failed: %v", err)
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

// ShouldAdmit is a convenience wrapper returning (bool, string).
func (ea *ExperimentAdmission) ShouldAdmit(experimentID string) (bool, string) {
	result := ea.ShouldAdmitWithResult(experimentID)
	return result.Allowed, result.Reason
}

// ShouldAdmitWithResult decides whether a CreateEnvInstance request should be admitted.
// Reads only from the local cache (zero Redis calls on the hot path).
//   - p0_known: existing experiment, subject to max_instances check
//   - p1_new: new experiment, subject to watermark check
func (ea *ExperimentAdmission) ShouldAdmitWithResult(experimentID string) AdmissionResult {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	// Fail-open: no cluster data yet
	if !ea.hasClusterData {
		return AdmissionResult{Allowed: true, Tier: "p1_new"}
	}

	// P0: Known experiment — check instance cap
	if state, exists := ea.experiments[experimentID]; exists {
		if state.CurrentCount >= ea.maxInstances {
			return AdmissionResult{
				Allowed: false,
				Tier:    "p0_known",
				Reason: fmt.Sprintf(
					"Experiment admission denied: experiment %q instance count %d reached limit %d",
					experimentID, state.CurrentCount, ea.maxInstances,
				),
			}
		}
		return AdmissionResult{Allowed: true, Tier: "p0_known"}
	}

	// P1: New experiment — watermark gate
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

	return AdmissionResult{Allowed: true, Tier: "p1_new"}
}

// GetMetrics returns current admission state for debugging/observability.
func (ea *ExperimentAdmission) GetMetrics() map[string]interface{} {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	experiments := make(map[string]interface{})
	for exp, state := range ea.experiments {
		experiments[exp] = map[string]interface{}{
			"current_count": state.CurrentCount,
			"first_seen":    state.FirstSeen,
		}
	}

	return map[string]interface{}{
		"cluster_total":    ea.clusterTotal,
		"cluster_used":     ea.clusterUsed,
		"has_cluster_data": ea.hasClusterData,
		"max_instances":    ea.maxInstances,
		"watermark":        ea.watermark,
		"experiment_count": len(ea.experiments),
		"experiments":      experiments,
	}
}
