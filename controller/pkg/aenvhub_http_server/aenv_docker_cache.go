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

package aenvhub_http_server

import (
	"context"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"k8s.io/klog"
)

// CachedContainer represents a cached container state
type CachedContainer struct {
	ID        string
	Status    string
	IP        string
	EnvName   string
	Version   string
	Owner     string
	TTL       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AEnvDockerCache caches Docker container states
type AEnvDockerCache struct {
	dockerClient *client.Client
	cache        map[string]*CachedContainer
	mutex        sync.RWMutex
	stopChan     chan struct{}
}

// NewAEnvDockerCache creates a new Docker container cache
func NewAEnvDockerCache(dockerClient *client.Client) *AEnvDockerCache {
	cache := &AEnvDockerCache{
		dockerClient: dockerClient,
		cache:        make(map[string]*CachedContainer),
		stopChan:     make(chan struct{}),
	}

	// Start background sync goroutine
	go cache.syncLoop()

	klog.Infof("Docker container cache initialized, sync interval: 30s")

	return cache
}

// Add adds a container to the cache
func (c *AEnvDockerCache) Add(containerID string, container *CachedContainer) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	container.UpdatedAt = time.Now()
	c.cache[containerID] = container
	klog.V(4).Infof("Added container %s to cache", containerID)
}

// Get retrieves a container from the cache
func (c *AEnvDockerCache) Get(containerID string) *CachedContainer {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.cache[containerID]
}

// Remove removes a container from the cache
func (c *AEnvDockerCache) Remove(containerID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.cache, containerID)
	klog.V(4).Infof("Removed container %s from cache", containerID)
}

// List returns all containers in the cache
func (c *AEnvDockerCache) List() []*CachedContainer {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	containers := make([]*CachedContainer, 0, len(c.cache))
	for _, container := range c.cache {
		containers = append(containers, container)
	}

	return containers
}

// syncLoop periodically syncs cache with Docker daemon
func (c *AEnvDockerCache) syncLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sync()
		case <-c.stopChan:
			klog.Infof("Docker cache sync loop stopped")
			return
		}
	}
}

// sync synchronizes cache with Docker daemon
func (c *AEnvDockerCache) sync() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List all containers with AEnv labels
	containers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		klog.Errorf("Failed to list containers during sync: %v", err)
		return
	}

	// Build a set of current container IDs
	currentIDs := make(map[string]bool)

	// Update cache with current containers
	for _, dockerContainer := range containers {
		// Only cache AEnv containers
		if _, ok := dockerContainer.Labels["aenv.env_name"]; !ok {
			continue
		}

		currentIDs[dockerContainer.ID] = true

		// Extract IP
		ipAddress := ""
		for _, network := range dockerContainer.NetworkSettings.Networks {
			if network.IPAddress != "" {
				ipAddress = network.IPAddress
				break
			}
		}

		// Map status
		status := dockerContainer.State
		switch status {
		case "running":
			status = "Running"
		case "exited":
			status = "Terminated"
		case "created":
			status = "Creating"
		default:
			status = "Unknown"
		}

		// Parse created time
		createdAt := time.Unix(dockerContainer.Created, 0)

		// Update or add to cache
		cached := &CachedContainer{
			ID:        dockerContainer.ID,
			Status:    status,
			IP:        ipAddress,
			EnvName:   dockerContainer.Labels["aenv.env_name"],
			Version:   dockerContainer.Labels["aenv.version"],
			Owner:     dockerContainer.Labels["aenv.owner"],
			TTL:       dockerContainer.Labels["aenv.ttl"],
			CreatedAt: createdAt,
			UpdatedAt: time.Now(),
		}

		c.Add(dockerContainer.ID, cached)
	}

	// Remove containers that no longer exist
	c.mutex.Lock()
	for id := range c.cache {
		if !currentIDs[id] {
			delete(c.cache, id)
			klog.V(4).Infof("Removed stale container %s from cache", id)
		}
	}
	c.mutex.Unlock()

	klog.V(4).Infof("Docker cache synced, total containers: %d", len(currentIDs))
}

// Stop stops the cache sync loop
func (c *AEnvDockerCache) Stop() {
	close(c.stopChan)
}

// GetExpiredContainers returns all expired containers
func (c *AEnvDockerCache) GetExpiredContainers() []*CachedContainer {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	expired := []*CachedContainer{}
	now := time.Now()

	for _, container := range c.cache {
		if container.TTL == "" {
			continue
		}

		// Parse TTL duration
		duration, err := time.ParseDuration(container.TTL)
		if err != nil {
			klog.Warningf("Failed to parse TTL for container %s: %v", container.ID, err)
			continue
		}

		// Check if expired
		expireTime := container.CreatedAt.Add(duration)
		if now.After(expireTime) {
			expired = append(expired, container)
		}
	}

	return expired
}
