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
	backend "envhub/models"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"api-service/models"
)

// ScheduleClient is a client for Schedule service
type ScheduleClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewScheduleClient creates a new Schedule client
func NewScheduleClient(baseURL string) *ScheduleClient {
	return &ScheduleClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreatePod creates a Pod
func (c *ScheduleClient) CreatePod(req *backend.Env) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/pods", c.baseURL)

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
	defer resp.Body.Close()

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

// GetPod queries a Pod
func (c *ScheduleClient) GetPod(podName string) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/pods/%s", c.baseURL, podName)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

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

// DeletePod deletes a Pod
func (c *ScheduleClient) DeletePod(podName string) (bool, error) {
	url := fmt.Sprintf("%s/pods/%s", c.baseURL, podName)

	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

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

// FilterPod filter pods by condition
func (c *ScheduleClient) FilterPods() (*[]models.EnvInstance, error) {
	url := fmt.Sprintf("%s/pods?filter=expired", c.baseURL)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

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

// CreateEnvInstance implements EnvInstanceService interface - delegate to CreatePod
func (c *ScheduleClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	return c.CreatePod(req)
}

// GetEnvInstance implements EnvInstanceService interface - delegate to GetPod
func (c *ScheduleClient) GetEnvInstance(id string) (*models.EnvInstance, error) {
	return c.GetPod(id)
}

// DeleteEnvInstance implements EnvInstanceService interface - delegate to DeletePod
func (c *ScheduleClient) DeleteEnvInstance(id string) error {
	success, err := c.DeletePod(id)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("failed to delete env instance with id: %s", id)
	}
	return nil
}

// ListEnvInstances implements EnvInstanceService interface - not implemented yet
func (c *ScheduleClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	return nil, fmt.Errorf("ListEnvInstances is not implemented")
}

func (c *ScheduleClient) Warmup(req *backend.Env) error {
	return fmt.Errorf("warmup is not implemented")
}

func (c *ScheduleClient) Cleanup() error {
	log.Infof("Starting cleanup task...")
	// get all EnvInstance
	envInstances, err := c.FilterPods()
	if err != nil {
		return fmt.Errorf("failed to get env instances: %v", err)
	}
	if envInstances == nil || len(*envInstances) == 0 {
		log.Infof("No env instances found")
		return nil
	}

	var deletedCount int

	for _, instance := range *envInstances {
		// skip terminated env instance
		if instance.Status == "Terminated" {
			continue
		}
		deleted, err := c.DeletePod(instance.ID)
		if err != nil {
			log.Warnf("Failed to delete instance %s: %v", instance.ID, err)
			continue
		}
		if deleted {
			deletedCount++
			log.Infof("Successfully deleted instance %s", instance.ID)
		} else {
			log.Infof("Instance %s was not deleted (may already be deleted)", instance.ID)
		}
	}
	log.Infof("Cleanup task completed. Deleted %d expired instances", deletedCount)
	return nil
}
