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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"docker-controller/pkg/constants"
	"docker-controller/pkg/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"k8s.io/klog"
)

// AEnvContainerHandler handles Docker Container CRUD operations
type AEnvContainerHandler struct {
	dockerClient   *client.Client
	containerCache *AEnvContainerCache
	labelPrefix    string
}

// NewAEnvContainerHandler creates new ContainerHandler
func NewAEnvContainerHandler() (*AEnvContainerHandler, error) {
	dockerSocket := os.Getenv("DOCKER_HOST")
	if dockerSocket == "" {
		dockerSocket = "unix:///var/run/docker.sock"
	}

	cli, err := client.NewClientWithOpts(client.WithHost(dockerSocket), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %v", err)
	}

	containerHandler := &AEnvContainerHandler{
		dockerClient: cli,
		labelPrefix:  "aenv",
	}

	// Initialize Container cache
	containerCache := NewAEnvContainerCache(cli)
	containerHandler.containerCache = containerCache

	klog.Infof("AEnv container handler is created, Docker socket: %s", dockerSocket)

	return containerHandler, nil
}

// ServeHTTP main routing method
func (h *AEnvContainerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 || parts[1] != "pods" {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	klog.Infof("access URL path %s, method %s, host %s", r.URL.Path, r.Method, r.Host)

	// Route handling
	switch {
	case r.Method == http.MethodPost && len(parts) == 2: // /pods
		h.createContainer(w, r)
	case r.Method == http.MethodGet && len(parts) == 2: // /pods/
		h.listContainer(w, r)
	case r.Method == http.MethodGet && len(parts) == 3: // /pods/{containerName}
		containerName := parts[2]
		h.getContainer(containerName, w, r)
	case r.Method == http.MethodDelete && len(parts) == 3: // /pods/{containerName}
		containerName := parts[2]
		h.deleteContainer(containerName, w, r)
	default:
		http.Error(w, "http method not allowed", http.StatusMethodNotAllowed)
	}
}

type HttpResponseData struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	TTL    string `json:"ttl"`
}

type HttpResponse struct {
	Success      bool             `json:"success"`
	Code         int              `json:"code"`
	ResponseData HttpResponseData `json:"data"`
}

type HttpDeleteResponse struct {
	Success      bool `json:"success"`
	Code         int  `json:"code"`
	ResponseData bool `json:"data"`
}

type HttpListResponseData struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	TTL       string    `json:"ttl"`
	CreatedAt time.Time `json:"created_at"`
}

type HttpListResponse struct {
	Success          bool                   `json:"success"`
	Code             int                    `json:"code"`
	ListResponseData []HttpListResponseData `json:"data"`
}

func (h *AEnvContainerHandler) createContainer(w http.ResponseWriter, r *http.Request) {
	var aenvHubEnv model.AEnvHubEnv
	if err := json.NewDecoder(r.Body).Decode(&aenvHubEnv); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get image from artifacts
	image := ""
	for _, artifact := range aenvHubEnv.Artifacts {
		if artifact.Type == "image" {
			image = artifact.Content
			break
		}
	}
	if image == "" {
		http.Error(w, "No image found in artifacts", http.StatusBadRequest)
		return
	}

	klog.Infof("received env deploy config: %v", aenvHubEnv.DeployConfig)

	// Generate container name
	containerName := fmt.Sprintf("%s-%s", aenvHubEnv.Name, RandString(6))

	// Prepare container config
	containerConfig := &container.Config{
		Image: image,
		Labels: map[string]string{
			"aenv.name":    aenvHubEnv.Name,
			"aenv.version": aenvHubEnv.Version,
			"aenv.managed": "true",
		},
	}

	// Set TTL label if provided
	if aenvHubEnv.DeployConfig["ttl"] != nil {
		ttlValue := fmt.Sprintf("%v", aenvHubEnv.DeployConfig["ttl"])
		containerConfig.Labels[constants.AENV_TTL] = ttlValue
		klog.Infof("add aenv-ttl label with value:%v for container:%s", ttlValue, containerName)
	}

	// Set environment variables
	if envVars, ok := aenvHubEnv.DeployConfig["environmentVariables"].(map[string]interface{}); ok {
		for k, v := range envVars {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%v", k, v))
		}
	}

	// Set arguments
	if args, ok := aenvHubEnv.DeployConfig["arguments"].([]interface{}); ok {
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				containerConfig.Cmd = append(containerConfig.Cmd, str)
			}
		}
	}

	// Handle second container (for terminal mode)
	if secondImageName, ok := aenvHubEnv.DeployConfig["secondImageName"]; ok {
		klog.Infof("secondImageName detected: %v, but Docker single container mode doesn't support sidecar", secondImageName)
	}

	// Prepare host config
	hostConfig := &container.HostConfig{
		AutoRemove: false, // We'll manage removal ourselves
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Set resource limits if provided
	if cpu, ok := aenvHubEnv.DeployConfig["cpu"].(string); ok {
		// Parse CPU (e.g., "1C" or "1000m")
		// Docker uses nano CPUs, so "1C" = 1000000000
		// For simplicity, we'll parse common formats
		if strings.HasSuffix(cpu, "C") {
			cpuValue := strings.TrimSuffix(cpu, "C")
			var nanoCPUs int64
			if _, err := fmt.Sscanf(cpuValue, "%d", &nanoCPUs); err == nil {
				hostConfig.Resources.NanoCPUs = nanoCPUs * 1000000000
			}
		}
	}

	if memory, ok := aenvHubEnv.DeployConfig["memory"].(string); ok {
		// Parse memory (e.g., "2G", "512M")
		if strings.HasSuffix(memory, "G") {
			memValue := strings.TrimSuffix(memory, "G")
			var memGB int64
			if _, err := fmt.Sscanf(memValue, "%d", &memGB); err == nil {
				hostConfig.Resources.Memory = memGB * 1024 * 1024 * 1024
			}
		} else if strings.HasSuffix(memory, "M") {
			memValue := strings.TrimSuffix(memory, "M")
			var memMB int64
			if _, err := fmt.Sscanf(memValue, "%d", &memMB); err == nil {
				hostConfig.Resources.Memory = memMB * 1024 * 1024
			}
		}
	}

	ctx := context.Background()

	// Create container
	createdContainer, err := h.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil, // networking config
		nil, // platform
		containerName,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create container: %v", err), http.StatusInternalServerError)
		return
	}

	// Start container
	if err := h.dockerClient.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{}); err != nil {
		// Try to remove the container if start fails
		h.dockerClient.ContainerRemove(ctx, createdContainer.ID, types.ContainerRemoveOptions{Force: true})
		http.Error(w, fmt.Sprintf("Failed to start container: %v", err), http.StatusInternalServerError)
		return
	}

	klog.Infof("created container %s successfully", containerName)

	// Get container info to get IP
	containerInfo, err := h.dockerClient.ContainerInspect(ctx, createdContainer.ID)
	if err != nil {
		klog.Warningf("failed to inspect container %s: %v", createdContainer.ID, err)
	}

	ip := ""
	if containerInfo.NetworkSettings != nil {
		ip = containerInfo.NetworkSettings.IPAddress
	}

	status := "Running"
	if containerInfo.State != nil {
		status = containerInfo.State.Status
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	res := &HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     containerName,
			Status: status,
			IP:     ip,
			TTL:    containerConfig.Labels[constants.AENV_TTL],
		},
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

func (h *AEnvContainerHandler) getContainer(containerName string, w http.ResponseWriter, r *http.Request) {
	if containerName == "" {
		http.Error(w, "missing container name", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get container from cache first
	var containerInfo *types.ContainerJSON
	var err error
	containerInfo, err = h.containerCache.GetContainer(containerName)
	if err != nil {
		// Fall back to Docker API
		info, err2 := h.dockerClient.ContainerInspect(ctx, containerName)
		if err2 != nil {
			if client.IsErrNotFound(err2) {
				http.Error(w, fmt.Sprintf("Container not found: %s", containerName), http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("Failed to get container: %v", err2), http.StatusInternalServerError)
			return
		}
		containerInfo = &info
	}

	ip := ""
	if containerInfo.NetworkSettings != nil {
		ip = containerInfo.NetworkSettings.IPAddress
	}

	status := "Unknown"
	if containerInfo.State != nil {
		status = containerInfo.State.Status
	}

	ttl := ""
	if containerInfo.Config != nil && containerInfo.Config.Labels != nil {
		ttl = containerInfo.Config.Labels[constants.AENV_TTL]
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	res := &HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     containerName,
			TTL:    ttl,
			Status: status,
			IP:     ip,
		},
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

func (h *AEnvContainerHandler) listContainer(w http.ResponseWriter, r *http.Request) {
	// query param:?filter=expired
	filterMark := r.URL.Query().Get("filter")

	ctx := context.Background()

	var containerList []types.Container
	var err error

	if filterMark == "expired" {
		containerList, err = h.containerCache.ListExpiredContainers()
		if err != nil {
			klog.Errorf("failed to list expired containers: %v", err)
			http.Error(w, fmt.Sprintf("Failed to list expired containers: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// List all containers with aenv label
		filter := filters.NewArgs()
		filter.Add("label", "aenv.managed=true")
		containerList, err = h.dockerClient.ContainerList(ctx, types.ContainerListOptions{
			All:     true,
			Filters: filter,
		})
		if err != nil {
			klog.Errorf("failed to list containers: %v", err)
			http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	httpListResponse := &HttpListResponse{
		Success: true,
		Code:    0,
	}

	for _, cnt := range containerList {
		// Get container details for TTL and creation time
		containerInfo, err := h.dockerClient.ContainerInspect(ctx, cnt.ID)
		if err != nil {
			klog.Warningf("failed to inspect container %s: %v", cnt.ID, err)
			continue
		}

		ttl := ""
		if containerInfo.Config != nil && containerInfo.Config.Labels != nil {
			ttl = containerInfo.Config.Labels[constants.AENV_TTL]
		}

		createdAt := time.Now()
		if containerInfo.Created != "" {
			if t, err := time.Parse(time.RFC3339Nano, containerInfo.Created); err == nil {
				createdAt = t
			}
		}

		status := cnt.Status
		// Normalize status to match Kubernetes pod status
		if strings.Contains(status, "Up") {
			status = "Running"
		} else if strings.Contains(status, "Exited") {
			status = "Terminated"
		} else if strings.Contains(status, "Created") {
			status = "Pending"
		}

		// Use container name (without leading /)
		containerName := cnt.Names[0]
		if len(containerName) > 0 && containerName[0] == '/' {
			containerName = containerName[1:]
		}

		httpListResponse.ListResponseData = append(httpListResponse.ListResponseData, HttpListResponseData{
			ID:        containerName,
			Status:    status,
			CreatedAt: createdAt,
			TTL:       ttl,
		})
	}

	err = json.NewEncoder(w).Encode(httpListResponse)
	if err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

func (h *AEnvContainerHandler) deleteContainer(containerName string, w http.ResponseWriter, r *http.Request) {
	if containerName == "" {
		http.Error(w, "missing container name", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Stop and remove container
	err := h.dockerClient.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		if client.IsErrNotFound(err) {
			http.Error(w, fmt.Sprintf("Container not found: %s", containerName), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to delete container: %v", err), http.StatusInternalServerError)
		return
	}

	klog.Infof("delete container %s successfully", containerName)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	res := &HttpDeleteResponse{
		Success:      true,
		Code:         0,
		ResponseData: true,
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

