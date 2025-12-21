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

package docker_http_server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"docker-controller/pkg/constants"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"k8s.io/klog"
)

// AEnvContainerCache caches Container information
type AEnvContainerCache struct {
	dockerClient *client.Client
	cache        map[string]*types.ContainerJSON
	mu           sync.RWMutex
	stopCh       chan struct{}
}

// NewAEnvContainerCache creates new Container cache
func NewAEnvContainerCache(dockerClient *client.Client) *AEnvContainerCache {
	cache := &AEnvContainerCache{
		dockerClient: dockerClient,
		cache:        make(map[string]*types.ContainerJSON),
		stopCh:       make(chan struct{}),
	}

	// Start cache refresh goroutine
	go cache.refreshCache()

	klog.Infof("Container cache initialization started")
	return cache
}

// refreshCache periodically refreshes the container cache
func (c *AEnvContainerCache) refreshCache() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.updateCache()
		case <-c.stopCh:
			return
		}
	}
}

// updateCache updates the cache with current container list
func (c *AEnvContainerCache) updateCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := filters.NewArgs()
	filter.Add("label", "aenv.managed=true")

	containers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		klog.Warningf("failed to list containers for cache update: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing cache
	c.cache = make(map[string]*types.ContainerJSON)

	// Populate cache
	for _, cnt := range containers {
		containerInfo, err := c.dockerClient.ContainerInspect(ctx, cnt.ID)
		if err != nil {
			klog.Warningf("failed to inspect container %s: %v", cnt.ID, err)
			continue
		}

		// Use container name as key (without leading /)
		containerName := cnt.Names[0]
		if len(containerName) > 0 && containerName[0] == '/' {
			containerName = containerName[1:]
		}
		// Store pointer to containerInfo
		infoCopy := containerInfo
		c.cache[containerName] = &infoCopy
	}

	klog.V(4).Infof("Container cache updated, number of containers: %d", len(c.cache))
}

// Stop stops cache refresh
func (c *AEnvContainerCache) Stop() {
	close(c.stopCh)
}

// GetContainer gets Container from cache
func (c *AEnvContainerCache) GetContainer(name string) (*types.ContainerJSON, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	containerInfo, exists := c.cache[name]
	if !exists {
		return nil, fmt.Errorf("container %s not found in cache", name)
	}
	return containerInfo, nil
}

// ListContainers lists all containers from cache
func (c *AEnvContainerCache) ListContainers() ([]*types.ContainerJSON, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	containers := make([]*types.ContainerJSON, 0, len(c.cache))
	for _, containerInfo := range c.cache {
		containers = append(containers, containerInfo)
	}
	return containers, nil
}

// ListExpiredContainers list all expired containers
func (c *AEnvContainerCache) ListExpiredContainers() ([]types.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := filters.NewArgs()
	filter.Add("label", "aenv.managed=true")

	containers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, err
	}

	expired := make([]types.Container, 0)
	for _, cnt := range containers {
		// Get container details for TTL
		containerInfo, err := c.dockerClient.ContainerInspect(ctx, cnt.ID)
		if err != nil {
			klog.Warningf("failed to inspect container %s: %v", cnt.ID, err)
			continue
		}

		ttlValue := ""
		if containerInfo.Config != nil && containerInfo.Config.Labels != nil {
			ttlValue = containerInfo.Config.Labels[constants.AENV_TTL]
		}
		if ttlValue == "" {
			continue
		}

		var limited time.Duration
		if limited, err = time.ParseDuration(ttlValue); err != nil {
			klog.Warningf("Failed to parse ttl value %s for container %s will not auto clean", ttlValue, cnt.ID)
			continue
		}

		createdAt := time.Now()
		if containerInfo.Created != "" {
			if t, err := time.Parse(time.RFC3339Nano, containerInfo.Created); err == nil {
				createdAt = t
			}
		}

		currentTime := time.Now()
		if currentTime.Sub(createdAt) > limited {
			containerName := cnt.Names[0]
			if len(containerName) > 0 && containerName[0] == '/' {
				containerName = containerName[1:]
			}
			klog.Infof("Instance %s has expired (created: %s, ttl: %v), deleting...", containerName, createdAt, limited)
			expired = append(expired, cnt)
		}
	}
	return expired, nil
}

