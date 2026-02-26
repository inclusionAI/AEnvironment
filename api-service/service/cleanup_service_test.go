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
	backend "envhub/models"
	"errors"
	"testing"
	"time"
)

func TestNewCleanupService(t *testing.T) {
	// This test is kept for compatibility but doesn't actually test anything meaningful
	// since we can't easily instantiate a real ScheduleClient in tests
	t.Skip("Skipping integration test that requires real ScheduleClient")
}

// MockEnvInstanceService is a mock implementation of EnvInstanceService for testing
type MockEnvInstanceService struct {
	ListEnvInstancesFunc  func(envName string) ([]*models.EnvInstance, error)
	DeleteEnvInstanceFunc func(id string) error
}

func (m *MockEnvInstanceService) GetEnvInstance(id string) (*models.EnvInstance, error) {
	return nil, nil
}

func (m *MockEnvInstanceService) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	return nil, nil
}

func (m *MockEnvInstanceService) DeleteEnvInstance(id string) error {
	if m.DeleteEnvInstanceFunc != nil {
		return m.DeleteEnvInstanceFunc(id)
	}
	return nil
}

func (m *MockEnvInstanceService) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	if m.ListEnvInstancesFunc != nil {
		return m.ListEnvInstancesFunc(envName)
	}
	return nil, nil
}

func (m *MockEnvInstanceService) Warmup(req *backend.Env) error {
	return nil
}

func (m *MockEnvInstanceService) Cleanup() error {
	return nil
}

// TestPerformCleanupNoInstances tests cleanup when there are no env instances
func TestPerformCleanupNoInstances(t *testing.T) {
	// Create mock service that returns empty list
	mockService := &MockEnvInstanceService{
		ListEnvInstancesFunc: func(envName string) ([]*models.EnvInstance, error) {
			return []*models.EnvInstance{}, nil
		},
	}

	// Create clean manager with mock service
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Perform cleanup
	manager.performCleanup()

	// Since there are no instances, no delete operations should be called
	// The test passes if no panic occurs
}

// TestPerformCleanupWithExpiredInstances tests cleanup with expired env instances
func TestPerformCleanupWithExpiredInstances(t *testing.T) {
	// Create mock service with expired instances
	expiredInstance := &models.EnvInstance{
		ID:        "test-instance-1",
		Status:    "Running",
		CreatedAt: "2025-01-01 10:00:00",
		TTL:       "1h",
	}

	terminatedInstance := &models.EnvInstance{
		ID:        "test-instance-2",
		Status:    "Terminated",
		CreatedAt: "2025-01-01 10:00:00",
		TTL:       "1h",
	}

	activeInstance := &models.EnvInstance{
		ID:        "test-instance-3",
		Status:    "Running",
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		TTL:       "1h",
	}

	var deletedInstances []string
	mockService := &MockEnvInstanceService{
		ListEnvInstancesFunc: func(envName string) ([]*models.EnvInstance, error) {
			return []*models.EnvInstance{expiredInstance, terminatedInstance, activeInstance}, nil
		},
		DeleteEnvInstanceFunc: func(id string) error {
			deletedInstances = append(deletedInstances, id)
			return nil
		},
	}

	// Create clean manager with mock service
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Perform cleanup
	manager.performCleanup()

	// Verify that only the expired instance was deleted
	if len(deletedInstances) != 1 {
		t.Errorf("Expected 1 deleted instance, got %d", len(deletedInstances))
	}

	if len(deletedInstances) > 0 && deletedInstances[0] != "test-instance-1" {
		t.Errorf("Expected deleted instance ID 'test-instance-1', got '%s'", deletedInstances[0])
	}
}

// TestPerformCleanupWithDeleteError tests cleanup when delete operation fails
func TestPerformCleanupWithDeleteError(t *testing.T) {
	// Create mock service with expired instance that fails to delete
	expiredInstance := &models.EnvInstance{
		ID:        "test-instance-1",
		Status:    "Running",
		CreatedAt: "2025-01-01 10:00:00",
		TTL:       "1h",
	}

	mockService := &MockEnvInstanceService{
		ListEnvInstancesFunc: func(envName string) ([]*models.EnvInstance, error) {
			return []*models.EnvInstance{expiredInstance}, nil
		},
		DeleteEnvInstanceFunc: func(id string) error {
			return errors.New("delete failed")
		},
	}

	// Create clean manager with mock service
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Perform cleanup
	manager.performCleanup()

	// The test passes if no panic occurs even when delete fails
}

// TestPerformCleanupWithListError tests cleanup when listing instances fails
func TestPerformCleanupWithListError(t *testing.T) {
	// Create mock service that fails to list instances
	mockService := &MockEnvInstanceService{
		ListEnvInstancesFunc: func(envName string) ([]*models.EnvInstance, error) {
			return nil, errors.New("list failed")
		},
	}

	// Create clean manager with mock service
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Perform cleanup
	manager.performCleanup()

	// The test passes if no panic occurs even when list fails
}

// TestIsExpiredWithDateTimeFormat tests isExpired with time.DateTime format
func TestIsExpiredWithDateTimeFormat(t *testing.T) {
	// Create instance with DateTime format creation time
	expiredInstance := &models.EnvInstance{
		ID:        "test-expired-1",
		Status:    "Running",
		CreatedAt: "2025-01-01 10:00:00", // time.DateTime format
		TTL:       "1h",
	}

	activeInstance := &models.EnvInstance{
		ID:        "test-active-1",
		Status:    "Running",
		CreatedAt: time.Now().Format(time.DateTime),
		TTL:       "1h",
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Test expired instance
	if !manager.isExpired(expiredInstance) {
		t.Errorf("Expected instance %s to be expired", expiredInstance.ID)
	}

	// Test active instance
	if manager.isExpired(activeInstance) {
		t.Errorf("Expected instance %s to be active", activeInstance.ID)
	}
}

// TestIsExpiredWithRFC3339Format tests isExpired with RFC3339 format (fallback)
func TestIsExpiredWithRFC3339Format(t *testing.T) {
	// Create instance with RFC3339 format creation time
	expiredInstance := &models.EnvInstance{
		ID:        "test-expired-rfc3339-1",
		Status:    "Running",
		CreatedAt: "2025-01-01T10:00:00+08:00", // RFC3339 format
		TTL:       "1h",
	}

	activeInstance := &models.EnvInstance{
		ID:        "test-active-rfc3339-1",
		Status:    "Running",
		CreatedAt: time.Now().Format(time.RFC3339),
		TTL:       "1h",
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Test expired instance
	if !manager.isExpired(expiredInstance) {
		t.Errorf("Expected instance %s to be expired", expiredInstance.ID)
	}

	// Test active instance
	if manager.isExpired(activeInstance) {
		t.Errorf("Expected instance %s to be active", activeInstance.ID)
	}
}

// TestIsExpiredWithInvalidTimeFormat tests isExpired with invalid time format
func TestIsExpiredWithInvalidTimeFormat(t *testing.T) {
	// Create instance with invalid time format
	invalidInstance := &models.EnvInstance{
		ID:        "test-invalid-time",
		Status:    "Running",
		CreatedAt: "invalid-time-format",
		TTL:       "1h",
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Should return false for invalid time format
	if manager.isExpired(invalidInstance) {
		t.Errorf("Expected instance %s with invalid time format to not be expired", invalidInstance.ID)
	}
}

// TestIsExpiredWithInvalidTTLFormat tests isExpired with invalid TTL format
func TestIsExpiredWithInvalidTTLFormat(t *testing.T) {
	// Create instance with invalid TTL format
	invalidInstance := &models.EnvInstance{
		ID:        "test-invalid-ttl",
		Status:    "Running",
		CreatedAt: time.Now().Format(time.DateTime),
		TTL:       "invalid-ttl",
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Should return false for invalid TTL format
	if manager.isExpired(invalidInstance) {
		t.Errorf("Expected instance %s with invalid TTL format to not be expired", invalidInstance.ID)
	}
}

// TestIsExpiredWithEmptyTTL tests isExpired with empty TTL
func TestIsExpiredWithEmptyTTL(t *testing.T) {
	// Create instance with empty TTL
	emptyTTLInstance := &models.EnvInstance{
		ID:        "test-empty-ttl",
		Status:    "Running",
		CreatedAt: "2025-01-01 10:00:00",
		TTL:       "",
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	// Should return false for empty TTL
	if manager.isExpired(emptyTTLInstance) {
		t.Errorf("Expected instance %s with empty TTL to not be expired", emptyTTLInstance.ID)
	}
}

// TestIsExpiredWithVariousTTLDurations tests isExpired with various TTL durations
func TestIsExpiredWithVariousTTLDurations(t *testing.T) {
	testCases := []struct {
		name      string
		createdAt string
		ttl       string
		expected  bool
	}{
		{
			name:      "expired with seconds",
			createdAt: "2025-01-01 10:00:00",
			ttl:       "30s",
			expected:  true,
		},
		{
			name:      "expired with minutes",
			createdAt: "2025-01-01 10:00:00",
			ttl:       "5m",
			expected:  true,
		},
		{
			name:      "expired with hours",
			createdAt: "2025-01-01 10:00:00",
			ttl:       "2h",
			expected:  true,
		},
		{
			name:      "active with long TTL",
			createdAt: time.Now().Format(time.DateTime),
			ttl:       "24h",
			expected:  false,
		},
	}

	mockService := &MockEnvInstanceService{}
	manager := NewAEnvCleanManager(mockService, time.Minute)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instance := &models.EnvInstance{
				ID:        "test-" + tc.name,
				Status:    "Running",
				CreatedAt: tc.createdAt,
				TTL:       tc.ttl,
			}

			result := manager.isExpired(instance)
			if result != tc.expected {
				t.Errorf("Expected isExpired to be %v, got %v for instance %s", tc.expected, result, instance.ID)
			}
		})
	}
}
