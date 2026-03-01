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
==== Service Management Methods ====
====================================
*/

// CreateService creates a Service (Deployment + Service + PVC)
func (c *ScheduleClient) CreateService(req *backend.Env) (*models.EnvService, error) {
	url := fmt.Sprintf("%s/services", c.baseURL)

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

	var createResp models.ClientResponse[models.EnvService]
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !createResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", createResp.Code)
	}

	return &createResp.Data, nil
}

// GetService queries a Service
func (c *ScheduleClient) GetService(serviceName string) (*models.EnvService, error) {
	url := fmt.Sprintf("%s/services/%s", c.baseURL, serviceName)

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

	var getResp models.ClientResponse[models.EnvService]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", getResp.Code)
	}

	return &getResp.Data, nil
}

// DeleteService deletes a Service
func (c *ScheduleClient) DeleteService(serviceName string, deleteStorage bool) (bool, error) {
	url := fmt.Sprintf("%s/services/%s", c.baseURL, serviceName)
	if deleteStorage {
		url += "?deleteStorage=true"
	}

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

// UpdateServiceRequest represents the request body for updating a service
type UpdateServiceRequest struct {
	Replicas             *int32             `json:"replicas,omitempty"`
	EnvironmentVariables *map[string]string `json:"environment_variables,omitempty"`
}

// UpdateService updates a Service (replicas, image, env vars)
func (c *ScheduleClient) UpdateService(serviceName string, updateReq *UpdateServiceRequest) (*models.EnvService, error) {
	url := fmt.Sprintf("%s/services/%s", c.baseURL, serviceName)

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	httpReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var updateResp models.ClientResponse[models.EnvService]
	if err := json.Unmarshal(body, &updateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !updateResp.Success {
		return nil, fmt.Errorf("server returned error, code: %d", updateResp.Code)
	}

	return &updateResp.Data, nil
}

// ServiceListResponseData represents a single service item from controller's list endpoint
type ServiceListResponseData struct {
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	ServiceURL        string            `json:"service_url"`
	Replicas          int32             `json:"replicas"`
	AvailableReplicas int32             `json:"available_replicas"`
	Owner             string            `json:"owner"`
	EnvName           string            `json:"envname"`
	Version           string            `json:"version"`
	PVCName           string            `json:"pvc_name"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	EnvironmentVars   map[string]string `json:"environment_variables,omitempty"`
}

// ServiceListResponse represents the response structure from controller's list service endpoint
type ServiceListResponse struct {
	Success bool                      `json:"success"`
	Code    int                       `json:"code"`
	Data    []ServiceListResponseData `json:"data"`
}

// ListServices lists services, optionally filtered by environment name
func (c *ScheduleClient) ListServices(envName string) ([]*models.EnvService, error) {
	url := fmt.Sprintf("%s/services", c.baseURL)
	if envName != "" {
		url = fmt.Sprintf("%s/services?envName=%s", c.baseURL, envName)
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("list services: failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list services: failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("list services: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list services: request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var serviceListResp ServiceListResponse
	if err := json.Unmarshal(body, &serviceListResp); err != nil {
		return nil, fmt.Errorf("list services: failed to unmarshal response: %v", err)
	}

	if !serviceListResp.Success {
		return nil, fmt.Errorf("list services: server returned error, code: %d", serviceListResp.Code)
	}

	// Convert controller response to EnvService models
	services := make([]*models.EnvService, len(serviceListResp.Data))
	for i, svcData := range serviceListResp.Data {
		// Build Env object from EnvName and Version
		var env *backend.Env
		if svcData.EnvName != "" || svcData.Version != "" {
			env = &backend.Env{
				Name:    svcData.EnvName,
				Version: svcData.Version,
			}
		}

		services[i] = &models.EnvService{
			ID:                   svcData.ID,
			Env:                  env,
			Status:               svcData.Status,
			CreatedAt:            svcData.CreatedAt,
			UpdatedAt:            svcData.UpdatedAt,
			Replicas:             svcData.Replicas,
			AvailableReplicas:    svcData.AvailableReplicas,
			ServiceURL:           svcData.ServiceURL,
			Owner:                svcData.Owner,
			EnvironmentVariables: svcData.EnvironmentVars,
			PVCName:              svcData.PVCName,
		}
	}

	return services, nil
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

// PodListResponseData represents the data structure returned by controller's list pod endpoint
type PodListResponseData struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	TTL       string    `json:"ttl"`
	CreatedAt time.Time `json:"created_at"`
	EnvName   string    `json:"envname"`
	Version   string    `json:"version"`
	IP        string    `json:"ip"`
	Owner     string    `json:"owner"`
}

// PodListResponse represents the response structure from controller's list pod endpoint
type PodListResponse struct {
	Success bool                  `json:"success"`
	Code    int                   `json:"code"`
	Data    []PodListResponseData `json:"data"`
}

// ListEnvInstances implements EnvInstanceService interface
// Lists environment instances, optionally filtered by environment name
func (c *ScheduleClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/pods", c.baseURL)
	if envName != "" {
		url = fmt.Sprintf("%s/pods?envName=%s", c.baseURL, envName)
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list env instances: request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var podListResp PodListResponse
	if err := json.Unmarshal(body, &podListResp); err != nil {
		return nil, fmt.Errorf("list env instances: failed to unmarshal response: %v", err)
	}

	if !podListResp.Success {
		return nil, fmt.Errorf("list env instances: server returned error, code: %d", podListResp.Code)
	}

	// Convert PodListResponseData to EnvInstance
	instances := make([]*models.EnvInstance, len(podListResp.Data))
	for i, podData := range podListResp.Data {
		// Create a minimal Env object with Name and Version
		env := &backend.Env{
			Name:    podData.EnvName,
			Version: podData.Version,
		}

		// Format CreatedAt time
		createdAtStr := podData.CreatedAt.Format(time.RFC3339)
		nowStr := time.Now().Format(time.RFC3339)

		instances[i] = &models.EnvInstance{
			ID:        podData.ID,
			Env:       env,
			Status:    podData.Status,
			CreatedAt: createdAtStr,
			UpdatedAt: nowStr,
			IP:        podData.IP,
			TTL:       podData.TTL,
			Owner:     podData.Owner,
		}
	}

	return instances, nil
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
