/*
Copyright 2022 The Karmada Authors.

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

package cluster

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	config "github.com/karmada-io/karmada/pkg/controllers/cluster/evictionqueue_config"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// maxEvictionDelay is the maximum delay for eviction when the rate is 0
const maxEvictionDelay = 1000 * time.Second

// DynamicRateLimiter adjusts its rate based on the overall health of clusters.
// It implements the workqueue.RateLimiter interface with dynamic behavior.
type DynamicRateLimiter[T comparable] struct {
	resourceEvictionRate          float32
	secondaryResourceEvictionRate float32
	unhealthyClusterThreshold     float32
	largeClusterNumThreshold      int
	informerManager               genericmanager.SingleClusterInformerManager
}

// NewDynamicRateLimiter creates a new DynamicRateLimiter with the given options.
func NewDynamicRateLimiter[T comparable](informerManager genericmanager.SingleClusterInformerManager, opts config.EvictionQueueOptions) workqueue.TypedRateLimiter[T] {
	return &DynamicRateLimiter[T]{
		resourceEvictionRate:          opts.ResourceEvictionRate,
		secondaryResourceEvictionRate: opts.SecondaryResourceEvictionRate,
		unhealthyClusterThreshold:     opts.UnhealthyClusterThreshold,
		largeClusterNumThreshold:      opts.LargeClusterNumThreshold,
		informerManager:               informerManager,
	}
}

// When determines how long to wait before processing an item.
// Returns a longer delay when the system is unhealthy.
func (d *DynamicRateLimiter[T]) When(item T) time.Duration {
	currentRate := d.getCurrentRate()
	if currentRate == 0 {
		return maxEvictionDelay
	}
	return time.Duration(1 / currentRate * float32(time.Second))
}

// getCurrentRate calculates the appropriate rate based on cluster health:
// - Normal rate when system is healthy
// - Secondary rate when system is unhealthy but large-scale
// - Zero (halt evictions) when system is unhealthy and small-scale
func (d *DynamicRateLimiter[T]) getCurrentRate() float32 {
	clusterGVR := clusterv1alpha1.SchemeGroupVersion.WithResource("clusters")

	var lister = d.informerManager.Lister(clusterGVR)
	if lister == nil {
		klog.Errorf("Failed to get cluster lister, halting eviction for safety")
		return 0
	}

	clusters, err := lister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters from informer cache: %v, halting eviction for safety", err)
		return 0
	}

	totalClusters := len(clusters)
	if totalClusters == 0 {
		return d.resourceEvictionRate
	}

	unhealthyClusters := 0
	for _, clusterObj := range clusters {
		cluster, ok := clusterObj.(*clusterv1alpha1.Cluster)
		if !ok {
			continue
		}
		if !util.IsClusterReady(&cluster.Status) {
			unhealthyClusters++
		}
	}

	// Update metrics
	failureRate := float32(unhealthyClusters) / float32(totalClusters)
	metrics.RecordClusterHealthMetrics(unhealthyClusters, float64(failureRate))

	// Determine rate based on health status
	isUnhealthy := failureRate > d.unhealthyClusterThreshold
	if !isUnhealthy {
		return d.resourceEvictionRate
	}

	isLargeScale := totalClusters > d.largeClusterNumThreshold
	if isLargeScale {
		klog.V(2).Infof("System is unhealthy (failure rate: %.2f), downgrading eviction rate to secondary rate: %.2f/s",
			failureRate, d.secondaryResourceEvictionRate)
		return d.secondaryResourceEvictionRate
	}

	klog.V(2).Infof("System is unhealthy (failure rate: %.2f) and instance is small, halting eviction.", failureRate)
	return 0
}

// Forget is a no-op as this rate limiter doesn't track individual items.
func (d *DynamicRateLimiter[T]) Forget(item T) {
	// No-op
}

// NumRequeues always returns 0 as this rate limiter doesn't track retries.
func (d *DynamicRateLimiter[T]) NumRequeues(item T) int {
	return 0
}

// NewGracefulEvictionRateLimiter creates a combined rate limiter for eviction.
// It uses the maximum delay from both dynamic and default rate limiters to ensure
// both cluster health and retry backoff are considered.
func NewGracefulEvictionRateLimiter[T comparable](
	informerManager genericmanager.SingleClusterInformerManager,
	evictionOpts config.EvictionQueueOptions,
	rateLimiterOpts ratelimiterflag.Options) workqueue.TypedRateLimiter[T] {

	dynamicLimiter := NewDynamicRateLimiter[T](informerManager, evictionOpts)
	defaultLimiter := ratelimiterflag.DefaultControllerRateLimiter[T](rateLimiterOpts)
	return workqueue.NewTypedMaxOfRateLimiter[T](dynamicLimiter, defaultLimiter)
}
