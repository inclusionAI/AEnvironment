package service

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"api-service/models"
	"api-service/service/faas_model"
	backend "envhub/models"
)

var _ EnvInstanceService = (*FaaSClient)(nil)

type FaaSClient struct {
	client *faas_model.HTTPClient
}

func NewFaaSClient(endpoint string) *FaaSClient {
	client := faas_model.NewHTTPClient(endpoint)
	return &FaaSClient{client}
}

// CreateEnvInstance creates an environment instance by triggering the create-env function
func (c *FaaSClient) CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error) {
	functionName := fmt.Sprintf("%s-%s", req.Name, req.Version)
	// use datasource as runtime name
	dynamicRuntimeName := ""
	if name, ok := req.DeployConfig["dataSource"]; ok {
		s, ok := name.(string)
		if !ok {
			return nil, fmt.Errorf("value for 'dataSource' in DeployConfig must be a string, but got %T", name)
		}
		dynamicRuntimeName = s
	}
	//if err := c.PrepareFunction(functionName, req); err != nil {
	//	return nil, fmt.Errorf("prepare function failed: %v", err.Error())
	//}
	// Synchronously call the function
	instanceId, err := c.CreateInstanceByFunction(functionName, dynamicRuntimeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create env instance %s: %v", functionName, err)
	}
	return models.NewEnvInstance(instanceId, req, ""), nil
}

func (c *FaaSClient) PrepareFunction(functionName string, req *backend.Env) error {
	runtimeName := fmt.Sprintf("runtime-%s", functionName)
	// Create runtime
	runtime, err := c.GetRuntime(runtimeName)
	if runtime == nil || err != nil {
		runtimeReq := faas_model.RuntimeCreateOrUpdateRequest{
			Name:        runtimeName,
			Description: req.Description,
			Content: &faas_model.RuntimeContent{
				OssURL: req.GetImage(),
			},
			Labels: map[string]string{
				"huse.alipay.com/runsc-oci": "true",
			},
		}
		if err := c.CreateRuntime(&runtimeReq); err != nil {
			return fmt.Errorf("failed to create runtime: %v", err.Error())
		}
	} else if runtime.Status != string(faas_model.RuntimeStatusActive) {
		return fmt.Errorf("runtime %s is not active: %v", runtime.Name, runtime.Status)
	}
	// Create function
	function, err := c.GetFunction(functionName)
	if function == nil || err != nil {
		memoryQuntity, err := resource.ParseQuantity(req.GetMemory())
		if err != nil {
			return fmt.Errorf("failed to parse memory value: %v", err.Error())
		}
		functionReq := faas_model.FunctionCreateOrUpdateRequest{
			Name:        functionName,
			PackageType: "zip",
			// FIXME: swe hard code here, should use specified env source code as function code
			Code: &faas_model.FunctionCode{
				OSSURL: "",
			},
			Runtime: runtimeName,
			Labels: map[string]string{
				faas_model.LabelStatefulFunction: "true",
				//faas-api-service receiver uses strconv.Atoi, using int here to prevent overflow
				"custom.hcsfaas.hcs.io/idle-timeout": strconv.FormatInt(math.MaxInt32, 10),
			},
			Description: req.Description,
			Memory:      memoryQuntity.ScaledValue(resource.Mega),
			Timeout:     3600,
		}
		if err := c.CreateFunction(&functionReq); err != nil {
			return fmt.Errorf("failed to create function: %v", err.Error())
		}
	}
	return nil
}

func (c *FaaSClient) CreateInstanceByFunction(name string, dynamicRuntimeName string) (string, error) {
	f, err := c.GetFunction(name)
	if err != nil {
		return "", err
	}

	instanceId, err := c.InitializeFunction(f.Name, dynamicRuntimeName, faas_model.FunctionInvocationTypeSync, []byte("{}"))
	if err != nil {
		return "", fmt.Errorf("failed to create functions instance from faas server: %v", err.Error())
	}
	return instanceId, nil
}

// GetEnvInstance gets the details of the specified environment instance
func (c *FaaSClient) GetEnvInstance(id string) (*models.EnvInstance, error) {
	// Reuse HCSFaaSClient's GetInstance
	instance, err := c.GetInstance(id)
	if err != nil {
		return nil, fmt.Errorf("get instance %s failed: %w", id, err)
	}

	// Map model.Instance -> models.EnvInstance
	envInst := &models.EnvInstance{
		ID:  instance.InstanceID,
		IP:  instance.IP,
		TTL: "", // No TTL field source available yet, can be added later
		// CreatedAt / UpdatedAt use current time or default values (should actually be returned by backend)
		CreatedAt: time.UnixMilli(instance.CreateTimestamp).Format(time.RFC3339),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Status:    convertStatus(instance.Status),
		// Env field cannot be directly obtained from Instance, needs to rely on Create return or additional queries
		// Can only be empty here, recommend maintaining through Create/CreateFromRecord
		Env: nil,
	}

	return envInst, nil
}

// DeleteEnvInstance deletes the specified environment instance
func (c *FaaSClient) DeleteEnvInstance(id string) error {
	return c.DeleteInstance(id) // Direct proxy
}

// ListEnvInstances lists all environment instances, supporting filtering by env name
func (c *FaaSClient) ListEnvInstances(envName string) ([]*models.EnvInstance, error) {
	labels := make(map[string]string)
	if envName != "" {
		labels["env"] = envName
	}

	resp, err := c.ListInstances(labels)
	if err != nil {
		return nil, fmt.Errorf("list instances failed: %w", err)
	}

	var result []*models.EnvInstance
	for _, inst := range resp.Instances {
		result = append(result, &models.EnvInstance{
			ID:        inst.InstanceID,
			IP:        inst.IP,
			Status:    convertStatus(inst.Status),
			CreatedAt: time.UnixMilli(inst.CreateTimestamp).Format(time.RFC3339), // Could consider constructing from CreateTimestamp
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
			TTL:       "",
			Env:       nil, // Cannot obtain full Env information from Instance
		})
	}

	return result, nil
}

// Warmup warms up the specified environment: polling PrepareFunction calls until success or timeout
func (c *FaaSClient) Warmup(req *backend.Env) error {
	errCh := c.WarmupAsyncChan(req)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		} else {
			return nil
		}
	case <-time.After(300 * time.Second):
		return fmt.Errorf("timed out waiting for env instance to become ready")
	}
}

// WarmupAsyncChan async warmup, returns result channel
func (c *FaaSClient) WarmupAsyncChan(req *backend.Env) <-chan error {
	resultCh := make(chan error, 1) // Buffer of 1 to prevent goroutine leak

	go func() {
		defer close(resultCh)

		const (
			timeout  = 300 * time.Second
			interval = 10 * time.Second
		)

		deadline := time.Now().Add(timeout)
		functionName := fmt.Sprintf("%s-%s", req.Name, req.Version)

		var lastErr error
		for time.Now().Before(deadline) {
			lastErr = c.PrepareFunction(functionName, req)
			if lastErr == nil {
				return // Success, don't send error
			}

			fmt.Printf("Warmup retry: %v\n", lastErr)
			time.Sleep(interval)
		}

		// Timeout, send error
		resultCh <- fmt.Errorf("warmup timeout: function %s not ready after %v", functionName, timeout)
	}()

	return resultCh
}

func (c *FaaSClient) Cleanup() error {
	return fmt.Errorf("cleanup not implemented in faas")
}

// --- Newly added local method implementations ---

func (c *FaaSClient) CreateFunction(in *faas_model.FunctionCreateOrUpdateRequest) error {
	uri := "/hapis/faas.hcs.io/v1/functions/"

	funcResp := &faas_model.APIResponse{}
	err := c.client.Post(uri).Body(*in).Do().Into(funcResp)
	if err != nil {
		return err
	}

	return nil
}

func (c *FaaSClient) GetFunction(name string) (*faas_model.Function, error) {
	uri := "/hapis/faas.hcs.io/v1/functions/" + name

	funcResp := &faas_model.APIResponse{}
	err := c.client.Get(uri).Do().Into(&funcResp)
	if err != nil {
		return nil, fmt.Errorf("get function failed with err: %s", err)
	}

	if !funcResp.Success {
		return nil, fmt.Errorf("failed to get function from faas server with message: %s", funcResp.ErrorMessage)
	}

	data, ok := funcResp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type for Function")
	}

	// Convert map to Function struct
	function := &faas_model.Function{}
	if name, ok := data["name"].(string); ok {
		function.Name = name
	}
	if packageType, ok := data["packageType"].(string); ok {
		function.PackageType = packageType
	}
	if description, ok := data["description"].(string); ok {
		function.Description = description
	}
	if runtime, ok := data["runtime"].(string); ok {
		function.Runtime = runtime
	}
	if memory, ok := data["memory"].(float64); ok {
		function.Memory = int64(memory)
	}
	if timeout, ok := data["timeout"].(float64); ok {
		function.Timeout = int64(timeout)
	}

	return function, nil
}

func (c *FaaSClient) CreateRuntime(in *faas_model.RuntimeCreateOrUpdateRequest) error {
	uri := "/hapis/faas.hcs.io/v1/runtimes/"

	runtimeResp := &faas_model.APIResponse{}
	err := c.client.Post(uri).Body(*in).Do().Into(runtimeResp)
	if err != nil {
		return err
	}

	return nil
}

func (c *FaaSClient) GetRuntime(name string) (*faas_model.Runtime, error) {
	uri := "/hapis/faas.hcs.io/v1/runtimes/" + name

	runtimeResp := &faas_model.APIResponse{}
	err := c.client.Get(uri).Do().Into(&runtimeResp)
	if err != nil {
		return nil, fmt.Errorf("get runtime failed with err: %s", err)
	}

	if !runtimeResp.Success {
		return nil, fmt.Errorf("failed to get runtime from faas server with message: %s", runtimeResp.ErrorMessage)
	}

	data, ok := runtimeResp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type for Runtime")
	}

	// Convert map to Runtime struct
	runtime := &faas_model.Runtime{}
	if name, ok := data["name"].(string); ok {
		runtime.Name = name
	}
	if description, ok := data["description"].(string); ok {
		runtime.Description = description
	}
	if status, ok := data["status"].(string); ok {
		runtime.Status = status
	}

	return runtime, nil
}

func (c *FaaSClient) InitializeFunction(name string, dynamicRuntimeName string, invocationType string, invocationBody []byte) (string, error) {
	uri := fmt.Sprintf("/hapis/faas.hcs.io/v1/functions/%s/initialize", name)

	f, err := c.GetFunction(name)
	if err != nil {
		return "", err
	}

	if invocationType == faas_model.FunctionInvocationTypeAsync {
		invocationType = faas_model.FunctionInvocationTypeAsync
	} else {
		invocationType = faas_model.FunctionInvocationTypeSync
	}

	req := c.client.Post(uri).BodyData(invocationBody).Timeout(time.Duration(f.Timeout)*time.Second).Query("invocationType", invocationType)

	// If dynamicRuntimeName is provided, add it to the query parameters
	if dynamicRuntimeName != "" {
		req = req.Query("dynamicRuntimeName", dynamicRuntimeName)
	}

	resp, err := req.Do().Response()
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to initialize function from faas server with status code %d", resp.StatusCode)
	}

	instanceId := resp.Header.Get(faas_model.HttpHeaderInstanceID)
	return instanceId, nil
}

func (c *FaaSClient) ListInstances(labels map[string]string) (*faas_model.InstanceListResp, error) {
	uri := "/hapis/faas.hcs.io/v1/instances"

	req := &faas_model.InstanceListRequest{Labels: labels}
	resp := &faas_model.APIResponse{
		Data: &faas_model.InstanceListResp{},
	}
	err := c.client.Post(uri).Body(*req).Do().Into(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to list instances: %s", resp.ErrorMessage)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type for InstanceListResp")
	}

	// Convert map to InstanceListResp struct
	instances := []*faas_model.Instance{}
	if insts, ok := data["instances"].([]interface{}); ok {
		for _, inst := range insts {
			if instMap, ok := inst.(map[string]interface{}); ok {
				instance := &faas_model.Instance{}
				if instanceID, ok := instMap["instanceID"].(string); ok {
					instance.InstanceID = instanceID
				}
				if ip, ok := instMap["ip"].(string); ok {
					instance.IP = ip
				}
				if status, ok := instMap["status"].(string); ok {
					instance.Status = faas_model.InstanceStatus(status)
				}
				instances = append(instances, instance)
			}
		}
	}

	return &faas_model.InstanceListResp{Instances: instances}, nil
}

func (c *FaaSClient) GetInstance(name string) (*faas_model.Instance, error) {
	uri := fmt.Sprintf("/hapis/faas.hcs.io/v1/instances/%s", name)

	resp := &faas_model.APIResponse{
		Data: &faas_model.Instance{},
	}
	err := c.client.Get(uri).Do().Into(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance %s: %w", name, err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get instance %s: %s", name, resp.ErrorMessage)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type for Instance")
	}

	// Convert map to Instance struct
	instance := &faas_model.Instance{}
	if instanceID, ok := data["instanceID"].(string); ok {
		instance.InstanceID = instanceID
	}
	if ip, ok := data["ip"].(string); ok {
		instance.IP = ip
	}
	if status, ok := data["status"].(string); ok {
		instance.Status = faas_model.InstanceStatus(status)
	}
	if createTimestamp, ok := data["createTimestamp"].(float64); ok {
		instance.CreateTimestamp = int64(createTimestamp)
	}

	return instance, nil
}

func (c *FaaSClient) DeleteInstance(name string) error {
	uri := fmt.Sprintf("/hapis/faas.hcs.io/v1/instances/%s", name)

	resp := &faas_model.APIResponse{}
	err := c.client.Delete(uri).Do().Into(resp)
	if err != nil {
		return fmt.Errorf("failed to delete instance %s: %w", name, err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to delete instance %s: %s", name, resp.ErrorMessage)
	}

	return nil
}

// --- Utility functions ---

// convertStatus converts model.InstanceStatus to models.EnvInstanceStatus.String()
func convertStatus(s faas_model.InstanceStatus) string {
	switch s {
	case "Pending":
		return models.EnvInstanceStatusPending.String()
	case "Creating":
		return models.EnvInstanceStatusCreating.String()
	case "Running":
		return models.EnvInstanceStatusRunning.String()
	case "Failed":
		return models.EnvInstanceStatusFailed.String()
	case "Terminated":
		return models.EnvInstanceStatusTerminated.String()
	default:
		return models.EnvInstanceStatusRunning.String()
	}
}
