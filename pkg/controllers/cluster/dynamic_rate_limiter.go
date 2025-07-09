/*
Copyright 2025 The Karmada Authors.

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

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

// maxEvictionDelay is the maximum delay for eviction when the rate is 0
const maxEvictionDelay = 1800 * time.Second

// DynamicRateLimiter adjusts its rate based on the overall health of clusters.
// It implements the workqueue.RateLimiter interface with dynamic behavior.
type DynamicRateLimiter[T comparable] struct {
	resourceEvictionRate float32
}

// NewDynamicRateLimiter creates a new DynamicRateLimiter with the given options.
func NewDynamicRateLimiter[T comparable](opts EvictionQueueOptions) workqueue.TypedRateLimiter[T] {
	return &DynamicRateLimiter[T]{
		resourceEvictionRate: opts.ResourceEvictionRate,
	}
}

// When determines how long to wait before processing an item.
// Returns a longer delay when the system is unhealthy.
func (d *DynamicRateLimiter[T]) When(_ T) time.Duration {
	currentRate := d.getCurrentRate()
	klog.V(4).Infof("DynamicRateLimiter: Current rate: %.2f/s", currentRate)
	if currentRate == 0 {
		return maxEvictionDelay
	}
	return time.Duration(1 / currentRate * float32(time.Second))
}

// getCurrentRate returns the configured rate.
// For safety, when informer lister is unavailable or listing fails, it returns 0 to halt evictions.
func (d *DynamicRateLimiter[T]) getCurrentRate() float32 {
	return d.resourceEvictionRate
}

// Forget is a no-op as this rate limiter doesn't track individual items.
func (d *DynamicRateLimiter[T]) Forget(_ T) {
	// No-op
}

// NumRequeues always returns 0 as this rate limiter doesn't track retries.
func (d *DynamicRateLimiter[T]) NumRequeues(_ T) int {
	return 0
}

// NewGracefulEvictionRateLimiter creates a combined rate limiter for eviction.
// It uses the maximum delay from both dynamic and default rate limiters to ensure
// both cluster health and retry backoff are considered.
func NewGracefulEvictionRateLimiter[T comparable](
	evictionOpts EvictionQueueOptions,
	rateLimiterOpts ratelimiterflag.Options) workqueue.TypedRateLimiter[T] {
	dynamicLimiter := NewDynamicRateLimiter[T](evictionOpts)
	defaultLimiter := ratelimiterflag.DefaultControllerRateLimiter[T](rateLimiterOpts)
	return workqueue.NewTypedMaxOfRateLimiter[T](dynamicLimiter, defaultLimiter)
}
