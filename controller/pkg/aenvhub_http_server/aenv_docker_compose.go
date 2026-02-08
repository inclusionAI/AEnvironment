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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"controller/pkg/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"k8s.io/klog"
)

// ComposeProject represents a Docker Compose project
type ComposeProject struct {
	ID          string
	Name        string
	ComposeFile string
	ProjectName string
	MainService string
	Services    []string
	Status      string
	IP          string
	TTL         string
	Owner       string
	CreatedAt   time.Time
}

// createComposeStack creates a Docker Compose stack
func (h *AEnvDockerHandler) createComposeStack(w http.ResponseWriter, r *http.Request, aenvHubEnv *model.AEnvHubEnv) {
	ctx := context.Background()

	// Generate project ID
	projectID := generateContainerID(aenvHubEnv.Name)
	projectName := fmt.Sprintf("aenv-%s", projectID)

	klog.Infof("Creating Compose stack %s for env %s", projectName, aenvHubEnv.Name)

	// Extract composeFile content
	composeFileContent, ok := aenvHubEnv.DeployConfig["composeFile"].(string)
	if !ok {
		http.Error(w, "Invalid composeFile content", http.StatusBadRequest)
		return
	}

	// Create temporary compose file
	tmpDir := "/tmp"
	composeFilePath := filepath.Join(tmpDir, fmt.Sprintf("aenv-compose-%s.yaml", projectID))

	if err := os.WriteFile(composeFilePath, []byte(composeFileContent), 0644); err != nil {
		klog.Errorf("Failed to write compose file: %v", err)
		http.Error(w, fmt.Sprintf("Failed to write compose file: %v", err), http.StatusInternalServerError)
		return
	}

	klog.Infof("Compose file written to %s", composeFilePath)

	// Add metadata to compose file (inject labels)
	if err := h.injectComposeLabels(composeFilePath, aenvHubEnv, projectID); err != nil {
		klog.Warningf("Failed to inject labels to compose file: %v", err)
	}

	// Detect docker-compose command (V2: docker compose, V1: docker-compose)
	composeCmd := h.detectComposeCommand()
	klog.Infof("Using compose command: %s", composeCmd)

	// Execute docker-compose up -d
	var cmd *exec.Cmd
	if composeCmd == "docker compose" {
		cmd = exec.Command("docker", "compose", "-f", composeFilePath, "-p", projectName, "up", "-d")
	} else {
		cmd = exec.Command("docker-compose", "-f", composeFilePath, "-p", projectName, "up", "-d")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("Failed to start compose stack: %v, output: %s", err, string(output))
		http.Error(w, fmt.Sprintf("Failed to start compose stack: %v, output: %s", err, string(output)), http.StatusInternalServerError)
		return
	}

	klog.Infof("Compose stack started: %s", string(output))

	// Wait for containers to be created (max 10 seconds)
	time.Sleep(2 * time.Second)

	// Query containers by project label
	containers, err := h.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		klog.Errorf("Failed to list containers: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter containers belonging to this project
	var projectContainers []types.Container
	for _, c := range containers {
		if c.Labels["com.docker.compose.project"] == projectName {
			projectContainers = append(projectContainers, c)
		}
	}

	if len(projectContainers) == 0 {
		klog.Errorf("No containers found for project %s", projectName)
		http.Error(w, fmt.Sprintf("No containers found for project %s", projectName), http.StatusInternalServerError)
		return
	}

	klog.Infof("Found %d containers for project %s", len(projectContainers), projectName)

	// Find main service (marked with aenv.main=true or first service)
	var mainContainer *types.Container
	for i, c := range projectContainers {
		if c.Labels["aenv.main"] == "true" {
			mainContainer = &projectContainers[i]
			break
		}
	}
	if mainContainer == nil {
		mainContainer = &projectContainers[0]
	}

	// Extract IP from main container
	ipAddress := ""
	for _, network := range mainContainer.NetworkSettings.Networks {
		if network.IPAddress != "" {
			ipAddress = network.IPAddress
			break
		}
	}

	// Get TTL from labels
	ttl := mainContainer.Labels["aenv.ttl"]
	owner := mainContainer.Labels["aenv.owner"]

	// Cache the compose project
	h.containerCache.Add(projectID, &CachedContainer{
		ID:        projectID,
		Status:    "Running",
		IP:        ipAddress,
		EnvName:   aenvHubEnv.Name,
		Version:   aenvHubEnv.Version,
		Owner:     owner,
		TTL:       ttl,
		CreatedAt: time.Now(),
	})

	klog.Infof("Compose stack %s created successfully, main IP: %s", projectName, ipAddress)

	// Build response
	response := HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     projectID,
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

// detectComposeCommand detects docker-compose command version
func (h *AEnvDockerHandler) detectComposeCommand() string {
	// Try docker compose (V2) first
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		return "docker compose"
	}

	// Fall back to docker-compose (V1)
	cmd = exec.Command("docker-compose", "version")
	if err := cmd.Run(); err == nil {
		return "docker-compose"
	}

	// Default to V2
	klog.Warningf("Neither 'docker compose' nor 'docker-compose' found, defaulting to 'docker compose'")
	return "docker compose"
}

// injectComposeLabels injects AEnv metadata labels into compose file
func (h *AEnvDockerHandler) injectComposeLabels(composeFilePath string, aenvHubEnv *model.AEnvHubEnv, projectID string) error {
	// Read compose file
	content, err := os.ReadFile(composeFilePath)
	if err != nil {
		return err
	}

	// Parse as YAML (simple string manipulation for labels)
	composeStr := string(content)

	// Prepare labels
	labels := fmt.Sprintf(`
    labels:
      - "aenv.env_name=%s"
      - "aenv.version=%s"
      - "aenv.owner="
      - "aenv.project_id=%s"`, aenvHubEnv.Name, aenvHubEnv.Version, projectID)

	// Add TTL if configured
	if ttl, ok := aenvHubEnv.DeployConfig["ttl"].(string); ok && ttl != "" {
		labels += fmt.Sprintf(`
      - "aenv.ttl=%s"
      - "aenv.created_at=%s"`, ttl, time.Now().Format(time.RFC3339))
	}

	// Inject labels after each service definition (simplified approach)
	// This is a basic implementation; a full YAML parser would be more robust
	lines := strings.Split(composeStr, "\n")
	var newLines []string
	inServices := false
	serviceIndent := 0

	for i, line := range lines {
		newLines = append(newLines, line)

		// Detect services section
		if strings.TrimSpace(line) == "services:" {
			inServices = true
			continue
		}

		// Detect service definition (indented after services:)
		if inServices && len(line) > 0 && line[0] == ' ' {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if serviceIndent == 0 || indent <= serviceIndent {
				serviceIndent = indent
				// Check if next lines have image: or build:
				if i+1 < len(lines) {
					nextLine := lines[i+1]
					if strings.Contains(nextLine, "image:") || strings.Contains(nextLine, "build:") {
						// Inject labels after this service
						newLines = append(newLines, labels)
					}
				}
			}
		}
	}

	// Write modified compose file
	modifiedContent := strings.Join(newLines, "\n")
	return os.WriteFile(composeFilePath, []byte(modifiedContent), 0644)
}

// deleteComposeStack deletes a Docker Compose stack
func (h *AEnvDockerHandler) deleteComposeStack(projectID string) error {
	projectName := fmt.Sprintf("aenv-%s", projectID)
	composeFilePath := filepath.Join("/tmp", fmt.Sprintf("aenv-compose-%s.yaml", projectID))

	klog.Infof("Deleting compose stack %s", projectName)

	// Check if compose file exists
	if _, err := os.Stat(composeFilePath); os.IsNotExist(err) {
		klog.Warningf("Compose file not found: %s, will try to stop containers by label", composeFilePath)
		return h.stopContainersByLabel(projectName)
	}

	// Detect compose command
	composeCmd := h.detectComposeCommand()

	// Execute docker-compose down
	var cmd *exec.Cmd
	if composeCmd == "docker compose" {
		cmd = exec.Command("docker", "compose", "-f", composeFilePath, "-p", projectName, "down")
	} else {
		cmd = exec.Command("docker-compose", "-f", composeFilePath, "-p", projectName, "down")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("Failed to stop compose stack: %v, output: %s", err, string(output))
		// Try to stop containers manually
		return h.stopContainersByLabel(projectName)
	}

	klog.Infof("Compose stack stopped: %s", string(output))

	// Remove compose file
	if err := os.Remove(composeFilePath); err != nil {
		klog.Warningf("Failed to remove compose file: %v", err)
	}

	return nil
}

// stopContainersByLabel stops all containers with a specific project label
func (h *AEnvDockerHandler) stopContainersByLabel(projectName string) error {
	ctx := context.Background()

	// List containers by project label
	containers, err := h.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Filter and stop containers
	for _, c := range containers {
		if c.Labels["com.docker.compose.project"] == projectName {
			klog.Infof("Stopping container %s", c.ID)
			timeout := 10
			stopOptions := container.StopOptions{
				Timeout: &timeout,
			}
			if err := h.dockerClient.ContainerStop(ctx, c.ID, stopOptions); err != nil {
				klog.Warningf("Failed to stop container %s: %v", c.ID, err)
			}

			// Remove container
			removeOptions := types.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			}
			if err := h.dockerClient.ContainerRemove(ctx, c.ID, removeOptions); err != nil {
				klog.Warningf("Failed to remove container %s: %v", c.ID, err)
			}
		}
	}

	return nil
}

// isComposeProject checks if a container ID is a compose project
func (h *AEnvDockerHandler) isComposeProject(containerID string) bool {
	// Check if there's a compose file for this ID
	composeFilePath := filepath.Join("/tmp", fmt.Sprintf("aenv-compose-%s.yaml", containerID))
	_, err := os.Stat(composeFilePath)
	return err == nil
}
