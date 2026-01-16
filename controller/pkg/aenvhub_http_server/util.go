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
	"fmt"
	"math/rand"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

const (
	letters                 = "abcdefghijklmnopqrstuvwxyz0123456789" // ABCDEFGHIJKLMNOPQRSTUVWXYZ
	envInstanceName         = "env-pod-pool-name"
	AMD64                   = "amd64"
	WIN64                   = "win64"
	SingleContainerTemplate = "singleContainer"
	DeploymentTemplate      = "deployment"
	ServiceTemplate         = "service"
	PVCTemplate             = "pvc"
	podTemplateBaseDir      = "/etc/aenv/pod-templates"
)

// ResourceConfig holds resource configuration for containers
type ResourceConfig struct {
	CPURequest              string
	CPULimit                string
	MemoryRequest           string
	MemoryLimit             string
	EphemeralStorageRequest string
	EphemeralStorageLimit   string
}

func RandString(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

func MergePodWithFields(pod *corev1.Pod, Labels map[string]string,
	Environs map[string]string,
	Memory int,
	EphemeralStorage int64) {

}

// AddLabelToPod adds label key=value to Pod
func AddLabelToPod(pod *corev1.Pod, poolName string, description string) {
	if pod == nil {
		return
	}
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[envInstanceName] = poolName
	// pod.Labels[envInstanceDescription] = description
}

func MergePod(pod *corev1.Pod, labels map[string]string, environs map[string]string, memory int, ephemeralStorage int64, image string) {
	// Pre-calculate byte boundaries for resource validation
	const (
		minMemoryBytes           = 256 * 1024 * 1024       // 256MiB
		maxMemoryBytes           = 8 * 1024 * 1024 * 1024  // 8GiB
		minEphemeralStorageBytes = 1 * 1024 * 1024 * 1024  // 1GiB
		maxEphemeralStorageBytes = 50 * 1024 * 1024 * 1024 // 50GiB
	)

	// Merge labels
	if labels != nil {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		for k, v := range labels {
			pod.Labels[k] = v
		}
	}

	// Merge environment variables
	if environs != nil {
		mergeEnvVars := func(containers []corev1.Container) {
			for i := range containers {
				container := &containers[i]
				for k, v := range environs {
					found := false
					for j := range container.Env {
						if container.Env[j].Name == k {
							container.Env[j].Value = v
							found = true
							break
						}
					}
					if !found {
						container.Env = append(container.Env, corev1.EnvVar{
							Name:  k,
							Value: v,
						})
					}
				}
			}
		}
		mergeEnvVars(pod.Spec.InitContainers)
		mergeEnvVars(pod.Spec.Containers)
	}

	// Helper to update container resources
	updateResources := func(container *corev1.Container) {
		// Validate and set memory resources
		memoryBytes := int64(memory) * 1024 * 1024
		if memory >= 256 && memory <= 8192 { // 256MiB-8192MiB (8GiB)
			memQty := resource.NewQuantity(memoryBytes, resource.BinarySI)
			if container.Resources.Requests == nil {
				container.Resources.Requests = make(corev1.ResourceList)
			}
			container.Resources.Requests[corev1.ResourceMemory] = *memQty

			if container.Resources.Limits == nil {
				container.Resources.Limits = make(corev1.ResourceList)
			}
			container.Resources.Limits[corev1.ResourceMemory] = *memQty

			// klog.Infof("set mem req to xxx, %v", container.Resources.Requests[corev1.ResourceMemory])
			// klog.Infof("set mem limit to xxx, %v", container.Resources.Limits[corev1.ResourceMemory])
		}

		// Validate and set ephemeral storage resources
		if ephemeralStorage >= minEphemeralStorageBytes && ephemeralStorage <= maxEphemeralStorageBytes {
			storageQty := resource.NewQuantity(ephemeralStorage, resource.BinarySI)
			if container.Resources.Requests == nil {
				container.Resources.Requests = make(corev1.ResourceList)
			}
			container.Resources.Requests[corev1.ResourceEphemeralStorage] = *storageQty

			if container.Resources.Limits == nil {
				container.Resources.Limits = make(corev1.ResourceList)
			}
			container.Resources.Limits[corev1.ResourceEphemeralStorage] = *storageQty
		}
	}

	// Update resources for all containers
	for i := range pod.Spec.InitContainers {
		updateResources(&pod.Spec.InitContainers[i])

		// Image
		pod.Spec.InitContainers[i].Image = image
	}
	for i := range pod.Spec.Containers {
		updateResources(&pod.Spec.Containers[i])

		// Image
		pod.Spec.Containers[i].Image = image
	}
}

// Machine type: win64, amd64, darwin64
// LoadPodTemplateFromYaml loads Pod template from mounted ConfigMap directory
// machineType: template type, such as "amd64", "win64", "singleContainer", etc.
func LoadPodTemplateFromYaml(machineType string) *corev1.Pod {
	// Construct template file path
	templateFilePath := fmt.Sprintf("%s/%s.yaml", podTemplateBaseDir, machineType)

	// Try to read from mounted directory
	yamlFile, err := os.ReadFile(templateFilePath)
	if err != nil {
		// If mounted template doesn't exist, fall back to old hardcoded path
		klog.Warningf("failed to read template from %s: %v, falling back to legacy path", templateFilePath, err)
		return loadPodTemplateFromLegacyPath(machineType)
	}

	klog.Infof("loaded pod template from %s for type %s", templateFilePath, machineType)

	// Deserialize YAML to Pod object
	var pod *corev1.Pod
	if err := yaml.Unmarshal(yamlFile, &pod); err != nil {
		panic(fmt.Errorf("failed to unmarshal YAML from %s: %v", templateFilePath, err))
	}

	// Clear auto-generated fields
	pod.ResourceVersion = ""
	pod.UID = ""

	return pod
}

// loadPodTemplateFromLegacyPath loads template from old hardcoded path (backward compatibility)
func loadPodTemplateFromLegacyPath(machineType string) *corev1.Pod {
	templateFileName := "/home/admin/pod_template_linux/config.yaml"
	switch machineType {
	case AMD64, SingleContainerTemplate:
		templateFileName = "/home/admin/pod_template_linux/config.yaml"
	case WIN64:
		templateFileName = "/home/admin/pod_template_windows/config.yaml"
	case "Terminal":
		templateFileName = "/home/admin/pod_template_terminal/config.yaml"
	}

	yamlFile, err := os.ReadFile(templateFileName)
	if err != nil {
		panic(fmt.Errorf("failed to read YAML file %s: %v", templateFileName, err))
	}

	klog.Infof("loaded template from legacy path %s for type %s", templateFileName, machineType)

	var pod *corev1.Pod
	if err := yaml.Unmarshal(yamlFile, &pod); err != nil {
		panic(fmt.Errorf("failed to unmarshal YAML: %v", err))
	}

	return pod
}

// LoadNsFromPodTemplate gets namespace
func LoadNsFromPodTemplate(machineType string) string {
	pod := LoadPodTemplateFromYaml(machineType)
	klog.Infof("load ns %s from pod template", pod.Namespace)
	return pod.Namespace
}

/*
		Status        string `json:"status,omitempty"`

		CreateTimestamp int64 `protobuf:"varint,15,opt,name=createTimestamp,proto3" json:"createTimestamp"`
		UpdateTimeStamp int64 `protobuf:"varint,16,opt,name=updateTimeStamp,proto3" json:"updateTimeStamp"`
		Version         int64 `protobuf:"varint,17,opt,name=version,proto3" json:"version"`
		Revision        int64 `protobuf:"varint,18,opt,name=revision,proto3" json:"revision"`
	}
*/

type CustomTime struct {
	time.Time
}

const customTimeLayout = "2006/01/02 15:04:05.000000"

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	formatted := ct.Format(customTimeLayout)
	return []byte(`"` + formatted + `"`), nil
}

func (ct *CustomTime) UnmarshalJSON(data []byte) error {

	loc, err := time.LoadLocation("Asia/Shanghai") // China Standard Time (UTC+8)
	if err != nil {
		klog.Errorf("failed to set time loc, should not happen, err is %v", err)
	}

	str := string(data)
	str = str[1 : len(str)-1] // remove the quotes
	t, err := time.ParseInLocation(customTimeLayout, str, loc)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}

// LoadDeploymentTemplateFromYaml loads Deployment template from mounted ConfigMap directory
func LoadDeploymentTemplateFromYaml() *appsv1.Deployment {
	templateFilePath := fmt.Sprintf("%s/%s.yaml", podTemplateBaseDir, DeploymentTemplate)

	yamlFile, err := os.ReadFile(templateFilePath)
	if err != nil {
		panic(fmt.Errorf("failed to read deployment template from %s: %v", templateFilePath, err))
	}

	klog.Infof("loaded deployment template from %s", templateFilePath)

	var deployment *appsv1.Deployment
	if err := yaml.Unmarshal(yamlFile, &deployment); err != nil {
		panic(fmt.Errorf("failed to unmarshal deployment YAML from %s: %v", templateFilePath, err))
	}

	// Clear auto-generated fields
	deployment.ResourceVersion = ""
	deployment.UID = ""

	return deployment
}

// LoadServiceTemplateFromYaml loads Service template from mounted ConfigMap directory
func LoadServiceTemplateFromYaml() *corev1.Service {
	templateFilePath := fmt.Sprintf("%s/%s.yaml", podTemplateBaseDir, ServiceTemplate)

	yamlFile, err := os.ReadFile(templateFilePath)
	if err != nil {
		panic(fmt.Errorf("failed to read service template from %s: %v", templateFilePath, err))
	}

	klog.Infof("loaded service template from %s", templateFilePath)

	var service *corev1.Service
	if err := yaml.Unmarshal(yamlFile, &service); err != nil {
		panic(fmt.Errorf("failed to unmarshal service YAML from %s: %v", templateFilePath, err))
	}

	// Clear auto-generated fields
	service.ResourceVersion = ""
	service.UID = ""

	return service
}

// LoadPVCTemplateFromYaml loads PVC template from mounted ConfigMap directory
func LoadPVCTemplateFromYaml() *corev1.PersistentVolumeClaim {
	templateFilePath := fmt.Sprintf("%s/%s.yaml", podTemplateBaseDir, PVCTemplate)

	yamlFile, err := os.ReadFile(templateFilePath)
	if err != nil {
		panic(fmt.Errorf("failed to read PVC template from %s: %v", templateFilePath, err))
	}

	klog.Infof("loaded PVC template from %s", templateFilePath)

	var pvc *corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(yamlFile, &pvc); err != nil {
		panic(fmt.Errorf("failed to unmarshal PVC YAML from %s: %v", templateFilePath, err))
	}

	// Clear auto-generated fields
	pvc.ResourceVersion = ""
	pvc.UID = ""

	return pvc
}

// MergeDeployment merges environment-specific configuration into a Deployment template
func MergeDeployment(deployment *appsv1.Deployment, name string, replicas int32, labels map[string]string, environs map[string]string, image string, pvcName string, mountPath string, resources *ResourceConfig) {
	// Set deployment name
	deployment.Name = name
	deployment.Spec.Replicas = &replicas

	// Update selector labels and template labels based on what's defined in the template
	// Preserve the selector keys from template, only update their values
	if deployment.Spec.Selector.MatchLabels == nil {
		deployment.Spec.Selector.MatchLabels = make(map[string]string)
	}
	if deployment.Spec.Template.Labels == nil {
		deployment.Spec.Template.Labels = make(map[string]string)
	}

	for key := range deployment.Spec.Selector.MatchLabels {
		deployment.Spec.Selector.MatchLabels[key] = name
		deployment.Spec.Template.Labels[key] = name
	}

	// Merge additional labels (preserve existing labels from template)
	// These additional labels are only added to deployment metadata and template labels
	// NOT to selector, to avoid selector mismatch
	if deployment.Labels == nil {
		deployment.Labels = make(map[string]string)
	}
	if labels != nil {
		for k, v := range labels {
			deployment.Labels[k] = v
			deployment.Spec.Template.Labels[k] = v
		}
	}

	// Update container image, env vars, and resources
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]

		// Set image
		if image != "" {
			container.Image = image
		}

		// Merge environment variables
		if environs != nil {
			for k, v := range environs {
				found := false
				for j := range container.Env {
					if container.Env[j].Name == k {
						container.Env[j].Value = v
						found = true
						break
					}
				}
				if !found {
					container.Env = append(container.Env, corev1.EnvVar{
						Name:  k,
						Value: v,
					})
				}
			}
		}

		// Update resource limits and requests
		if resources != nil {
			if container.Resources.Requests == nil {
				container.Resources.Requests = make(corev1.ResourceList)
			}
			if container.Resources.Limits == nil {
				container.Resources.Limits = make(corev1.ResourceList)
			}

			// CPU
			if cpuReq, err := resource.ParseQuantity(resources.CPURequest); err == nil {
				container.Resources.Requests[corev1.ResourceCPU] = cpuReq
			} else {
				klog.Warningf("failed to parse CPU request %s: %v", resources.CPURequest, err)
			}
			if cpuLimit, err := resource.ParseQuantity(resources.CPULimit); err == nil {
				container.Resources.Limits[corev1.ResourceCPU] = cpuLimit
			} else {
				klog.Warningf("failed to parse CPU limit %s: %v", resources.CPULimit, err)
			}

			// Memory
			if memReq, err := resource.ParseQuantity(resources.MemoryRequest); err == nil {
				container.Resources.Requests[corev1.ResourceMemory] = memReq
			} else {
				klog.Warningf("failed to parse memory request %s: %v", resources.MemoryRequest, err)
			}
			if memLimit, err := resource.ParseQuantity(resources.MemoryLimit); err == nil {
				container.Resources.Limits[corev1.ResourceMemory] = memLimit
			} else {
				klog.Warningf("failed to parse memory limit %s: %v", resources.MemoryLimit, err)
			}

			// Ephemeral Storage
			if storageReq, err := resource.ParseQuantity(resources.EphemeralStorageRequest); err == nil {
				container.Resources.Requests[corev1.ResourceEphemeralStorage] = storageReq
			} else {
				klog.Warningf("failed to parse ephemeral storage request %s: %v", resources.EphemeralStorageRequest, err)
			}
			if storageLimit, err := resource.ParseQuantity(resources.EphemeralStorageLimit); err == nil {
				container.Resources.Limits[corev1.ResourceEphemeralStorage] = storageLimit
			} else {
				klog.Warningf("failed to parse ephemeral storage limit %s: %v", resources.EphemeralStorageLimit, err)
			}
		}

		// Update mount path in volumeMounts
		if mountPath != "" {
			for j := range container.VolumeMounts {
				volumeMount := &container.VolumeMounts[j]
				// Update the data volume mount path
				if volumeMount.Name == "data" {
					volumeMount.MountPath = mountPath
				}
			}
		}
	}

	// Update PVC name in volumes, or remove PVC volumes if pvcName is empty
	if pvcName == "" {
		// Remove PVC volumes and their corresponding volumeMounts when no storage is requested
		var filteredVolumes []corev1.Volume
		for i := range deployment.Spec.Template.Spec.Volumes {
			volume := &deployment.Spec.Template.Spec.Volumes[i]
			if volume.PersistentVolumeClaim == nil {
				// Keep non-PVC volumes
				filteredVolumes = append(filteredVolumes, *volume)
			}
		}
		deployment.Spec.Template.Spec.Volumes = filteredVolumes

		// Remove volumeMounts that reference removed PVC volumes (e.g., "data" volume)
		for i := range deployment.Spec.Template.Spec.Containers {
			container := &deployment.Spec.Template.Spec.Containers[i]
			var filteredMounts []corev1.VolumeMount
			for j := range container.VolumeMounts {
				volumeMount := &container.VolumeMounts[j]
				// Keep volumeMounts that have a corresponding volume in the deployment
				hasVolume := false
				for k := range deployment.Spec.Template.Spec.Volumes {
					if deployment.Spec.Template.Spec.Volumes[k].Name == volumeMount.Name {
						hasVolume = true
						break
					}
				}
				if hasVolume {
					filteredMounts = append(filteredMounts, *volumeMount)
				}
			}
			container.VolumeMounts = filteredMounts
		}
	} else {
		// Update PVC ClaimName when pvcName is provided
		for i := range deployment.Spec.Template.Spec.Volumes {
			volume := &deployment.Spec.Template.Spec.Volumes[i]
			if volume.PersistentVolumeClaim != nil {
				volume.PersistentVolumeClaim.ClaimName = pvcName
			}
		}
	}
}

// MergeService merges environment-specific configuration into a Service template
func MergeService(service *corev1.Service, name string, port int32) {
	// Set service name
	service.Name = name

	// Update selector
	if service.Spec.Selector == nil {
		service.Spec.Selector = make(map[string]string)
	}
	service.Spec.Selector["app"] = name

	// Update port if specified
	if port > 0 && len(service.Spec.Ports) > 0 {
		service.Spec.Ports[0].Port = port
		service.Spec.Ports[0].TargetPort.IntVal = port
	}
}

// MergePVC merges environment-specific configuration into a PVC template
func MergePVC(pvc *corev1.PersistentVolumeClaim, name string, storageSize string, storageClass string) {
	// Set PVC name
	pvc.Name = name

	// Update storage size if specified
	if storageSize != "" {
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = make(corev1.ResourceList)
		}
		quantity, err := resource.ParseQuantity(storageSize)
		if err == nil {
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = quantity
		} else {
			klog.Warningf("failed to parse storage size %s: %v", storageSize, err)
		}
	}

	// Update storage class only if specified (otherwise use template default)
	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass
	}
	// Note: If storageClass is empty, the StorageClassName from template (values.yaml) is preserved
}
