package service

import (
	"api-service/models"
	"bytes"
	"encoding/json"
	backend "envhub/models"
	"fmt"
	"io"
	"net/http"
	"time"
)

const AEnvOpenAPIPrefix = "aenvironment/"
const AEnvOpenAPIInstance = AEnvOpenAPIPrefix + "instance"

// EnvInstanceService defines sandbox crud interfaces
type EnvInstanceService interface {
	GetEnvInstance(id string) (*models.EnvInstance, error)
	CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error)
	DeleteEnvInstance(id string) error
	ListEnvInstances(envName string) ([]*models.EnvInstance, error)
	Warmup(req *backend.Env) error
	Cleanup() error
}

type EnvInstanceClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewEnvInstanceClient(baseURL string) *EnvInstanceClient {
	return &EnvInstanceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateEnvInstance creates a new environment instance based on the provided environment configuration.
//
// Parameters:
//   - req (*backend.Env): The environment configuration used to create the instance.
//
// Returns:
//   - *models.EnvInstance: The created environment instance if successful.
//   - error: An error if the request fails, including HTTP errors, JSON parsing errors, or service-reported errors.
func (c *EnvInstanceClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, AEnvOpenAPIInstance)

	jsonData, err := req.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("create env instance: failed to marshal request: %v", err)
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create env instance: failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("create env instance: failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("create env instance: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create env instance: request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var createResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("create env instance: failed to unmarshal response: %v", err)
	}

	if !createResp.Success {
		return nil, fmt.Errorf("create env instance: server returned error, code: %d", createResp.Code)
	}

	return &createResp.Data, nil
}

// GetEnvInstance retrieves an existing environment instance by its ID.
//
// Parameters:
//   - id (string): The unique identifier of the environment instance.
//
// Returns:
//   - *models.EnvInstance: The requested environment instance if found.
//   - error: An error if the instance does not exist, HTTP request fails, or response is invalid.
func (c *EnvInstanceClient) GetEnvInstance(id string) (*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, AEnvOpenAPIInstance, id)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("get env instance %s: failed to create request: %v", id, err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get env instance %s: failed to send request: %v", id, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("get env instance %s: failed to read response body: %v", id, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get env instance %s: request failed with status %d: %s", id, resp.StatusCode, truncateBody(body))
	}

	var getResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("get env instance %s: failed to unmarshal response: %v", id, err)
	}

	if !getResp.Success {
		return nil, fmt.Errorf("get env instance %s: server returned error, code: %d", id, getResp.Code)
	}

	return &getResp.Data, nil
}

// DeleteEnvInstance deletes an environment instance by its ID.
//
// Parameters:
//   - id (string): The unique identifier of the environment instance to delete.
//
// Returns:
//   - error: nil if deletion is successful; otherwise, an error indicating failure in request, response, or service logic.
func (c *EnvInstanceClient) DeleteEnvInstance(id string) error {
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, AEnvOpenAPIInstance, id)

	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("delete env instance %s: failed to create request: %v", id, err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("delete env instance %s: failed to send request: %v", id, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("delete env instance %s: failed to read response body: %v", id, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete env instance %s: request failed with status %d: %s", id, resp.StatusCode, truncateBody(body))
	}

	var deleteResp models.ClientResponse[bool]
	if err := json.Unmarshal(body, &deleteResp); err != nil {
		return fmt.Errorf("delete env instance %s: failed to unmarshal response: %v", id, err)
	}

	if !deleteResp.Success {
		return fmt.Errorf("delete env instance %s: server returned error, code: %d", id, deleteResp.Code)
	}

	return nil
}

// ListEnvInstances lists environment instances filtered by environment name.
//
// Parameters:
//   - envName (string): The name of the environment to filter instances by. Use empty string to list all.
//
// Returns:
//   - []*models.EnvInstance: A slice of matching environment instances.
//   - error: An error if the request fails, response is invalid, or service reports an error.
func (c *EnvInstanceClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	url := fmt.Sprintf("%s/%s?envName=%s", c.baseURL, AEnvOpenAPIInstance, envName)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("list env instances: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list env instances: request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var getResp models.ClientResponse[[]*models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("list env instances: failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return nil, fmt.Errorf("list env instances: server returned error, code: %d", getResp.Code)
	}

	return getResp.Data, nil
}

// Warmup triggers a warm-up process for the environment to prepare resources in advance.
//
// Parameters:
//   - req (*backend.Env): The environment configuration used for warm-up preparation.
//
// Returns:
//   - error: nil if warm-up is successful; otherwise, an error describing the failure.
func (c *EnvInstanceClient) Warmup(req *backend.Env) error {
	url := fmt.Sprintf("%s/%s/action/warmup", c.baseURL, AEnvOpenAPIInstance)

	httpReq, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("warmup env: failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("warmup env: failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("warmup env: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("warmup env: request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var getResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return fmt.Errorf("warmup env: failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return fmt.Errorf("warmup env: server returned error, code: %d", getResp.Code)
	}

	return nil
}

// Cleanup performs a cleanup operation to release unused environment resources.
//
// Parameters:
//   - None
//
// Returns:
//   - error: nil if cleanup is successful; otherwise, an error indicating failure.
func (c *EnvInstanceClient) Cleanup() error {
	url := fmt.Sprintf("%s/%s/action/cleanup", c.baseURL, AEnvOpenAPIInstance)

	httpReq, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("cleanup env: failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cleanup env: failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cleanup env: failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cleanup env: request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var getResp models.ClientResponse[models.EnvInstance]
	if err := json.Unmarshal(body, &getResp); err != nil {
		return fmt.Errorf("cleanup env: failed to unmarshal response: %v", err)
	}

	if !getResp.Success {
		return fmt.Errorf("cleanup env: server returned error, code: %d", getResp.Code)
	}

	return nil
}

// truncateBody truncate body for memory protection
func truncateBody(body []byte) string {
	const maxLen = 500
	if len(body) > maxLen {
		return string(body[:maxLen]) + "...(truncated)"
	}
	return string(body)
}
