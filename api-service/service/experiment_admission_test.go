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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestAdmission(totalCPU, perInstanceCPU int64, peakWindow time.Duration) *ExperimentAdmission {
	ea := NewExperimentAdmission("http://localhost:14457", perInstanceCPU, peakWindow, 0.7, []string{"experiment"})
	ea.mu.Lock()
	ea.clusterTotal = totalCPU
	ea.hasClusterData = true
	ea.mu.Unlock()
	return ea
}

func makeInstances(experiments map[string]int) []*models.EnvInstance {
	var instances []*models.EnvInstance
	id := 0
	for exp, count := range experiments {
		for i := 0; i < count; i++ {
			id++
			labels := map[string]string{"experiment": exp}
			instances = append(instances, &models.EnvInstance{
				ID:     fmt.Sprintf("inst-%d", id),
				Status: "Running",
				Labels: labels,
			})
		}
	}
	return instances
}

// Need fmt for makeInstances
func init() {}

func TestKnownExperimentAlwaysAdmitted(t *testing.T) {
	ea := newTestAdmission(10000, 2000, 15*time.Minute)

	// Register experiment "exp-1" with instances
	instances := makeInstances(map[string]int{"exp-1": 5})
	ea.UpdateExperimentCounts(instances)

	// Known experiment should always be admitted, even if cluster is "full"
	ea.mu.Lock()
	ea.clusterTotal = 1 // artificially tiny cluster
	ea.mu.Unlock()

	allowed, reason := ea.ShouldAdmit("exp-1")
	if !allowed {
		t.Errorf("Expected known experiment to be admitted, got rejected: %s", reason)
	}
}

func TestNewExperimentAdmittedWhenCapacityAvailable(t *testing.T) {
	ea := newTestAdmission(20000, 2000, 15*time.Minute)

	// Register experiment "exp-1" with 3 instances (peak=3, reserved=6000)
	instances := makeInstances(map[string]int{"exp-1": 3})
	ea.UpdateExperimentCounts(instances)

	// New experiment "exp-2": available = 20000 - 6000 = 14000 > 2000
	allowed, reason := ea.ShouldAdmit("exp-2")
	if !allowed {
		t.Errorf("Expected new experiment to be admitted, got rejected: %s", reason)
	}
}

func TestNewExperimentRejectedWhenCapacityInsufficient(t *testing.T) {
	ea := newTestAdmission(10000, 2000, 15*time.Minute)

	// Register experiment "exp-1" with 5 instances (peak=5, reserved=10000)
	instances := makeInstances(map[string]int{"exp-1": 5})
	ea.UpdateExperimentCounts(instances)

	// New experiment "exp-2": available = 10000 - 10000 = 0, not > 2000
	allowed, _ := ea.ShouldAdmit("exp-2")
	if allowed {
		t.Error("Expected new experiment to be rejected when capacity is insufficient")
	}
}

func TestMultipleExperimentsReserveCapacity(t *testing.T) {
	ea := newTestAdmission(20000, 2000, 15*time.Minute)

	// Two experiments: exp-1=3, exp-2=4, reserved = (3+4)*2000 = 14000
	instances := makeInstances(map[string]int{"exp-1": 3, "exp-2": 4})
	ea.UpdateExperimentCounts(instances)

	// New experiment "exp-3": available = 20000 - 14000 = 6000 > 2000 → admit
	allowed, _ := ea.ShouldAdmit("exp-3")
	if !allowed {
		t.Error("Expected exp-3 to be admitted with 6000 available")
	}

	// Now add more to exp-2 to fill cluster: exp-1=3, exp-2=7, reserved = (3+7)*2000 = 20000
	instances = makeInstances(map[string]int{"exp-1": 3, "exp-2": 7})
	ea.UpdateExperimentCounts(instances)

	// New experiment "exp-3": available = 20000 - 20000 = 0, not > 2000 → reject
	allowed, _ = ea.ShouldAdmit("exp-3")
	if allowed {
		t.Error("Expected exp-3 to be rejected when cluster is fully reserved")
	}
}

func TestPeakSlidingWindowEviction(t *testing.T) {
	peakWindow := 100 * time.Millisecond
	ea := newTestAdmission(10000, 2000, peakWindow)

	// Record a high count
	instances := makeInstances(map[string]int{"exp-1": 5})
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	peak := ea.experiments["exp-1"].PeakCount
	ea.mu.RUnlock()
	if peak != 5 {
		t.Errorf("Expected peak=5, got %d", peak)
	}

	// Wait for the window to expire
	time.Sleep(peakWindow + 50*time.Millisecond)

	// Update with lower count — old peak sample should be evicted
	instances = makeInstances(map[string]int{"exp-1": 2})
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	peak = ea.experiments["exp-1"].PeakCount
	ea.mu.RUnlock()
	if peak != 2 {
		t.Errorf("Expected peak=2 after window eviction, got %d", peak)
	}
}

func TestExperimentRemovedAfterWindowExpires(t *testing.T) {
	peakWindow := 100 * time.Millisecond
	ea := newTestAdmission(10000, 2000, peakWindow)

	instances := makeInstances(map[string]int{"exp-1": 3})
	ea.UpdateExperimentCounts(instances)

	// Wait for window to expire
	time.Sleep(peakWindow + 50*time.Millisecond)

	// Update with no instances for exp-1
	ea.UpdateExperimentCounts([]*models.EnvInstance{})

	ea.mu.RLock()
	_, exists := ea.experiments["exp-1"]
	ea.mu.RUnlock()
	if exists {
		t.Error("Expected exp-1 to be removed after all samples expired")
	}
}

func TestFailOpenWhenNoClusterData(t *testing.T) {
	ea := NewExperimentAdmission("http://localhost:14457", 2000, 15*time.Minute, 0.7, []string{"experiment"})
	// hasClusterData is false by default

	allowed, _ := ea.ShouldAdmit("new-experiment")
	if !allowed {
		t.Error("Expected fail-open when no cluster data available")
	}
}

func TestEmptyExperimentIDTreatedAsDefault(t *testing.T) {
	ea := newTestAdmission(10000, 2000, 15*time.Minute)

	allowed, _ := ea.ShouldAdmit("")
	if !allowed {
		t.Error("Expected empty experiment ID to be admitted (treated as default)")
	}
}

func TestTerminatedInstancesNotCounted(t *testing.T) {
	ea := newTestAdmission(10000, 2000, 15*time.Minute)

	instances := []*models.EnvInstance{
		{ID: "inst-1", Status: "Running", Labels: map[string]string{"experiment": "exp-1"}},
		{ID: "inst-2", Status: "Terminated", Labels: map[string]string{"experiment": "exp-1"}},
		{ID: "inst-3", Status: "Failed", Labels: map[string]string{"experiment": "exp-1"}},
		{ID: "inst-4", Status: "Running", Labels: map[string]string{"experiment": "exp-1"}},
	}
	ea.UpdateExperimentCounts(instances)

	ea.mu.RLock()
	count := ea.experiments["exp-1"].CurrentCount
	peak := ea.experiments["exp-1"].PeakCount
	ea.mu.RUnlock()

	if count != 2 {
		t.Errorf("Expected current count=2 (excluding terminated/failed), got %d", count)
	}
	if peak != 2 {
		t.Errorf("Expected peak=2, got %d", peak)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ea := newTestAdmission(100000, 2000, 15*time.Minute)

	instances := makeInstances(map[string]int{"exp-1": 10, "exp-2": 5})
	ea.UpdateExperimentCounts(instances)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ea.ShouldAdmit("exp-1")
			ea.ShouldAdmit("new-exp")
			ea.GetMetrics()
		}(i)
	}

	// Concurrent updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ea.UpdateExperimentCounts(instances)
		}()
	}

	wg.Wait()
}

func TestClusterResourcePoller(t *testing.T) {
	// Create a mock faas-api-service HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hapis/faas.hcs.io/v1/clusterinfo" {
			http.NotFound(w, r)
			return
		}
		resp := clusterInfoResponse{
			Success: true,
			Data: clusterInfoData{
				TotalCPU:          50000,
				UsedCPU:           20000,
				FreeCPU:           30000,
				TotalMemory:       128000,
				UsedMemory:        64000,
				FreeMemory:        64000,
				HealthyPartitions: 2,
				TotalPartitions:   2,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ea := NewExperimentAdmission(server.URL, 2000, 15*time.Minute, 0.7, []string{"experiment"})
	ea.pollClusterResource()

	ea.mu.RLock()
	defer ea.mu.RUnlock()

	if !ea.hasClusterData {
		t.Error("Expected hasClusterData=true after successful poll")
	}
	if ea.clusterTotal != 50000 {
		t.Errorf("Expected clusterTotal=50000, got %d", ea.clusterTotal)
	}
	if ea.clusterUsed != 20000 {
		t.Errorf("Expected clusterUsed=20000, got %d", ea.clusterUsed)
	}
}

func TestClusterResourcePollerFailureGraceful(t *testing.T) {
	// Create a server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ea := NewExperimentAdmission(server.URL, 2000, 15*time.Minute, 0.7, []string{"experiment"})

	// Set initial data
	ea.mu.Lock()
	ea.clusterTotal = 30000
	ea.hasClusterData = true
	ea.mu.Unlock()

	// Poll should fail but keep existing data
	ea.pollClusterResource()

	ea.mu.RLock()
	defer ea.mu.RUnlock()

	if ea.clusterTotal != 30000 {
		t.Errorf("Expected clusterTotal to remain 30000 after poll failure, got %d", ea.clusterTotal)
	}
}

func TestGetMetrics(t *testing.T) {
	ea := newTestAdmission(20000, 2000, 15*time.Minute)

	instances := makeInstances(map[string]int{"exp-1": 3, "exp-2": 5})
	ea.UpdateExperimentCounts(instances)

	m := ea.GetMetrics()

	if m["cluster_total"].(int64) != 20000 {
		t.Errorf("Expected cluster_total=20000, got %v", m["cluster_total"])
	}
	if m["experiment_count"].(int) != 2 {
		t.Errorf("Expected experiment_count=2, got %v", m["experiment_count"])
	}
	if m["reserved_capacity"].(int64) != (3+5)*2000 {
		t.Errorf("Expected reserved_capacity=16000, got %v", m["reserved_capacity"])
	}
}

func TestWatermarkRejectsNewExperiment(t *testing.T) {
	ea := newTestAdmission(100000, 2000, 15*time.Minute)
	// Set watermark to 0.5
	ea.mu.Lock()
	ea.watermark = 0.5
	ea.clusterUsed = 60000 // 60% utilization, above 50% watermark
	ea.mu.Unlock()

	// Register exp-1 so it's known
	instances := makeInstances(map[string]int{"exp-1": 3})
	ea.UpdateExperimentCounts(instances)

	// P0: known experiment should still be admitted
	result := ea.ShouldAdmitWithResult("exp-1")
	if !result.Allowed || result.Tier != "p0_known" {
		t.Errorf("Expected P0 known experiment to be admitted, got allowed=%v tier=%s", result.Allowed, result.Tier)
	}

	// P1: new experiment should be rejected due to watermark
	result = ea.ShouldAdmitWithResult("exp-new")
	if result.Allowed {
		t.Error("Expected new experiment to be rejected when utilization exceeds watermark")
	}
	if result.Tier != "p1_new" {
		t.Errorf("Expected tier=p1_new, got %s", result.Tier)
	}
}

func TestWatermarkPassesWhenUtilizationLow(t *testing.T) {
	ea := newTestAdmission(100000, 2000, 15*time.Minute)
	ea.mu.Lock()
	ea.watermark = 0.7
	ea.clusterUsed = 50000 // 50% utilization, below 70% watermark
	ea.mu.Unlock()

	result := ea.ShouldAdmitWithResult("new-exp")
	if !result.Allowed {
		t.Errorf("Expected new experiment to be admitted when utilization below watermark, reason: %s", result.Reason)
	}
	if result.Tier != "p1_new" {
		t.Errorf("Expected tier=p1_new, got %s", result.Tier)
	}
}

func TestCheckRequiredLabels(t *testing.T) {
	ea := NewExperimentAdmission("http://localhost", 2000, 15*time.Minute, 0.7, []string{"experiment", "team"})

	// All present
	missing := ea.CheckRequiredLabels(map[string]string{"experiment": "exp-1", "team": "ml"})
	if len(missing) != 0 {
		t.Errorf("Expected no missing labels, got %v", missing)
	}

	// One missing
	missing = ea.CheckRequiredLabels(map[string]string{"experiment": "exp-1"})
	if len(missing) != 1 || missing[0] != "team" {
		t.Errorf("Expected missing=[team], got %v", missing)
	}

	// All missing
	missing = ea.CheckRequiredLabels(map[string]string{})
	if len(missing) != 2 {
		t.Errorf("Expected 2 missing labels, got %d", len(missing))
	}

	// Nil labels
	missing = ea.CheckRequiredLabels(nil)
	if len(missing) != 2 {
		t.Errorf("Expected 2 missing labels for nil map, got %d", len(missing))
	}
}

func TestTierClassification(t *testing.T) {
	ea := newTestAdmission(100000, 2000, 15*time.Minute)
	ea.mu.Lock()
	ea.watermark = 0.7
	ea.clusterUsed = 10000
	ea.mu.Unlock()

	instances := makeInstances(map[string]int{"exp-1": 5})
	ea.UpdateExperimentCounts(instances)

	// P0: known
	result := ea.ShouldAdmitWithResult("exp-1")
	if result.Tier != "p0_known" {
		t.Errorf("Expected tier=p0_known for known experiment, got %s", result.Tier)
	}

	// P1: new
	result = ea.ShouldAdmitWithResult("exp-new")
	if result.Tier != "p1_new" {
		t.Errorf("Expected tier=p1_new for new experiment, got %s", result.Tier)
	}
}

func TestDefaultWatermarkAndLabels(t *testing.T) {
	// Invalid watermark should default to 0.7
	ea := NewExperimentAdmission("http://localhost", 2000, 15*time.Minute, 0, nil)
	ea.mu.RLock()
	if ea.watermark != 0.7 {
		t.Errorf("Expected default watermark=0.7, got %f", ea.watermark)
	}
	if len(ea.requiredLabels) != 1 || ea.requiredLabels[0] != "experiment" {
		t.Errorf("Expected default requiredLabels=[experiment], got %v", ea.requiredLabels)
	}
	ea.mu.RUnlock()
}
