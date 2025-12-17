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
)

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
	const podTemplateBaseDir = "/etc/aenv/pod-templates"

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
