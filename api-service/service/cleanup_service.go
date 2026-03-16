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
	"time"

	log "github.com/sirupsen/logrus"
)

type AEnvCleanManager struct {
	envInstanceService EnvInstanceService

	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc

	// Metrics counters
	incrementCleanupSuccess func()
	incrementCleanupFailure func()
}

func NewAEnvCleanManager(envInstanceService EnvInstanceService, duration time.Duration) *AEnvCleanManager {
	ctx, cancel := context.WithCancel(context.Background())
	AEnvCleanManager := &AEnvCleanManager{
		envInstanceService: envInstanceService,

		interval: duration,
		ctx:      ctx,
		cancel:   cancel,

		// Default metrics functions
		incrementCleanupSuccess: func() {},
		incrementCleanupFailure: func() {},
	}
	return AEnvCleanManager
}

// WithMetrics sets the metrics functions for the clean manager
func (cm *AEnvCleanManager) WithMetrics(incrementSuccess, incrementFailure func()) *AEnvCleanManager {
	cm.incrementCleanupSuccess = incrementSuccess
	cm.incrementCleanupFailure = incrementFailure
	return cm
}

// Start starts the cleanup service
func (cm *AEnvCleanManager) Start() {
	log.Infof("Starting cleanup service with interval: %v", cm.interval)
	// Execute cleanup immediately
	cm.performCleanup()

	// Start timer
	ticker := time.NewTicker(cm.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cm.performCleanup()
			case <-cm.ctx.Done():
				log.Info("Cleanup service stopped")
				return
			}
		}
	}()
}

// performCleanup performs the actual cleanup task by checking TTL expiration
func (cm *AEnvCleanManager) performCleanup() {
	log.Debug("Starting TTL-based cleanup task...")

	// Get all environment instances
	envInstances, err := cm.envInstanceService.ListEnvInstances("")
	if err != nil {
		log.Errorf("Failed to list environment instances: %v", err)
		return
	}

	cm.CleanupFromInstances(envInstances)
}

// CleanupFromInstances performs TTL-based cleanup on a pre-fetched instance list.
// This allows callers to share the same ListEnvInstances result across multiple consumers.
func (cm *AEnvCleanManager) CleanupFromInstances(envInstances []*models.EnvInstance) {
	if len(envInstances) == 0 {
		log.Debug("No environment instances found")
		return
	}

	var deletedCount int

	// Check each instance for TTL expiration
	for _, instance := range envInstances {
		// Skip already terminated instances
		if instance.Status == "Terminated" {
			continue
		}

		// Check if TTL is set and has expired
		if cm.isExpired(instance) {
			log.Infof("Instance %s has expired (TTL: %s), deleting...", instance.ID, instance.TTL)
			err := cm.envInstanceService.DeleteEnvInstance(instance.ID)
			if err != nil {
				log.Errorf("Failed to delete expired instance %s: %v", instance.ID, err)
				cm.incrementCleanupFailure()
				continue
			}
			deletedCount++
			cm.incrementCleanupSuccess()
			log.Infof("Successfully deleted expired instance %s", instance.ID)
		}
	}

	log.Infof("TTL-based cleanup task completed. Deleted %d expired instances", deletedCount)
}

// isExpired checks if an environment instance has expired based on its TTL and creation time
func (cm *AEnvCleanManager) isExpired(instance *models.EnvInstance) bool {
	// If TTL is not set, consider it as non-expiring
	if instance.TTL == "" {
		return false
	}

	// Parse TTL duration
	ttlDuration, err := time.ParseDuration(instance.TTL)
	if err != nil {
		log.Warnf("Failed to parse TTL '%s' for instance %s: %v", instance.TTL, instance.ID, err)
		return false
	}

	// Parse creation time using time.DateTime format (2006-01-02 15:04:05)
	createdAt, err := time.Parse(time.DateTime, instance.CreatedAt)
	if err != nil {
		// Fallback to RFC3339 if DateTime parsing fails
		createdAt, err = time.Parse(time.RFC3339, instance.CreatedAt)
		if err != nil {
			log.Warnf("Failed to parse creation time '%s' for instance %s: %v", instance.CreatedAt, instance.ID, err)
			return false
		}
	}

	// Check if the instance has expired
	expirationTime := createdAt.Add(ttlDuration)
	return time.Now().After(expirationTime)
}

// Stop stops the cleanup service
func (cm *AEnvCleanManager) Stop() {
	cm.cancel()
}
