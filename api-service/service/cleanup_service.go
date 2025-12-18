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
	"context"
	"log"
	"time"
)

type AEnvCleaner interface {
	cleanup()
}

type AEnvCleanManager struct {
	cleaner AEnvCleaner

	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewAEnvCleanManager(cleaner AEnvCleaner, duration time.Duration) *AEnvCleanManager {
	ctx, cancel := context.WithCancel(context.Background())
	AEnvCleanManager := &AEnvCleanManager{
		cleaner: cleaner,

		interval: duration,
		ctx:      ctx,
		cancel:   cancel,
	}
	return AEnvCleanManager
}

// Start starts the cleanup service
func (cm *AEnvCleanManager) Start() {
	log.Printf("Starting cleanup service with interval: %v", cm.interval)
	// Execute cleanup immediately
	cm.cleaner.cleanup()

	// Start timer
	ticker := time.NewTicker(cm.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cm.cleaner.cleanup()
			case <-cm.ctx.Done():
				log.Println("Cleanup service stopped")
				return
			}
		}
	}()
}

// Stop stops the cleanup service
func (cm *AEnvCleanManager) Stop() {
	cm.cancel()
}

// KubeCleaner cleanup service responsible for periodically cleaning expired EnvInstances
type KubeCleaner struct {
	scheduleClient EnvInstanceService
}

// NewCleanupService
func NewKubeCleaner(scheduleClient EnvInstanceService) *KubeCleaner {
	return &KubeCleaner{
		scheduleClient: scheduleClient,
	}
}

// cleanup executes cleanup task
func (cs *KubeCleaner) cleanup() {
	_ = cs.scheduleClient.Cleanup()
}
