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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"controller/pkg/constants"

	"k8s.io/apimachinery/pkg/api/resource"

	"controller/pkg/model"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// AEnvPodHandler handles Kubernetes Pod CRUD operations
// Note: AEnvPodHandler only handles one namespace, which is read from pod template.
type AEnvPodHandler struct {
	clientset kubernetes.Interface
	podCache  *AEnvPodCache
	namespace string
}

// NewAEnvPodHandler creates new PodHandler
func NewAEnvPodHandler() (*AEnvPodHandler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			// For local testing
			kubeconfig = "<your local path>"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %v", err)
		}
	}

	// Set useragent
	config.UserAgent = "aenv-controller"
	config.QPS = 1000
	config.Burst = 1000

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset, err is %v", err)
	}

	podHandler := &AEnvPodHandler{
		clientset: clientset,
	}

	// Get namespace
	namespace := LoadNsFromPodTemplate(SingleContainerTemplate)
	podHandler.namespace = namespace

	// Initialize Pod cache for namespace
	podCache := NewAEnvPodCache(clientset, namespace)
	podHandler.podCache = podCache

	klog.Infof("AEnv pod handler is created, namespace is %s", podHandler.namespace)

	return podHandler, nil
}

// ServeHTTP main routing method
func (h *AEnvPodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 || parts[1] != "pods" {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	klog.Infof("access URL path %s, method %s, host %s", r.URL.Path, r.Method, r.Host)

	// Route handling
	switch {
	case r.Method == http.MethodPost && len(parts) == 2: // /pods
		h.createPod(w, r)
	case r.Method == http.MethodGet && len(parts) == 2: // /pods/
		h.listPod(w, r)
	case r.Method == http.MethodGet && len(parts) == 3: // /pods/{podName}
		podName := parts[2]
		h.getPod(podName, w, r)
	case r.Method == http.MethodDelete && len(parts) == 3: // /pods/{podName}
		podName := parts[2]
		h.deletePod(podName, w, r)
	default:
		http.Error(w, "http method not allowed", http.StatusMethodNotAllowed)
	}
}

// createPod creates new Pod
/**
{
    "name": "envName",
    "version": "0.0.1",
    "tags": [
        "swe",
        "python",
        "linux"
    ],
    "status": "Ready",
    "codeUrl": "oss://xxx" # Code file
    "artifacts": [
        {
            "type": "image",
            "content": "http://docker.alibaba-inc.com/aenv/xxx:0.0.1"
        },
        {
            "type": "whl",
            "content": "http://pypi.artifact.antfin.com/xxx/envName.whl"
        }
    ]
    "buildConfig": {
        "dockerfile": "./Dockerfile"
    },
    "testConfig": {
        "script": "pytest xxx"
    },
    "deployConfig": {
        "cpu": "1C",
        "memory": "2G",
        "os": "linux"
    },
}
*/
/**
response is:
{
  "success": true,
  "code": 0,
  "data": {
      "id": "leopard-linux-v1-7q8y9v0a1b2c",
      "status": "Pending",
      "ip": ""
  }
}
*/

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

/*
*

	{
	  "id": "leopard-linux-v1-7q8y9v0a1b2c",
	  "createdAt": "2025-10-10 11:11:11",
	  "status": "Running",
	},
*/
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

func (h *AEnvPodHandler) createPod(w http.ResponseWriter, r *http.Request) {
	var aenvHubEnv model.AEnvHubEnv
	if err := json.NewDecoder(r.Body).Decode(&aenvHubEnv); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get podTemplate type, default to "singleContainer"
	templateType := SingleContainerTemplate
	if podTemplateValue, ok := aenvHubEnv.DeployConfig["podTemplate"]; ok {
		if podTemplateStr, ok := podTemplateValue.(string); ok && podTemplateStr != "" {
			templateType = podTemplateStr
		}
	}

	pod := LoadPodTemplateFromYaml(templateType)

	klog.Infof("received env deploy config: %v", aenvHubEnv.DeployConfig)
	// Merge pod with image from Env
	MergePodImage(pod, &aenvHubEnv)

	klog.Infof("rendered pod template: %v", pod.Spec.Containers[0].Env)

	// Generate name
	pod.Name = fmt.Sprintf("%s-%s", aenvHubEnv.Name, RandString(6))
	// Set pods TTL by label
	if aenvHubEnv.DeployConfig["ttl"] != nil {
		labels := pod.Labels
		if labels == nil {
			labels = make(map[string]string)
			pod.Labels = labels
		}
		ttlValue := aenvHubEnv.DeployConfig["ttl"].(string)
		labels[constants.AENV_TTL] = ttlValue
		klog.Infof("add aenv-ttl label with value:%v for pod:%s", ttlValue, pod.Name)
	}

	createdPod, err := h.clientset.CoreV1().Pods(h.namespace).Create(r.Context(), pod, metav1.CreateOptions{})
	if err != nil {
		handleK8sAPiError(w, err, "failed to create pod")
		return
	}
	klog.Infof("created pod %s/%s successfully", h.namespace, createdPod.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	res := &HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     createdPod.Name,
			Status: string(createdPod.Status.Phase),
			IP:     createdPod.Status.PodIP,
		},
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// getPod gets single Pod (from cache)
/*
{
  "success": true,
  "code": 0,
  "data": {
    "id": "leopard-linux-v1-7q8y9v0a1b2c",
    "status": "Running",
    "ip": "xxx"
  }
}
*/
func (h *AEnvPodHandler) getPod(podName string, w http.ResponseWriter, r *http.Request) {
	if podName == "" {
		http.Error(w, "missing pod name", http.StatusBadRequest)
		return
	}

	// Get Pod from cache
	pod, err := h.podCache.GetPod(h.namespace, podName)
	if err != nil {
		// Fall back to K8s API
		pod, err = h.clientset.CoreV1().Pods(h.namespace).Get(r.Context(), podName, metav1.GetOptions{})
		if err != nil {
			handleK8sAPiError(w, err, "failed to get pod")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	res := &HttpResponse{
		Success: true,
		Code:    0,
		ResponseData: HttpResponseData{
			ID:     pod.Name,
			TTL:    pod.Labels[constants.AENV_TTL],
			Status: string(pod.Status.Phase),
			IP:     pod.Status.PodIP,
		},
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

/*
*

	{
	  "success": true,
	  "code": 0,
	  "data": [
	      {
	        "id": "leopard-linux-v1-7q8y9v0a1b2c",
	        "createdAt": "2025-10-10 11:11:11",
	        "status": "Running",
	      },
	      {
	        "id": "leopard-linux-v1-7q8y9v0a1b2c",
	        "createdAt": "2025-10-10 11:11:11",
	        "status": "Running",
	      }
	  ]
	}
*/
func (h *AEnvPodHandler) listPod(w http.ResponseWriter, r *http.Request) {
	// query param:?filter=expired
	filterMark := r.URL.Query().Get("filter")

	var podList []*corev1.Pod
	var err error
	if filterMark == "expired" {
		podList, err = h.podCache.ListExpiredPods(h.namespace)
		if err != nil {
			klog.Errorf("failed to list expired pods: %v", err)
			return
		}
	} else {
		// Get Pod from cache
		podList, err = h.podCache.ListPodsByNamespace(h.namespace)
		if err != nil {
			klog.Errorf("failed to list pods: %v", err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	httpListResponse := &HttpListResponse{
		Success: true,
		Code:    0,
	}
	for _, pod := range podList {
		httpListResponse.ListResponseData = append(httpListResponse.ListResponseData, HttpListResponseData{
			ID:        pod.Name,
			Status:    string(pod.Status.Phase),
			CreatedAt: pod.CreationTimestamp.Time,
			TTL:       pod.Labels[constants.AENV_TTL],
		})
	}

	err = json.NewEncoder(w).Encode(httpListResponse)
	if err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

// deletePod deletes Pod
/**
{
  "success": true,
  "code": 0,
  "data": true
}
*/
func (h *AEnvPodHandler) deletePod(podName string, w http.ResponseWriter, r *http.Request) {
	if podName == "" {
		http.Error(w, "missing pod name", http.StatusBadRequest)
		return
	}

	deleteOptions := metav1.DeleteOptions{}
	err := h.clientset.CoreV1().Pods(h.namespace).Delete(r.Context(), podName, deleteOptions)
	if err != nil {
		handleK8sAPiError(w, err, "failed to delete Pod")
		return
	}
	klog.Infof("delete pod %s/%s successfully", h.namespace, podName)

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

// handleK8sAPiError handles Kubernetes API errors
func handleK8sAPiError(w http.ResponseWriter, err error, action string) {
	if statusErr, ok := err.(*errors.StatusError); ok {
		status := statusErr.Status()
		http.Error(w, fmt.Sprintf("%s failed: err is %s", action, status.Message), httpStatusFromK8s(status.Code))
	} else {
		http.Error(w, fmt.Sprintf("%s failed: err is %v", action, err), http.StatusInternalServerError)
	}
}

// httpStatusFromK8s converts Kubernetes status code to HTTP status code
func httpStatusFromK8s(code int32) int {
	switch code {
	case 200, 201, 202:
		return http.StatusOK
	case 400:
		return http.StatusBadRequest
	case 401:
		return http.StatusUnauthorized
	case 403:
		return http.StatusForbidden
	case 404:
		return http.StatusNotFound
	case 409:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func MergePodImage(pod *corev1.Pod, aenv *model.AEnvHubEnv) {

	image := ""
	for _, artifact := range aenv.Artifacts {
		if artifact.Type == "image" {
			image = artifact.Content
			break
		}
	}

	// Update images for all containers
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].Image = image
	}
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].Image = image
		// Note: When podTemplate is Terminal, set second container's image to secondImageName
		if i == 1 {
			if secondImageName, ok := aenv.DeployConfig["secondImageName"]; ok {
				pod.Spec.Containers[i].Image = secondImageName.(string)
				klog.Infof("setting pod %s/%s second container image to %s", pod.Namespace, pod.Name, secondImageName.(string))
			}
		}
		// Update CPU and memory
		applyConfig(aenv.DeployConfig, &pod.Spec.Containers[i])
	}
}

func applyConfig(configs map[string]interface{}, container *corev1.Container) {

	klog.Infof("config environment variables: %v", configs["environmentVariables"])

	if environmentVariables, ok := configs["environmentVariables"].(map[string]interface{}); ok {
		if len(environmentVariables) > 0 {
			for k, v := range environmentVariables {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  k,
					Value: v.(string),
				})
			}
		}
	}

	if arguments, ok := configs["arguments"].([]string); ok {
		if len(arguments) > 0 {
			container.Args = append(container.Args, arguments...)
		}
	}

	// merge deploy config
	if configs["resource"] != "autoscale" {
		klog.Infof("resource not autoscale for container %s", container.Name)
		return
	}
	cfgCpu := configs["cpu"]
	strCpu := cfgCpu.(string)
	strMemory := configs["memory"].(string)
	klog.Infof("resource config cpu: %s, memory: %s", strCpu, strMemory)

	resources := container.Resources

	var expectCpu resource.Quantity
	var err error
	if expectCpu, err = resource.ParseQuantity(strCpu); err != nil {
		klog.Errorf("failed to parse cpu quantity %s: %v", strCpu, err)
		return
	}
	var expectMemory resource.Quantity
	if expectMemory, err = resource.ParseQuantity(strMemory); err != nil {
		klog.Errorf("failed to parse memory quantity %s: %v", strMemory, err)
		return
	}

	requestCpu := resources.Requests.Cpu()
	if requestCpu.Cmp(expectCpu) != 0 {
		klog.Infof("reset resource request cpu: %s, expect: %s", requestCpu.String(), expectCpu.String())
		container.Resources.Requests[corev1.ResourceCPU] = expectCpu
	}
	requestMemory := resources.Requests[corev1.ResourceMemory]
	if requestMemory.Cmp(expectMemory) != 0 {
		container.Resources.Requests[corev1.ResourceMemory] = expectMemory
	}

	limitCpu := resources.Limits[corev1.ResourceCPU]
	if limitCpu.Cmp(expectCpu) != 0 {
		klog.Infof("reset limit request cpu: %s, expect: %s", limitCpu.String(), expectCpu.String())
		container.Resources.Limits[corev1.ResourceCPU] = expectCpu
	}
	limitMemory := resources.Limits[corev1.ResourceMemory]
	if limitMemory.Cmp(expectMemory) != 0 {
		container.Resources.Limits[corev1.ResourceMemory] = expectMemory
	}
}
