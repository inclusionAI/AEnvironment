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
	"strings"
	"time"

	"controller/pkg/constants"
	"controller/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// AEnvServiceHandler handles Kubernetes Deployment + Service + PVC operations
type AEnvServiceHandler struct {
	clientset           kubernetes.Interface
	namespace           string
	serviceDomainSuffix string
}

// NewAEnvServiceHandler creates new ServiceHandler
func NewAEnvServiceHandler() (*AEnvServiceHandler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = "<your local path>"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %v", err)
		}
	}

	config.UserAgent = "aenv-controller"
	config.QPS = 1000
	config.Burst = 1000

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset, err is %v", err)
	}

	serviceHandler := &AEnvServiceHandler{
		clientset: clientset,
	}

	// Get namespace from pod template
	namespace := LoadNsFromPodTemplate(SingleContainerTemplate)
	serviceHandler.namespace = namespace

	// Get service domain suffix from environment variable, default to "svc.cluster.local"
	serviceDomainSuffix := os.Getenv("SERVICE_DOMAIN_SUFFIX")
	if serviceDomainSuffix == "" {
		serviceDomainSuffix = "svc.cluster.local"
	}
	serviceHandler.serviceDomainSuffix = serviceDomainSuffix

	klog.Infof("AEnv service handler is created, namespace is %s, serviceDomainSuffix is %s", serviceHandler.namespace, serviceHandler.serviceDomainSuffix)

	return serviceHandler, nil
}

// HttpServiceResponseData represents service response data
type HttpServiceResponseData struct {
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
	Env               map[string]string `json:"environment_variables,omitempty"`
}

type HttpServiceResponse struct {
	Success      bool                    `json:"success"`
	Code         int                     `json:"code"`
	ResponseData HttpServiceResponseData `json:"data"`
}

type HttpServiceListResponse struct {
	Success          bool                      `json:"success"`
	Code             int                       `json:"code"`
	ListResponseData []HttpServiceResponseData `json:"data"`
}

type HttpServiceDeleteResponse struct {
	Success      bool `json:"success"`
	Code         int  `json:"code"`
	ResponseData bool `json:"data"`
}

// ServeHTTP main routing method
func (h *AEnvServiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 || parts[1] != "services" {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	klog.Infof("access URL path %s, method %s, host %s", r.URL.Path, r.Method, r.Host)

	// Route handling
	switch {
	case r.Method == http.MethodPost && len(parts) == 2: // /services
		h.createService(w, r)
	case r.Method == http.MethodGet && len(parts) == 2: // /services
		h.listServices(w, r)
	case r.Method == http.MethodGet && len(parts) == 3: // /services/{serviceName}
		serviceName := parts[2]
		h.getService(serviceName, w, r)
	case r.Method == http.MethodPut && len(parts) == 3: // /services/{serviceName}
		serviceName := parts[2]
		h.updateService(serviceName, w, r)
	case r.Method == http.MethodDelete && len(parts) == 3: // /services/{serviceName}
		serviceName := parts[2]
		h.deleteService(serviceName, w, r)
	default:
		http.Error(w, "http method not allowed", http.StatusMethodNotAllowed)
	}
}

// createService creates a new service (Deployment + Service + PVC)
func (h *AEnvServiceHandler) createService(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	// Generate service name: use custom serviceName from DeployConfig if provided, otherwise auto-generate
	var serviceName string
	if customServiceName, ok := aenvHubEnv.DeployConfig["serviceName"]; ok {
		if customServiceNameStr, ok := customServiceName.(string); ok && customServiceNameStr != "" {
			serviceName = customServiceNameStr
			klog.Infof("Using custom service name: %s", serviceName)
		}
	}

	// If no custom serviceName provided, auto-generate using envName and random suffix
	if serviceName == "" {
		serviceName = fmt.Sprintf("%s-svc-%s", aenvHubEnv.Name, RandString(6))
		klog.Infof("Auto-generated service name: %s", serviceName)
	}

	// Get PVC name from deploy config, default to envName
	pvcName := aenvHubEnv.Name // Default PVC name equals envName
	if pvcNameValue, ok := aenvHubEnv.DeployConfig["pvcName"]; ok {
		if pvcNameStr, ok := pvcNameValue.(string); ok && pvcNameStr != "" {
			pvcName = pvcNameStr
		}
	}

	// Get mount path from deploy config, default to /home/admin/data
	mountPath := "/home/admin/data" // Default mount path
	if mountPathValue, ok := aenvHubEnv.DeployConfig["mountPath"]; ok {
		if mountPathStr, ok := mountPathValue.(string); ok && mountPathStr != "" {
			mountPath = mountPathStr
		}
	}

	// Get replicas from deploy config, default to 1
	replicas := int32(1)
	if replicasValue, ok := aenvHubEnv.DeployConfig["replicas"]; ok {
		if replicasInt, ok := replicasValue.(float64); ok {
			replicas = int32(replicasInt)
		} else if replicasInt32, ok := replicasValue.(int32); ok {
			replicas = replicasInt32
		}
	}

	// Check if PVC creation is enabled (default: false, only create when storageSize is specified)
	createPVC := false
	storageSize := ""
	if storageSizeValue, ok := aenvHubEnv.DeployConfig["storageSize"]; ok {
		if storageSizeStr, ok := storageSizeValue.(string); ok && storageSizeStr != "" {
			storageSize = storageSizeStr
			createPVC = true
		}
	}

	// If PVC creation is enabled, validate replicas must be 1
	if createPVC && replicas != 1 {
		http.Error(w, "When creating PVC (storageSize specified), replicas must be 1", http.StatusBadRequest)
		return
	}

	// Create or get existing PVC only if enabled
	var pvc *corev1.PersistentVolumeClaim
	if createPVC {
		var err error
		pvc, err = h.ensurePVC(ctx, pvcName, storageSize, &aenvHubEnv)
		if err != nil {
			handleK8sAPiError(w, err, "failed to ensure PVC")
			return
		}
		klog.Infof("PVC %s ensured", pvc.Name)
	} else {
		klog.Infof("PVC creation skipped (no storageSize specified)")
		pvcName = "" // Clear pvcName so deployment won't mount it
	}

	// Create Deployment
	deployment, err := h.createDeployment(ctx, serviceName, &aenvHubEnv, replicas, pvcName, mountPath)
	if err != nil {
		handleK8sAPiError(w, err, "failed to create deployment")
		return
	}
	klog.Infof("created deployment %s/%s successfully", h.namespace, deployment.Name)

	// Create Service
	service, err := h.createK8sService(ctx, serviceName, &aenvHubEnv)
	if err != nil {
		// Cleanup deployment if service creation fails
		_ = h.clientset.AppsV1().Deployments(h.namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{})
		handleK8sAPiError(w, err, "failed to create k8s service")
		return
	}
	klog.Infof("created k8s service %s/%s successfully", h.namespace, service.Name)

	// Build service URL with port
	serviceURL := fmt.Sprintf("%s.%s.%s:%d", service.Name, h.namespace, h.serviceDomainSuffix, service.Spec.Ports[0].Port)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	owner := ""
	if aenvHubEnv.DeployConfig["owner"] != nil {
		owner = aenvHubEnv.DeployConfig["owner"].(string)
	}

	res := &HttpServiceResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpServiceResponseData{
			ID:                serviceName,
			Status:            "Creating",
			ServiceURL:        serviceURL,
			Replicas:          replicas,
			AvailableReplicas: 0,
			Owner:             owner,
			EnvName:           aenvHubEnv.Name,
			Version:           aenvHubEnv.Version,
			PVCName:           pvcName,
			CreatedAt:         time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:         time.Now().Format("2006-01-02 15:04:05"),
		},
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// ensurePVC creates PVC if not exists, or returns existing one
func (h *AEnvServiceHandler) ensurePVC(ctx context.Context, pvcName string, storageSize string, env *model.AEnvHubEnv) (*corev1.PersistentVolumeClaim, error) {
	// Try to get existing PVC
	existingPVC, err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("PVC %s already exists, reusing it", pvcName)
		return existingPVC, nil
	}

	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Load PVC template and merge with environment config
	// storageClassName is now configured in values.yaml template, not passed as parameter
	pvc := LoadPVCTemplateFromYaml()
	pvc.Namespace = h.namespace
	MergePVC(pvc, pvcName, storageSize, "")

	// Add labels
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[constants.AENV_NAME] = env.Name
	pvc.Labels[constants.AENV_VERSION] = env.Version

	createdPVC, err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	klog.Infof("created PVC %s with size %s (storageClass from template)", pvcName, storageSize)
	return createdPVC, nil
}

// createDeployment creates a Deployment
func (h *AEnvServiceHandler) createDeployment(ctx context.Context, name string, env *model.AEnvHubEnv, replicas int32, pvcName string, mountPath string) (*appsv1.Deployment, error) {
	// Extract image from env artifacts
	image := ""
	for _, artifact := range env.Artifacts {
		if artifact.Type == "image" {
			image = artifact.Content
			break
		}
	}

	// Prepare labels
	labels := map[string]string{
		constants.AENV_NAME:    env.Name,
		constants.AENV_VERSION: env.Version,
	}
	if env.DeployConfig["owner"] != nil {
		labels[constants.AENV_OWNER] = env.DeployConfig["owner"].(string)
	}

	// Extract environment variables
	environs := make(map[string]string)
	if envVarsValue, ok := env.DeployConfig["environment_variables"]; ok {
		if envVarsMap, ok := envVarsValue.(map[string]interface{}); ok {
			for k, v := range envVarsMap {
				if vStr, ok := v.(string); ok {
					environs[k] = vStr
				}
			}
		}
	}

	// Extract resource configuration
	resources := &ResourceConfig{
		CPURequest:              getStringFromConfig(env.DeployConfig, "cpuRequest", "1"),
		CPULimit:                getStringFromConfig(env.DeployConfig, "cpuLimit", "1"),
		MemoryRequest:           getStringFromConfig(env.DeployConfig, "memoryRequest", "2Gi"),
		MemoryLimit:             getStringFromConfig(env.DeployConfig, "memoryLimit", "2Gi"),
		EphemeralStorageRequest: getStringFromConfig(env.DeployConfig, "ephemeralStorageRequest", "10Gi"),
		EphemeralStorageLimit:   getStringFromConfig(env.DeployConfig, "ephemeralStorageLimit", "10Gi"),
	}

	// Load Deployment template and merge with environment config
	deployment := LoadDeploymentTemplateFromYaml()
	deployment.Namespace = h.namespace
	MergeDeployment(deployment, name, replicas, labels, environs, image, pvcName, mountPath, resources)

	return h.clientset.AppsV1().Deployments(h.namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

// getStringFromConfig extracts a string value from DeployConfig with default fallback
func getStringFromConfig(config map[string]interface{}, key string, defaultValue string) string {
	if value, ok := config[key]; ok {
		if strValue, ok := value.(string); ok && strValue != "" {
			return strValue
		}
	}
	return defaultValue
}

// createK8sService creates a Kubernetes Service
func (h *AEnvServiceHandler) createK8sService(ctx context.Context, name string, env *model.AEnvHubEnv) (*corev1.Service, error) {
	// Default port 8080
	port := int32(8080)
	if portValue, ok := env.DeployConfig["port"]; ok {
		if portFloat, ok := portValue.(float64); ok {
			port = int32(portFloat)
		} else if portInt32, ok := portValue.(int32); ok {
			port = portInt32
		}
	}

	// Load Service template and merge with environment config
	service := LoadServiceTemplateFromYaml()
	service.Namespace = h.namespace
	MergeService(service, name, port)

	// Add labels
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}
	service.Labels[constants.AENV_NAME] = env.Name
	service.Labels[constants.AENV_VERSION] = env.Version

	return h.clientset.CoreV1().Services(h.namespace).Create(ctx, service, metav1.CreateOptions{})
}

// getService gets a service
func (h *AEnvServiceHandler) getService(serviceName string, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deployment, err := h.clientset.AppsV1().Deployments(h.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		handleK8sAPiError(w, err, "failed to get deployment")
		return
	}

	service, err := h.clientset.CoreV1().Services(h.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		handleK8sAPiError(w, err, "failed to get service")
		return
	}

	// Build service URL with port
	serviceURL := ""
	if len(service.Spec.Ports) > 0 {
		serviceURL = fmt.Sprintf("%s.%s.%s:%d", service.Name, h.namespace, h.serviceDomainSuffix, service.Spec.Ports[0].Port)
	}

	status := "Running"
	if deployment.Status.AvailableReplicas == 0 {
		status = "Creating"
	} else if deployment.Status.AvailableReplicas < *deployment.Spec.Replicas {
		status = "Updating"
	}

	// Extract PVC name from deployment spec
	pvcName := ""
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName = volume.PersistentVolumeClaim.ClaimName
			break
		}
	}

	// Extract environment variables from first container
	envVars := make(map[string]string)
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
			envVars[env.Name] = env.Value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	res := &HttpServiceResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpServiceResponseData{
			ID:                serviceName,
			Status:            status,
			ServiceURL:        serviceURL,
			Replicas:          *deployment.Spec.Replicas,
			AvailableReplicas: deployment.Status.AvailableReplicas,
			Owner:             deployment.Labels[constants.AENV_OWNER],
			EnvName:           deployment.Labels[constants.AENV_NAME],
			Version:           deployment.Labels[constants.AENV_VERSION],
			PVCName:           pvcName,
			CreatedAt:         deployment.CreationTimestamp.Format("2006-01-02 15:04:05"),
			UpdatedAt:         time.Now().Format("2006-01-02 15:04:05"),
			Env:               envVars,
		},
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// deleteService deletes a service (Deployment + Service, optionally delete PVC/storage)
func (h *AEnvServiceHandler) deleteService(serviceName string, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if deleteStorage query parameter is set
	deleteStorage := r.URL.Query().Get("deleteStorage") == "true"

	// Get deployment first to extract PVC name before deletion
	var pvcName string
	if deleteStorage {
		deployment, err := h.clientset.AppsV1().Deployments(h.namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			handleK8sAPiError(w, err, "failed to get deployment")
			return
		}
		if deployment != nil {
			// Extract PVC name from deployment spec
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil {
					pvcName = volume.PersistentVolumeClaim.ClaimName
					break
				}
			}
		}
	}

	// Delete Deployment
	err := h.clientset.AppsV1().Deployments(h.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		handleK8sAPiError(w, err, "failed to delete deployment")
		return
	}

	// Delete Service
	err = h.clientset.CoreV1().Services(h.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		handleK8sAPiError(w, err, "failed to delete service")
		return
	}

	// Delete PVC if requested and PVC name was found
	if deleteStorage && pvcName != "" {
		err = h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			handleK8sAPiError(w, err, "failed to delete PVC")
			return
		}
		klog.Infof("deleted PVC %s/%s successfully", h.namespace, pvcName)
	}

	klog.Infof("deleted service %s/%s successfully", h.namespace, serviceName)

	w.Header().Set("Content-Type", "application/json")
	res := &HttpServiceDeleteResponse{
		Success:      true,
		Code:         0,
		ResponseData: true,
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// updateService updates a service (replicas, image, env vars)
func (h *AEnvServiceHandler) updateService(serviceName string, w http.ResponseWriter, r *http.Request) {
	var updateReq struct {
		Replicas             *int32             `json:"replicas,omitempty"`
		Image                *string            `json:"image,omitempty"`
		EnvironmentVariables *map[string]string `json:"environment_variables,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			klog.Errorf("failed to close request body: %v", closeErr)
		}
	}()

	ctx := r.Context()

	deployment, err := h.clientset.AppsV1().Deployments(h.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		handleK8sAPiError(w, err, "failed to get deployment")
		return
	}

	// Update replicas
	if updateReq.Replicas != nil {
		deployment.Spec.Replicas = updateReq.Replicas
	}

	// Update image
	if updateReq.Image != nil && *updateReq.Image != "" {
		for i := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Image = *updateReq.Image
		}
	}

	// Update environment variables
	if updateReq.EnvironmentVariables != nil {
		for i := range deployment.Spec.Template.Spec.Containers {
			container := &deployment.Spec.Template.Spec.Containers[i]
			for key, value := range *updateReq.EnvironmentVariables {
				found := false
				for j := range container.Env {
					if container.Env[j].Name == key {
						container.Env[j].Value = value
						found = true
						break
					}
				}
				if !found {
					container.Env = append(container.Env, corev1.EnvVar{
						Name:  key,
						Value: value,
					})
				}
			}
		}
	}

	updatedDeployment, err := h.clientset.AppsV1().Deployments(h.namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		handleK8sAPiError(w, err, "failed to update deployment")
		return
	}

	klog.Infof("updated deployment %s/%s successfully", h.namespace, serviceName)

	service, _ := h.clientset.CoreV1().Services(h.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	serviceURL := ""
	if service != nil && len(service.Spec.Ports) > 0 {
		serviceURL = fmt.Sprintf("%s.%s.%s:%d", service.Name, h.namespace, h.serviceDomainSuffix, service.Spec.Ports[0].Port)
	}

	w.Header().Set("Content-Type", "application/json")
	res := &HttpServiceResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpServiceResponseData{
			ID:                serviceName,
			Status:            "Updating",
			ServiceURL:        serviceURL,
			Replicas:          *updatedDeployment.Spec.Replicas,
			AvailableReplicas: updatedDeployment.Status.AvailableReplicas,
			Owner:             updatedDeployment.Labels[constants.AENV_OWNER],
			EnvName:           updatedDeployment.Labels[constants.AENV_NAME],
			Version:           updatedDeployment.Labels[constants.AENV_VERSION],
			UpdatedAt:         time.Now().Format("2006-01-02 15:04:05"),
		},
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// listServices lists all services
func (h *AEnvServiceHandler) listServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	envName := r.URL.Query().Get("envName")

	// List deployments
	listOptions := metav1.ListOptions{}
	if envName != "" {
		listOptions.LabelSelector = fmt.Sprintf("%s=%s", constants.AENV_NAME, envName)
	}

	deployments, err := h.clientset.AppsV1().Deployments(h.namespace).List(ctx, listOptions)
	if err != nil {
		handleK8sAPiError(w, err, "failed to list deployments")
		return
	}

	responseData := make([]HttpServiceResponseData, 0, len(deployments.Items))
	for _, deployment := range deployments.Items {
		status := "Running"
		if deployment.Status.AvailableReplicas == 0 {
			status = "Creating"
		} else if deployment.Status.AvailableReplicas < *deployment.Spec.Replicas {
			status = "Updating"
		}

		// Try to get service
		service, _ := h.clientset.CoreV1().Services(h.namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		serviceURL := ""
		if service != nil && len(service.Spec.Ports) > 0 {
			serviceURL = fmt.Sprintf("%s.%s.%s:%d", service.Name, h.namespace, h.serviceDomainSuffix, service.Spec.Ports[0].Port)
		}

		// Extract PVC name from deployment spec
		pvcName := ""
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName = volume.PersistentVolumeClaim.ClaimName
				break
			}
		}

		// Extract environment variables from first container
		envVars := make(map[string]string)
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
				envVars[env.Name] = env.Value
			}
		}

		// Get EnvName and Version from labels
		// If labels don't exist (for old deployments), try to extract from deployment name
		envNameFromLabel := deployment.Labels[constants.AENV_NAME]
		versionFromLabel := deployment.Labels[constants.AENV_VERSION]

		// Fallback: extract from deployment name for backward compatibility
		// Deployment name format: {envName}-svc-{random}
		if envNameFromLabel == "" {
			// Try to extract from deployment name
			// Remove "-svc-{random}" suffix to get envName
			if idx := strings.Index(deployment.Name, "-svc-"); idx > 0 {
				envNameFromLabel = deployment.Name[:idx]
			}
		}

		responseData = append(responseData, HttpServiceResponseData{
			ID:                deployment.Name,
			Status:            status,
			ServiceURL:        serviceURL,
			Replicas:          *deployment.Spec.Replicas,
			AvailableReplicas: deployment.Status.AvailableReplicas,
			Owner:             deployment.Labels[constants.AENV_OWNER],
			EnvName:           envNameFromLabel,
			Version:           versionFromLabel,
			PVCName:           pvcName,
			CreatedAt:         deployment.CreationTimestamp.Format("2006-01-02 15:04:05"),
			UpdatedAt:         time.Now().Format("2006-01-02 15:04:05"),
			Env:               envVars,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	res := &HttpServiceListResponse{
		Success:          true,
		Code:             0,
		ListResponseData: responseData,
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}
