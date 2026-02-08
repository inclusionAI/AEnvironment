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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	backend "envhub/models"

	log "github.com/sirupsen/logrus"

	"api-service/models"
)

// DockerClient is a client for Docker Engine through Controller
type DockerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewDockerClient creates a new Docker client
func NewDockerClient(baseURL string) *DockerClient {
	return &DockerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateContainer creates a Docker container
func (c *DockerClient) CreateContainer(req *backend.Env) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/containers", c.baseURL)

	jsonData, err := req.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !createResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", createResp.Code)
	}

	return &createResp.Data, nil
}

// GetContainer queries a Docker container
func (c *DockerClient) GetContainer(containerID string) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/containers/%s", c.baseURL, containerID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var getResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", getResp.Code)
	}

	return &getResp.Data, nil
}

// DeleteContainer deletes a Docker container
func (c *DockerClient) DeleteContainer(containerID string) (bool, error) {
	url := fmt.Sprintf("%s/containers/%s", c.baseURL, containerID)

	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}
	var deleteResp models.ClientResponse[bool]
	if err := json.Unmarshal(body, &deleteResp); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !deleteResp.Success {
		return false, fmt.Errorf("server returned error, code: %d", deleteResp.Code)
	}

	return deleteResp.Data, nil
}

// ListContainers lists Docker containers, optionally filtered by envName or expired status
func (c *DockerClient) ListContainers(envName string, filterExpired bool) (*[]models.EnvInstance, error) {
	url := fmt.Sprintf("%s/containers", c.baseURL)

	// Build query parameters
	queryParams := ""
	if envName != "" {
		queryParams = fmt.Sprintf("?envName=%s", envName)
	}
	if filterExpired {
		if queryParams == "" {
			queryParams = "?filter=expired"
		} else {
			queryParams += "&filter=expired"
		}
	}
	url += queryParams

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var getResp models.ClientResponse[[]models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", getResp.Code)
	}

	return &getResp.Data, nil
}

/*
====================================
==== EnvInstanceService adapter ====
====================================
*/

// CreateEnvInstance implements EnvInstanceService interface - delegate to CreateContainer
func (c *DockerClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	return c.CreateContainer(req)
}

// GetEnvInstance implements EnvInstanceService interface - delegate to GetContainer
func (c *DockerClient) GetEnvInstance(id string) (*models.EnvInstance, error) {
	return c.GetContainer(id)
}

// DeleteEnvInstance implements EnvInstanceService interface - delegate to DeleteContainer
func (c *DockerClient) DeleteEnvInstance(id string) error {
	success, err := c.DeleteContainer(id)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("failed to delete env instance with id: %s", id)
	}
	return nil
}

// ListEnvInstances implements EnvInstanceService interface
// Lists environment instances, optionally filtered by environment name
func (c *DockerClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	instances, err := c.ListContainers(envName, false)
	if err != nil {
		return nil, err
	}

	// Convert []models.EnvInstance to []*models.EnvInstance
	result := make([]*models.EnvInstance, len(*instances))
	for i := range *instances {
		result[i] = &(*instances)[i]
	}

	return result, nil
}

// Warmup implements EnvInstanceService interface
// Warmup is not currently implemented for Docker engine
func (c *DockerClient) Warmup(req *backend.Env) error {
	return fmt.Errorf("warmup is not implemented for Docker engine")
}

// Cleanup implements EnvInstanceService interface
// Cleanup expired containers based on TTL
func (c *DockerClient) Cleanup() error {
	log.Infof("Starting Docker cleanup task...")
	// Get all expired containers
	envInstances, err := c.ListContainers("", true)
	if err != nil {
		return fmt.Errorf("failed to get expired containers: %v", err)
	}
	if envInstances == nil || len(*envInstances) == 0 {
		log.Infof("No expired containers found")
		return nil
	}

	var deletedCount int

	for _, instance := range *envInstances {
		// Skip already terminated containers
		if instance.Status == "Terminated" {
			continue
		}
		deleted, err := c.DeleteContainer(instance.ID)
		if err != nil {
			log.Warnf("Failed to delete container %s: %v", instance.ID, err)
			continue
		}
		if deleted {
			deletedCount++
			log.Infof("Successfully deleted container %s", instance.ID)
		} else {
			log.Infof("Container %s was not deleted (may already be deleted)", instance.ID)
		}
	}
	log.Infof("Docker cleanup task completed. Deleted %d expired containers", deletedCount)
	return nil
}
