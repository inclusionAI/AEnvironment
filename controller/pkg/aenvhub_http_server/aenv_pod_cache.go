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
	"controller/pkg/constants"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// AEnvPodCache caches Pod information
type AEnvPodCache struct {
	cache    cache.Indexer
	informer cache.Controller
	stopCh   chan struct{}
}

// NewAEnvPodCache creates new Pod cache
func NewAEnvPodCache(clientset kubernetes.Interface, namespace string) *AEnvPodCache {

	klog.Infof("Pod cache initialization starts (namespace: %s)", namespace)

	factory := informers.NewFilteredSharedInformerFactory(
		clientset,
		5*time.Minute,
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.Everything().String()
		},
	)

	podInformer := factory.Core().V1().Pods().Informer()

	stopCh := make(chan struct{})

	podCache := &AEnvPodCache{
		cache:    podInformer.GetIndexer(),
		informer: podInformer,
		stopCh:   stopCh,
	}

	// Start cache synchronization
	go podInformer.Run(stopCh)

	// Wait for cache synchronization to complete
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		klog.Fatalf("failed to wait for cache sync!")
	}

	klog.Infof("Pod cache initialization finished (namespace: %s), number of pods is %d", namespace, len(podCache.cache.ListKeys()))
	return podCache
}

// WaitForCacheSync waits for cache synchronization
func (c *AEnvPodCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, c.informer.HasSynced)
}

// Stop stops cache
func (c *AEnvPodCache) Stop() {
	close(c.stopCh)
}

// GetPod gets Pod from cache
func (c *AEnvPodCache) GetPod(namespace, name string) (*corev1.Pod, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, exists, err := c.cache.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("pod %s not found in cache", key)
	}
	return item.(*corev1.Pod), nil
}

// ListPodsByNamespace lists Pods by namespace
func (c *AEnvPodCache) ListPodsByNamespace(namespace string) ([]*corev1.Pod, error) {
	items, err := c.cache.ByIndex(cache.NamespaceIndex, namespace)
	if err != nil {
		return nil, err
	}

	pods := make([]*corev1.Pod, len(items))
	for i, item := range items {
		pods[i] = item.(*corev1.Pod)
	}
	return pods, nil
}

// ListExpiredPods list all expired pods
func (c *AEnvPodCache) ListExpiredPods(namespace string) ([]*corev1.Pod, error) {
	items, err := c.cache.ByIndex(cache.NamespaceIndex, namespace)
	if err != nil {
		return nil, err
	}

	expired := make([]*corev1.Pod, 0)
	for _, item := range items {
		pod := item.(*corev1.Pod)
		ttlValue := pod.Labels[constants.AENV_TTL]
		if ttlValue == "" {
			continue
		}
		var limited time.Duration
		if limited, err = time.ParseDuration(ttlValue); err != nil {
			klog.Warningf("Failed to parse ttl value %s for pod %s will not auto clean", ttlValue, pod.Name)
			continue
		}

		createdAt := pod.CreationTimestamp
		currentTime := time.Now()
		if currentTime.Sub(createdAt.Time) > limited {
			klog.Infof("Instance %s has expired (created: %s, ttl: %v), deleting...", pod.Name, createdAt, limited)
		}
		expired = append(expired, pod)
	}
	return expired, nil
}
