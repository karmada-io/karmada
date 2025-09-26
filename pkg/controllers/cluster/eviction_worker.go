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
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// EvictionQueueOptions holds the options that control the behavior of the graceful eviction queue based on the overall health of the clusters.
type EvictionQueueOptions struct {
	// ResourceEvictionRate is the number of resources to be evicted per second.
	// This is the default rate when the system is considered healthy.
	ResourceEvictionRate float32
	// SecondaryResourceEvictionRate is the secondary resource eviction rate.
	// When the number of cluster failures in the Karmada instance exceeds the UnhealthyClusterThreshold,
	// the resource eviction rate will be reduced to this secondary level.
	SecondaryResourceEvictionRate float32
	// UnhealthyClusterThreshold is the threshold of unhealthy clusters.
	// If the ratio of unhealthy clusters to total clusters exceeds this threshold, there are too many cluster failures in the Karmada instance
	// and the eviction rate will be downgraded to the secondary rate.
	UnhealthyClusterThreshold float32
	// LargeClusterNumThreshold is the threshold for a large-scale Karmada instance.
	// When the number of clusters in the instance exceeds this threshold and the instance is unhealthy,
	// the eviction rate is downgraded. For smaller instances that are unhealthy, eviction might be halted completely.
	LargeClusterNumThreshold int
}

type evictionWorker struct {
	name             string
	keyFunc          util.KeyFunc
	reconcileFunc    util.ReconcileFunc
	resourceKindFunc func(key interface{}) (clusterName, resourceKind string)
	queue            workqueue.TypedRateLimitingInterface[any]
	// pacer is the combined limiter (dynamic + default) used to throttle the
	// processing throughput between items even when initial enqueues are immediate.
	// It ensures overall throughput follows the dynamic rate under healthy/unhealthy states.
	pacer workqueue.TypedRateLimiter[any]
	// pacerKey is a sentinel key used for pacing. We call When/Forget on this key
	// for each successfully processed item to avoid exponential backoff accumulation.
	pacerKey any
}

// EvictionWorkerOptions configures a new EvictionWorker instance.
type EvictionWorkerOptions struct {
	// Name is the queue's name used for metrics and logging
	Name string

	// KeyFunc generates keys from objects for queue operations
	KeyFunc util.KeyFunc

	// ReconcileFunc processes keys from the queue
	ReconcileFunc util.ReconcileFunc

	// ResourceKindFunc returns resource metadata for metrics collection
	ResourceKindFunc func(key interface{}) (clusterName, resourceKind string)

	// InformerManager provides cluster information for dynamic rate limiting
	InformerManager genericmanager.SingleClusterInformerManager

	// EvictionQueueOptions configures dynamic rate limiting behavior
	EvictionQueueOptions EvictionQueueOptions

	// RateLimiterOptions configures general rate limiter behavior
	RateLimiterOptions ratelimiterflag.Options
}

// NewEvictionWorker creates a new EvictionWorker with dynamic rate limiting.
func NewEvictionWorker(opts EvictionWorkerOptions) util.AsyncWorker {
	rateLimiter := NewGracefulEvictionRateLimiter[any](
		opts.InformerManager,
		opts.EvictionQueueOptions,
		opts.RateLimiterOptions,
	)

	return &evictionWorker{
		name:             opts.Name,
		keyFunc:          opts.KeyFunc,
		reconcileFunc:    opts.ReconcileFunc,
		resourceKindFunc: opts.ResourceKindFunc,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig[any](rateLimiter, workqueue.TypedRateLimitingQueueConfig[any]{
			Name: opts.Name,
		}),
		pacer:    rateLimiter,
		pacerKey: struct{}{},
	}
}

// Enqueue converts an object to a key and adds it to the queue.
func (w *evictionWorker) Enqueue(obj interface{}) {
	key, err := w.keyFunc(obj)
	if err != nil {
		klog.Errorf("Failed to generate key for obj: %+v, err: %v", obj, err)
		return
	}

	if key == nil {
		return
	}

	w.Add(key)
}

// Add puts an item into the queue and updates metrics.
func (w *evictionWorker) Add(item interface{}) {
	if item == nil {
		klog.Warningf("Ignore nil item from queue")
		return
	}

	w.queue.Add(item)
	metrics.RecordEvictionQueueMetrics(w.name, float64(w.queue.Len()))

	// Update resource kind metrics if possible
	if w.resourceKindFunc != nil {
		clusterName, resourceKind := w.resourceKindFunc(item)
		metrics.RecordEvictionKindMetrics(clusterName, resourceKind, true)
	}
}

// AddAfter adds an item to the queue after a delay and updates metrics.
func (w *evictionWorker) AddAfter(item interface{}, duration time.Duration) {
	if item == nil {
		klog.Warningf("Ignore nil item from queue")
		return
	}

	w.queue.AddAfter(item, duration)
	metrics.RecordEvictionQueueMetrics(w.name, float64(w.queue.Len()))

	// Update resource kind metrics if possible
	if w.resourceKindFunc != nil {
		clusterName, resourceKind := w.resourceKindFunc(item)
		metrics.RecordEvictionKindMetrics(clusterName, resourceKind, true)
	}
}

// worker processes items from the queue until the context is canceled.
func (w *evictionWorker) worker(ctx context.Context) {
	for w.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem handles a single item from the queue with metrics tracking.
// Returns false when the queue is shutting down, true otherwise.
func (w *evictionWorker) processNextWorkItem(_ context.Context) bool {
	key, quit := w.queue.Get()
	if quit {
		return false
	}
	defer w.queue.Done(key)

	// Update queue metrics
	metrics.RecordEvictionQueueMetrics(w.name, float64(w.queue.Len()))

	// Get resource metadata for metrics
	var clusterName, resourceKind string
	if w.resourceKindFunc != nil {
		clusterName, resourceKind = w.resourceKindFunc(key)
	}

	// Process the item and measure latency
	startTime := time.Now()
	err := w.reconcileFunc(key)
	metrics.RecordEvictionProcessingMetrics(w.name, err, startTime)

	if err != nil {
		// Requeue with rate limiting on error
		w.queue.AddRateLimited(key)
		// Item remains in queue, so don't decrease metrics count
		return true
	}

	// Successfully processed
	w.queue.Forget(key)

	// Decrease resource kind count only after successful processing
	metrics.RecordEvictionKindMetrics(clusterName, resourceKind, false)

	// Apply pacing between items to enforce overall throughput, based on the
	// combined limiter (dynamic health-aware + default backoff/bucket).
	if w.pacer != nil {
		if delay := w.pacer.When(w.pacerKey); delay > 0 {
			time.Sleep(delay)
		}
		w.pacer.Forget(w.pacerKey)
	}

	return true
}

// Run starts worker goroutines and ensures cleanup when context is canceled.
func (w *evictionWorker) Run(ctx context.Context, workerNumber int) {
	klog.Infof("Starting %d workers for eviction worker %s", workerNumber, w.name)
	for i := 0; i < workerNumber; i++ {
		go w.worker(ctx)
	}

	// Clean up when context is canceled
	go func() {
		<-ctx.Done()
		klog.Infof("Shutting down eviction worker %s", w.name)
		w.queue.ShutDown()
	}()
}
