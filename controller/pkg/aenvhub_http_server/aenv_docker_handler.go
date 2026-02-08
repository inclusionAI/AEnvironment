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
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"controller/pkg/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"k8s.io/klog"
)

// AEnvDockerHandler handles Docker container CRUD operations
type AEnvDockerHandler struct {
	dockerClient   *client.Client
	containerCache *AEnvDockerCache
	composeEnabled bool
	defaultNetwork string
}

// NewAEnvDockerHandler creates new Docker handler with TLS support
func NewAEnvDockerHandler() (*AEnvDockerHandler, error) {
	// Get Docker host from environment variable
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	// Check if Compose is enabled
	composeEnabled := true
	if composeEnv := os.Getenv("COMPOSE_ENABLED"); composeEnv != "" {
		composeEnabled = composeEnv == "true"
	}

	// Get default network
	defaultNetwork := os.Getenv("DOCKER_NETWORK")
	if defaultNetwork == "" {
		defaultNetwork = "bridge"
	}

	// Get TLS configuration
	tlsVerify := os.Getenv("DOCKER_TLS_VERIFY") == "true"
	certPath := os.Getenv("DOCKER_CERT_PATH")

	// Build client options
	clientOpts := []client.Opt{
		client.WithHost(dockerHost),
		client.WithVersion("1.44"), // Explicitly set API version to 1.44
		client.WithAPIVersionNegotiation(),
	}

	// Add TLS configuration if enabled
	if tlsVerify && certPath != "" {
		klog.Infof("TLS verification enabled, cert path: %s", certPath)
		clientOpts = append(clientOpts, client.WithTLSClientConfig(
			certPath+"/cert.pem",
			certPath+"/key.pem",
			certPath+"/ca.pem",
		))
	}

	// Create Docker client with options
	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	// Health check: ping Docker daemon
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker daemon unreachable at %s: %v", dockerHost, err)
	}

	handler := &AEnvDockerHandler{
		dockerClient:   cli,
		composeEnabled: composeEnabled,
		defaultNetwork: defaultNetwork,
	}

	// Initialize container cache
	containerCache := NewAEnvDockerCache(cli)
	handler.containerCache = containerCache

	klog.Infof("AEnv Docker handler created, host: %s, network: %s, compose: %v, TLS: %v",
		dockerHost, defaultNetwork, composeEnabled, tlsVerify)

	return handler, nil
}

// ServeHTTP main routing method
func (h *AEnvDockerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 || parts[1] != "containers" {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	klog.Infof("access URL path %s, method %s, host %s", r.URL.Path, r.Method, r.Host)

	// Route handling
	switch {
	case r.Method == http.MethodPost && len(parts) == 2: // /containers
		h.createContainer(w, r)
	case r.Method == http.MethodGet && len(parts) == 2: // /containers/
		h.listContainers(w, r)
	case r.Method == http.MethodGet && len(parts) == 3: // /containers/{containerID}
		containerID := parts[2]
		h.getContainer(containerID, w, r)
	case r.Method == http.MethodDelete && len(parts) == 3: // /containers/{containerID}
		containerID := parts[2]
		h.deleteContainer(containerID, w, r)
	default:
		http.Error(w, "http method not allowed", http.StatusMethodNotAllowed)
	}
}

// generateContainerID generates a unique container ID
func generateContainerID(envName string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	suffix := make([]byte, 12)
	for i := range suffix {
		suffix[i] = charset[r.Intn(len(charset))]
	}
	return fmt.Sprintf("docker-%s-%s", envName, string(suffix))
}

// createContainer creates a new Docker container
func (h *AEnvDockerHandler) createContainer(w http.ResponseWriter, r *http.Request) {
	var aenvHubEnv model.AEnvHubEnv
	if err := json.NewDecoder(r.Body).Decode(&aenvHubEnv); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			klog.Errorf("failed to close request body: %v", closeErr)
		}
	}()

	klog.Infof("received env deploy config: %v", aenvHubEnv.DeployConfig)

	// Check if composeFile exists for Compose mode
	composeFile, hasCompose := aenvHubEnv.DeployConfig["composeFile"]
	if hasCompose && composeFile != nil {
		if h.composeEnabled {
			h.createComposeStack(w, r, &aenvHubEnv)
			return
		}
		klog.Warningf("Compose file provided but Compose support is disabled")
	}

	// Single container mode
	h.createSingleContainer(w, r, &aenvHubEnv)
}

// createSingleContainer creates a single Docker container
func (h *AEnvDockerHandler) createSingleContainer(w http.ResponseWriter, r *http.Request, aenvHubEnv *model.AEnvHubEnv) {
	ctx := context.Background()

	// Generate container ID
	containerID := generateContainerID(aenvHubEnv.Name)

	// Extract image from artifacts
	image := h.extractImageFromArtifacts(aenvHubEnv)
	if image == "" {
		http.Error(w, "No image found in artifacts", http.StatusBadRequest)
		return
	}

	klog.Infof("Creating container %s with image %s", containerID, image)

	// Parse resource limits
	resources := h.parseResourceLimits(aenvHubEnv.DeployConfig)

	// Parse environment variables
	envVars := h.parseEnvironmentVariables(aenvHubEnv.DeployConfig)

	// Add AEnv metadata labels
	labels := map[string]string{
		"aenv.env_name": aenvHubEnv.Name,
		"aenv.version":  aenvHubEnv.Version,
		"aenv.owner":    "", // Owner not available in AEnvHubEnv model
	}

	// Add TTL label if configured
	if ttl, ok := aenvHubEnv.DeployConfig["ttl"].(string); ok && ttl != "" {
		labels["aenv.ttl"] = ttl
		labels["aenv.created_at"] = time.Now().Format(time.RFC3339)
	}

	// Container configuration
	containerConfig := &container.Config{
		Image:  image,
		Env:    envVars,
		Labels: labels,
		ExposedPorts: nat.PortSet{
			"8081/tcp": struct{}{},
		},
	}

	// Host configuration with resource limits
	hostConfig := &container.HostConfig{
		Resources:   resources,
		NetworkMode: container.NetworkMode(h.defaultNetwork),
		RestartPolicy: container.RestartPolicy{
			Name: "no",
		},
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{}

	// Create container
	resp, err := h.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerID)
	if err != nil {
		klog.Errorf("Failed to create container: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create container: %v", err), http.StatusInternalServerError)
		return
	}

	// Start container
	if err := h.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		klog.Errorf("Failed to start container: %v", err)
		http.Error(w, fmt.Sprintf("Failed to start container: %v", err), http.StatusInternalServerError)
		return
	}

	// Inspect container to get IP
	containerJSON, err := h.dockerClient.ContainerInspect(ctx, resp.ID)
	if err != nil {
		klog.Errorf("Failed to inspect container: %v", err)
		http.Error(w, fmt.Sprintf("Failed to inspect container: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract IP address
	ipAddress := ""
	if containerJSON.NetworkSettings != nil {
		for _, network := range containerJSON.NetworkSettings.Networks {
			if network.IPAddress != "" {
				ipAddress = network.IPAddress
				break
			}
		}
	}

	// Get TTL from labels
	ttl := containerJSON.Config.Labels["aenv.ttl"]
	owner := containerJSON.Config.Labels["aenv.owner"]

	// Update cache
	h.containerCache.Add(resp.ID, &CachedContainer{
		ID:        resp.ID,
		Status:    "Running",
		IP:        ipAddress,
		EnvName:   aenvHubEnv.Name,
		Version:   aenvHubEnv.Version,
		Owner:     owner,
		TTL:       ttl,
		CreatedAt: time.Now(),
	})

	klog.Infof("Container %s created successfully, IP: %s", resp.ID, ipAddress)

	// Build response
	response := HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     resp.ID,
			Status: "Running",
			IP:     ipAddress,
			TTL:    ttl,
			Owner:  owner,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode response: %v", err)
	}
}

// extractImageFromArtifacts extracts Docker image from artifacts
func (h *AEnvDockerHandler) extractImageFromArtifacts(aenvHubEnv *model.AEnvHubEnv) string {
	// First try to get image from DeployConfig (for Docker standalone mode)
	if aenvHubEnv.DeployConfig != nil {
		if imageName, ok := aenvHubEnv.DeployConfig["imageName"].(string); ok && imageName != "" {
			return imageName
		}
	}

	// Fall back to Artifacts (for backend mode)
	for _, artifact := range aenvHubEnv.Artifacts {
		if artifact.Type == "image" {
			return artifact.Content
		}
	}
	return ""
}

// parseResourceLimits parses CPU and memory limits from deploy config
func (h *AEnvDockerHandler) parseResourceLimits(deployConfig map[string]interface{}) container.Resources {
	resources := container.Resources{}

	// Parse CPU
	if cpuStr, ok := deployConfig["cpu"].(string); ok && cpuStr != "" {
		// Parse format like "1C", "2.0C", "500m"
		cpuStr = strings.TrimSuffix(cpuStr, "C")
		cpuStr = strings.TrimSuffix(cpuStr, "m")
		if cpuFloat, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			// Convert to NanoCPUs (1 CPU = 1e9 NanoCPUs)
			if strings.HasSuffix(cpuStr, "m") {
				resources.NanoCPUs = int64(cpuFloat * 1e6) // millicores
			} else {
				resources.NanoCPUs = int64(cpuFloat * 1e9)
			}
		}
	}

	// Parse Memory
	if memStr, ok := deployConfig["memory"].(string); ok && memStr != "" {
		// Parse format like "2G", "2Gi", "512M"
		memStr = strings.ToUpper(memStr)
		memStr = strings.TrimSuffix(memStr, "I") // Remove 'i' from Gi, Mi

		var multiplier int64 = 1
		if strings.HasSuffix(memStr, "G") {
			multiplier = 1024 * 1024 * 1024
			memStr = strings.TrimSuffix(memStr, "G")
		} else if strings.HasSuffix(memStr, "M") {
			multiplier = 1024 * 1024
			memStr = strings.TrimSuffix(memStr, "M")
		}

		if memFloat, err := strconv.ParseFloat(memStr, 64); err == nil {
			resources.Memory = int64(memFloat * float64(multiplier))
		}
	}

	return resources
}

// parseEnvironmentVariables parses environment variables from deploy config
func (h *AEnvDockerHandler) parseEnvironmentVariables(deployConfig map[string]interface{}) []string {
	envVars := []string{}

	if envMap, ok := deployConfig["env"].(map[string]interface{}); ok {
		for key, value := range envMap {
			envVars = append(envVars, fmt.Sprintf("%s=%v", key, value))
		}
	}

	return envVars
}

// getContainer retrieves container information
func (h *AEnvDockerHandler) getContainer(containerID string, w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Try to get from cache first
	if cached := h.containerCache.Get(containerID); cached != nil {
		response := HttpResponse{
			Success: true,
			Code:    0,
			ResponseData: HttpResponseData{
				ID:     cached.ID,
				Status: cached.Status,
				IP:     cached.IP,
				TTL:    cached.TTL,
				Owner:  cached.Owner,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			klog.Errorf("Failed to encode response: %v", err)
		}
		return
	}

	// Inspect container from Docker daemon
	containerJSON, err := h.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			http.Error(w, fmt.Sprintf("Container not found: %s", containerID), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to inspect container: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Extract IP address
	ipAddress := ""
	if containerJSON.NetworkSettings != nil {
		for _, network := range containerJSON.NetworkSettings.Networks {
			if network.IPAddress != "" {
				ipAddress = network.IPAddress
				break
			}
		}
	}

	// Map container state to status
	status := "Unknown"
	if containerJSON.State != nil {
		if containerJSON.State.Running {
			status = "Running"
		} else if containerJSON.State.Restarting {
			status = "Restarting"
		} else if containerJSON.State.Paused {
			status = "Paused"
		} else if containerJSON.State.Dead {
			status = "Dead"
		} else {
			status = "Exited"
		}
	}

	response := HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     containerJSON.ID,
			Status: status,
			IP:     ipAddress,
			TTL:    containerJSON.Config.Labels["aenv.ttl"],
			Owner:  containerJSON.Config.Labels["aenv.owner"],
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode response: %v", err)
	}
}

// deleteContainer deletes a Docker container or Compose stack
func (h *AEnvDockerHandler) deleteContainer(containerID string, w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Check if this is a Compose project
	if h.isComposeProject(containerID) {
		klog.Infof("Detected Compose project, deleting stack: %s", containerID)
		if err := h.deleteComposeStack(containerID); err != nil {
			klog.Errorf("Failed to delete compose stack: %v", err)
			http.Error(w, fmt.Sprintf("Failed to delete compose stack: %v", err), http.StatusInternalServerError)
			return
		}

		// Remove from cache
		h.containerCache.Remove(containerID)

		klog.Infof("Compose stack %s deleted successfully", containerID)

		response := HttpDeleteResponse{
			Success:      true,
			Code:         0,
			ResponseData: true,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			klog.Errorf("Failed to encode response: %v", err)
		}
		return
	}

	// Single container deletion
	// Stop container first
	timeout := 10
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}
	if err := h.dockerClient.ContainerStop(ctx, containerID, stopOptions); err != nil {
		if !client.IsErrNotFound(err) {
			klog.Warningf("Failed to stop container %s: %v", containerID, err)
		}
	}

	// Remove container
	removeOptions := types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}
	if err := h.dockerClient.ContainerRemove(ctx, containerID, removeOptions); err != nil {
		if client.IsErrNotFound(err) {
			http.Error(w, fmt.Sprintf("Container not found: %s", containerID), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to remove container: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Remove from cache
	h.containerCache.Remove(containerID)

	klog.Infof("Container %s deleted successfully", containerID)

	response := HttpDeleteResponse{
		Success:      true,
		Code:         0,
		ResponseData: true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode response: %v", err)
	}
}

// listContainers lists Docker containers with optional filters
func (h *AEnvDockerHandler) listContainers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Parse query parameters
	envName := r.URL.Query().Get("envName")
	filterExpired := r.URL.Query().Get("filter") == "expired"

	// List all containers (we'll filter manually)
	containers, err := h.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter and build response
	responseData := []HttpListResponseData{}
	now := time.Now()

	for _, c := range containers {
		// Check if container has AEnv labels
		if _, ok := c.Labels["aenv.env_name"]; !ok {
			continue
		}

		// Filter by envName if specified
		if envName != "" && c.Labels["aenv.env_name"] != envName {
			continue
		}

		// Check if expired (if filter=expired)
		if filterExpired {
			ttl := c.Labels["aenv.ttl"]
			createdAt := c.Labels["aenv.created_at"]
			if !h.isExpired(ttl, createdAt, now) {
				continue
			}
		}

		// Extract IP
		ipAddress := ""
		for _, network := range c.NetworkSettings.Networks {
			if network.IPAddress != "" {
				ipAddress = network.IPAddress
				break
			}
		}

		// Map status
		status := c.State
		switch status {
		case "running":
			status = "Running"
		case "exited":
			status = "Terminated"
		default:
			status = "Unknown"
		}

		responseData = append(responseData, HttpListResponseData{
			ID:        c.ID,
			Status:    status,
			TTL:       c.Labels["aenv.ttl"],
			CreatedAt: time.Unix(c.Created, 0),
			EnvName:   c.Labels["aenv.env_name"],
			Version:   c.Labels["aenv.version"],
			IP:        ipAddress,
			Owner:     c.Labels["aenv.owner"],
		})
	}

	response := HttpListResponse{
		Success:          true,
		Code:             0,
		ListResponseData: responseData,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode response: %v", err)
	}
}

// isExpired checks if a container has expired based on TTL
func (h *AEnvDockerHandler) isExpired(ttl, createdAt string, now time.Time) bool {
	if ttl == "" || createdAt == "" {
		return false
	}

	// Parse created time
	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		klog.Warningf("Failed to parse created_at: %v", err)
		return false
	}

	// Parse TTL duration
	duration, err := time.ParseDuration(ttl)
	if err != nil {
		klog.Warningf("Failed to parse TTL: %v", err)
		return false
	}

	// Check if expired
	expireTime := created.Add(duration)
	return now.After(expireTime)
}
