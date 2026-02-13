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

type AEnvCleanManager struct {
	envInstanceService EnvInstanceService

	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewAEnvCleanManager(envInstanceService EnvInstanceService, duration time.Duration) *AEnvCleanManager {
	ctx, cancel := context.WithCancel(context.Background())
	AEnvCleanManager := &AEnvCleanManager{
		envInstanceService: envInstanceService,

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
	_ = cm.envInstanceService.Cleanup()

	// Start timer
	ticker := time.NewTicker(cm.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = cm.envInstanceService.Cleanup()
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
