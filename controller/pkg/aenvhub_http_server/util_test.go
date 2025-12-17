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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestApplyConfig(t *testing.T) {
	expectedPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewQuantity(1073741824, resource.BinarySI), // resource.MustParse("1073741824"), // 1024 * 1024 * 1024
							corev1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewQuantity(1073741824, resource.BinarySI),
							corev1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewQuantity(1073741824, resource.BinarySI),
							corev1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewQuantity(1073741824, resource.BinarySI),
							corev1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
	configs := make(map[string]interface{})
	configs["cpu"] = "4"
	configs["memory"] = "8G"
	configs["resource"] = "autoscale"

	applyConfig(configs, &expectedPod.Spec.Containers[0])

	fmt.Println(expectedPod.Spec.Containers[0].Resources.Requests)
}
