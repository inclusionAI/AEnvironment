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
	"api-service/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// newTestAdmission creates an ExperimentAdmission with cluster data pre-set (no Redis).
func newTestAdmission(totalCPU, usedCPU int64, maxInstances int, watermark float64) *ExperimentAdmission {
	ea := NewExperimentAdmission("http://localhost:0", maxInstances, watermark, nil)
	ea.mu.Lock()
	ea.clusterTotal = totalCPU
	ea.clusterUsed = usedCPU
	ea.hasClusterData = true
	ea.mu.Unlock()
	return ea
}

// newTestAdmissionWithRedis creates an ExperimentAdmission backed by miniredis.
func newTestAdmissionWithRedis(t *testing.T, totalCPU, usedCPU int64, maxInstances int, watermark float64) (*ExperimentAdmission, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	rc := &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ctx:    context.Background(),
	}
	ea := NewExperimentAdmission("http://localhost:0", maxInstances, watermark, rc)
	ea.mu.Lock()
	ea.clusterTotal = totalCPU
	ea.clusterUsed = usedCPU
	ea.hasClusterData = true
	ea.mu.Unlock()
	return ea, mr
}

func makeInstances(counts map[string]int) []*models.EnvInstance {
	var instances []*models.EnvInstance
	id := 0
	for exp, count := range counts {
		for i := 0; i < count; i++ {
			id++
			instances = append(instances, &models.EnvInstance{
				ID:     fmt.Sprintf("inst-%d", id),
				Status: "Running",
				Labels: map[string]string{"experiment": exp},
			})
		}
	}
	return instances
}

// --- Format Validation Tests ---

func TestValidateExperimentFormat_Valid(t *testing.T) {
	valid := []string{
		"zhangsan/swe-bench-lite",
		"team-a/eval-v2",
		"user_1/test.run",
		"owner/name",
	}
	for _, v := range valid {
		assert.NoError(t, ValidateExperimentFormat(v), "should be valid: %s", v)
	}
}

func TestValidateExperimentFormat_Invalid(t *testing.T) {
	invalid := []string{
		"swe-bench-lite", // no owner
		"/swe-bench",     // empty owner
		"owner/",         // empty name
		"a/b/c",          // too many segments
		"",               // empty
		"owner/ name",    // space
		"owner/name!",    // special char
	}
	for _, v := range invalid {
		assert.Error(t, ValidateExperimentFormat(v), "should be invalid: %s", v)
	}
}

// --- Max Instances Tests ---

func TestKnownExperimentAdmittedUnderLimit(t *testing.T) {
	ea := newTestAdmission(100000, 90000, 100, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 50}

	result := ea.ShouldAdmitWithResult("owner/exp-a")
	assert.True(t, result.Allowed)
	assert.Equal(t, "p0_known", result.Tier)
}

func TestKnownExperimentRejectedAtLimit(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 100, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 100}

	result := ea.ShouldAdmitWithResult("owner/exp-a")
	assert.False(t, result.Allowed)
	assert.Equal(t, "p0_known", result.Tier)
	assert.Contains(t, result.Reason, "instance count 100 reached limit 100")
}

func TestKnownExperimentRejectedOverLimit(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 100, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 150}

	result := ea.ShouldAdmitWithResult("owner/exp-a")
	assert.False(t, result.Allowed)
}

// --- Watermark Tests ---

func TestNewExperimentAdmittedBelowWatermark(t *testing.T) {
	ea := newTestAdmission(100000, 40000, 1000, 0.5)

	result := ea.ShouldAdmitWithResult("owner/new-exp")
	assert.True(t, result.Allowed)
	assert.Equal(t, "p1_new", result.Tier)
}

func TestNewExperimentRejectedAtWatermark(t *testing.T) {
	ea := newTestAdmission(100000, 50000, 1000, 0.5)

	result := ea.ShouldAdmitWithResult("owner/new-exp")
	assert.False(t, result.Allowed)
	assert.Equal(t, "p1_new", result.Tier)
	assert.Contains(t, result.Reason, "watermark")
}

func TestNewExperimentRejectedAboveWatermark(t *testing.T) {
	ea := newTestAdmission(100000, 80000, 1000, 0.5)

	result := ea.ShouldAdmitWithResult("owner/new-exp")
	assert.False(t, result.Allowed)
	assert.Equal(t, "p1_new", result.Tier)
}

func TestKnownExperimentAdmittedEvenAboveWatermark(t *testing.T) {
	ea := newTestAdmission(100000, 80000, 1000, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 50}

	result := ea.ShouldAdmitWithResult("owner/exp-a")
	assert.True(t, result.Allowed)
	assert.Equal(t, "p0_known", result.Tier)
}

// --- Fail-Open Tests ---

func TestFailOpenWhenNoClusterData(t *testing.T) {
	ea := NewExperimentAdmission("http://localhost:0", 1000, 0.5, nil)

	result := ea.ShouldAdmitWithResult("owner/any-exp")
	assert.True(t, result.Allowed)
}

// --- UpdateExperimentCounts Tests ---

func TestUpdateExperimentCountsBasic(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 1000, 0.5)

	instances := makeInstances(map[string]int{
		"owner/exp-a": 5,
		"owner/exp-b": 3,
	})
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, 5, ea.experiments["owner/exp-a"].CurrentCount)
	assert.Equal(t, 3, ea.experiments["owner/exp-b"].CurrentCount)
}

func TestUpdateExperimentCountsRemovesZero(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 1000, 0.5)

	instances := makeInstances(map[string]int{"owner/exp-a": 5})
	ea.UpdateExperimentCounts(instances)

	ea.UpdateExperimentCounts(nil)

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	_, exists := ea.experiments["owner/exp-a"]
	assert.False(t, exists, "experiment should be removed when count drops to 0")
}

func TestTerminatedInstancesNotCounted(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 1000, 0.5)

	instances := []*models.EnvInstance{
		{ID: "1", Status: "Running", Labels: map[string]string{"experiment": "owner/exp"}},
		{ID: "2", Status: "Running", Labels: map[string]string{"experiment": "owner/exp"}},
		{ID: "3", Status: "Terminated", Labels: map[string]string{"experiment": "owner/exp"}},
		{ID: "4", Status: "Failed", Labels: map[string]string{"experiment": "owner/exp"}},
	}
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, 2, ea.experiments["owner/exp"].CurrentCount)
}

func TestInstanceExperimentTracking(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 1000, 0.5)

	ea.RegisterInstance("inst-1", "owner/exp-a")
	ea.RegisterInstance("inst-2", "owner/exp-a")
	ea.RegisterInstance("inst-3", "owner/exp-b")

	// Simulate ListInstances without labels (FaaS mode)
	instances := []*models.EnvInstance{
		{ID: "inst-1", Status: "Running", Labels: map[string]string{}},
		{ID: "inst-2", Status: "Running", Labels: map[string]string{}},
		{ID: "inst-3", Status: "Running", Labels: map[string]string{}},
	}
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, 2, ea.experiments["owner/exp-a"].CurrentCount)
	assert.Equal(t, 1, ea.experiments["owner/exp-b"].CurrentCount)
}

// --- Concurrent Access Test ---

func TestConcurrentAccess(t *testing.T) {
	ea := newTestAdmission(100000, 30000, 1000, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 10}

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ea.ShouldAdmit("owner/exp-a")
			ea.ShouldAdmit("owner/new-exp")
			ea.GetMetrics()
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			instances := makeInstances(map[string]int{"owner/exp-a": 5})
			ea.UpdateExperimentCounts(instances)
		}()
	}

	wg.Wait()
}

// --- Cluster Resource Poller Tests ---

func TestClusterResourcePoller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := clusterInfoResponse{
			Success: true,
			Data: clusterInfoData{
				TotalCPU:          50000,
				UsedCPU:           20000,
				HealthyPartitions: 3,
				TotalPartitions:   3,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ea := NewExperimentAdmission(server.URL, 1000, 0.5, nil)
	ea.pollClusterResource()

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, int64(50000), ea.clusterTotal)
	assert.Equal(t, int64(20000), ea.clusterUsed)
	assert.True(t, ea.hasClusterData)
}

func TestClusterResourcePollerFailureGraceful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	ea := NewExperimentAdmission(server.URL, 1000, 0.5, nil)
	ea.mu.Lock()
	ea.clusterTotal = 30000
	ea.clusterUsed = 10000
	ea.hasClusterData = true
	ea.mu.Unlock()

	ea.pollClusterResource()

	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, int64(30000), ea.clusterTotal)
	assert.Equal(t, int64(10000), ea.clusterUsed)
}

// --- GetMetrics Test ---

func TestGetMetrics(t *testing.T) {
	ea := newTestAdmission(100000, 40000, 500, 0.5)
	ea.experiments["owner/exp-a"] = &ExperimentState{CurrentCount: 10}

	m := ea.GetMetrics()
	assert.Equal(t, int64(100000), m["cluster_total"])
	assert.Equal(t, int64(40000), m["cluster_used"])
	assert.Equal(t, 500, m["max_instances"])
	assert.Equal(t, 0.5, m["watermark"])
	assert.Equal(t, 1, m["experiment_count"])
}

// --- Default Value Tests ---

func TestDefaultWatermarkAndMaxInstances(t *testing.T) {
	ea := NewExperimentAdmission("http://localhost:0", 0, 0, nil)
	assert.Equal(t, 0.5, ea.watermark)
	assert.Equal(t, 1000, ea.maxInstances)
}

// --- Duplicate Launch Scenario Test ---

func TestDuplicateLaunchScenario(t *testing.T) {
	ea := newTestAdmission(100000, 10000, 200, 0.5)

	instances := makeInstances(map[string]int{"zhangsan/swe-bench": 200})
	ea.UpdateExperimentCounts(instances)

	result := ea.ShouldAdmitWithResult("zhangsan/swe-bench")
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "reached limit 200")

	result = ea.ShouldAdmitWithResult("lisi/swe-bench")
	assert.True(t, result.Allowed)
}

// --- Redis Integration Tests ---

func TestRedisRegisterInstance(t *testing.T) {
	ea, mr := newTestAdmissionWithRedis(t, 100000, 10000, 1000, 0.5)
	defer mr.Close()

	ea.RegisterInstance("inst-1", "owner/exp-a")
	ea.RegisterInstance("inst-2", "owner/exp-a")
	ea.RegisterInstance("inst-3", "owner/exp-b")

	// Check Redis state
	val := mr.HGet(experimentCountsKey, "owner/exp-a")
	assert.Equal(t, "2", val)

	val = mr.HGet(experimentCountsKey, "owner/exp-b")
	assert.Equal(t, "1", val)

	// Local cache should match
	ea.mu.RLock()
	assert.Equal(t, 2, ea.experiments["owner/exp-a"].CurrentCount)
	assert.Equal(t, 1, ea.experiments["owner/exp-b"].CurrentCount)
	ea.mu.RUnlock()
}

func TestRedisUnregisterInstance(t *testing.T) {
	ea, mr := newTestAdmissionWithRedis(t, 100000, 10000, 1000, 0.5)
	defer mr.Close()

	ea.RegisterInstance("inst-1", "owner/exp-a")
	ea.RegisterInstance("inst-2", "owner/exp-a")

	ea.UnregisterInstance("inst-1")

	// Redis should show count=1
	val := mr.HGet(experimentCountsKey, "owner/exp-a")
	assert.Equal(t, "1", val)

	// Unregister last instance — key should be deleted from Redis
	ea.UnregisterInstance("inst-2")

	val = mr.HGet(experimentCountsKey, "owner/exp-a")
	assert.Equal(t, "", val, "Redis key should be deleted when count reaches 0")

	// Local cache should be empty
	ea.mu.RLock()
	_, exists := ea.experiments["owner/exp-a"]
	ea.mu.RUnlock()
	assert.False(t, exists)
}

func TestRedisPeriodicSync(t *testing.T) {
	ea, mr := newTestAdmissionWithRedis(t, 100000, 10000, 1000, 0.5)
	defer mr.Close()

	// Manually set stale data in Redis
	mr.HSet(experimentCountsKey, "owner/stale-exp", "999")

	// Periodic sync should overwrite with actual counts
	instances := makeInstances(map[string]int{
		"owner/exp-a": 5,
		"owner/exp-b": 3,
	})
	ea.UpdateExperimentCounts(instances)

	// Stale experiment should be gone
	val := mr.HGet(experimentCountsKey, "owner/stale-exp")
	assert.Equal(t, "", val, "stale experiment should be removed by sync")

	// Actual experiments should be correct
	val = mr.HGet(experimentCountsKey, "owner/exp-a")
	assert.Equal(t, "5", val)
	val = mr.HGet(experimentCountsKey, "owner/exp-b")
	assert.Equal(t, "3", val)
}

func TestRedisWarmup(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Pre-populate Redis
	mr.HSet(experimentCountsKey, "owner/exp-a", "10")
	mr.HSet(experimentCountsKey, "owner/exp-b", "5")

	rc := &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ctx:    context.Background(),
	}
	ea := NewExperimentAdmission("http://localhost:0", 1000, 0.5, rc)
	ea.mu.Lock()
	ea.clusterTotal = 100000
	ea.clusterUsed = 10000
	ea.hasClusterData = true
	ea.mu.Unlock()

	// Warmup should have loaded experiments
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	assert.Equal(t, 10, ea.experiments["owner/exp-a"].CurrentCount)
	assert.Equal(t, 5, ea.experiments["owner/exp-b"].CurrentCount)
}

func TestRedisDownDegradesToLocal(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ctx:    context.Background(),
	}
	ea := NewExperimentAdmission("http://localhost:0", 1000, 0.5, rc)
	ea.mu.Lock()
	ea.clusterTotal = 100000
	ea.clusterUsed = 10000
	ea.hasClusterData = true
	ea.mu.Unlock()

	// Stop Redis
	mr.Close()

	// Operations should still work (local-only)
	ea.RegisterInstance("inst-1", "owner/exp-a")

	ea.mu.RLock()
	assert.Equal(t, 1, ea.experiments["owner/exp-a"].CurrentCount)
	ea.mu.RUnlock()

	// Admission check should work from local cache
	result := ea.ShouldAdmitWithResult("owner/exp-a")
	assert.True(t, result.Allowed)
}

func TestRedisMultiInstanceConsistency(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Simulate two api-service instances sharing the same Redis
	rc1 := &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ctx:    context.Background(),
	}
	rc2 := &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ctx:    context.Background(),
	}

	ea1 := NewExperimentAdmission("http://localhost:0", 100, 0.5, rc1)
	ea1.mu.Lock()
	ea1.clusterTotal = 100000
	ea1.clusterUsed = 10000
	ea1.hasClusterData = true
	ea1.mu.Unlock()

	ea2 := NewExperimentAdmission("http://localhost:0", 100, 0.5, rc2)
	ea2.mu.Lock()
	ea2.clusterTotal = 100000
	ea2.clusterUsed = 10000
	ea2.hasClusterData = true
	ea2.mu.Unlock()

	// Instance 1 registers 60 instances
	for i := 0; i < 60; i++ {
		ea1.RegisterInstance(fmt.Sprintf("a-inst-%d", i), "owner/exp-a")
	}
	// Instance 2 registers 40 more
	for i := 0; i < 40; i++ {
		ea2.RegisterInstance(fmt.Sprintf("b-inst-%d", i), "owner/exp-a")
	}

	// Redis should show total = 100
	val := mr.HGet(experimentCountsKey, "owner/exp-a")
	count, _ := strconv.Atoi(val)
	assert.Equal(t, 100, count)

	// ea2's local cache should also show 100 (refreshed by HINCRBY return value)
	ea2.mu.RLock()
	assert.Equal(t, 100, ea2.experiments["owner/exp-a"].CurrentCount)
	ea2.mu.RUnlock()
}
