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
