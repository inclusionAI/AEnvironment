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

package model

type Env struct {
	Id string `json:"id,omitempty"`
	// Environment name
	Name string `json:"name" binging:"required"`
	// Environment description
	Description string `json:"description,omitempty"`
	// Image|zip package needed to start environment
	Content EnvContent `json:"content,omitempty"`
	// Whether stateful, default false
	Stateful bool `json:"stateful,omitempty" default:"false"`
	// Custom labels
	Labels map[string]string `json:"labels,omitempty"`
	// Container injected environment variables
	Envs map[string]string `json:"envs,omitempty"`
	// Allocated memory size, minimum 256MiB, maximum 16GiB
	Memory int `json:"memory,omitempty" default:"2048" binding:"min=256,max=16384"`
	// Allocated disk size, minimum 1 GiB, maximum 50 GiB
	EphemeralStorage int64 `json:"ephemeralStorage,omitempty" default:"10737418240" binding:"min=0,max=102400"`
	// Pod idle recycle time, default 360s, max 5 hours
	ExpiredTime int `json:"expiredTime,omitempty" default:"360" binding:"min=0,max=18000"`
	// Prewarm instance count
	PrewarmSize int `json:"prewarmSize,omitempty" default:"10"`
	// Cluster where EnvInstance is created
	ClusterName string `json:"clusterName,omitempty"`
	// Namespace where EnvInstance is created
	Namespace string `json:"namespace,omitempty"`
	// Container type
	MachineType string `json:"machineType,omitempty"`

	Status string `json:"status,omitempty"`

	CreateTimestamp int64 `protobuf:"varint,15,opt,name=createTimestamp,proto3" json:"createTimestamp"`
	UpdateTimeStamp int64 `protobuf:"varint,16,opt,name=updateTimeStamp,proto3" json:"updateTimeStamp"`
	Version         int64 `protobuf:"varint,17,opt,name=version,proto3" json:"version"`
	Revision        int64 `protobuf:"varint,18,opt,name=revision,proto3" json:"revision"`
}

type EnvList struct {
	Envs []*Env `json:"envs"`
}

type EnvCreateRequest struct {
	// Environment instance name
	Name string `json:"name" binging:"required"`
	// Environment instance description
	Description string `json:"description,omitempty"`
	// Unique identifier for environment info, defaults to default image in k8s pool when not provided; provides real-time pod startup (slower startup) when provided. Supports image and env id
	Content EnvContent `json:"env,omitempty"`
	// Whether stateful instance, default false
	Stateful bool `json:"stateful,omitempty" default:"false"`
	// User custom labels
	Labels map[string]string `json:"labels,omitempty"`
	// Container injected environment variables
	Environs map[string]string `json:"environs,omitempty"`
	// Allocated memory size, minimum 256MiB, maximum 8GiB
	Memory int `json:"memory,omitempty" default:"2048" binding:"min=256,max=8192"`
	// Allocated disk size, minimum 1 GiB, maximum 50 GiB
	EphemeralStorage int64 `json:"ephemeralStorage,omitempty" default:"10737418240" binding:"min=0,max=102400"`
	// Pod idle recycle time, default 360s, max 3600s
	ExpiredTime int `json:"expiredTime,omitempty" default:"360" binding:"min=1,max=3600"`
	// Prewarm instance count
	PrewarmSize int `json:"prewarmSize,omitempty" default:"10"`
	// Cluster where EnvInstance is created
	ClusterName string `json:"clusterName,omitempty"`
	// Namespace where EnvInstance is created
	NamespaceName string `json:"namespaceName,omitempty"`
}

type EnvContent struct {
	// Zip package in envhub
	ZipFile string `json:"zipFile,omitempty"`
	// Zip package in OSS
	OssUrl string `json:"ossUrl,omitempty"`
	// Image address
	Image string `json:"image,omitempty"`
}

type EnvInstance struct {
	Id               string            `json:"id,omitempty"`
	Name             string            `json:"name"`
	Description      string            `json:"description,omitempty"`
	Env              string            `json:"env,omitempty"`
	Stateful         bool              `json:"stateful,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Environs         map[string]string `json:"environs,omitempty"`
	Memory           int               `json:"memory,omitempty"`
	EphemeralStorage int64             `json:"ephemeralStorage,omitempty"`
	ExpiredTime      int               `json:"expiredTime,omitempty"`
	Status           string            `json:"status,omitempty"`
	CreatedAt        string            `json:"createdAt,omitempty"`
	UpdatedAt        string            `json:"updatedAt,omitempty"`
	Address          interface{}       `json:"address,omitempty"`
}

func ConvertEnvInstanceToPodInfo(envInstance *EnvInstance) interface{} {
	// TODO: Convert to PodInfo
	return map[string]interface{}{}
}

func ConvertPodInfoToEnvInstance(podInfo interface{}) *EnvInstance {
	// TODO: Convert to EnvInstance
	return &EnvInstance{
		Address: podInfo,
	}
}

type EnvInstanceList struct {
	EnvInstances []*EnvInstance `json:"envInstances"`
}

type EnvInstanceCreateRequest struct {
	// Environment instance name
	Name string `json:"name" binging:"required"`
	// Environment instance description
	Description string `json:"description,omitempty"`
	// Corresponding environment name
	Env string `json:"env,omitempty" binding:"required"`
	// User custom labels
	Labels map[string]string `json:"labels,omitempty"`
}
