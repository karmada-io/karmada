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
	"github.com/karmada-io/karmada/pkg/util"
)

type evictionWorker struct {
	name             string
	keyFunc          util.KeyFunc
	reconcileFunc    util.ReconcileFunc
	resourceKindFunc func(key interface{}) (clusterName, resourceKind string)
	queue            workqueue.TypedRateLimitingInterface[any]
	// pacer is the combined limiter (dynamic + default) used to throttle the
	// processing throughput between items even when initial enqueues are immediate.
	pacer workqueue.TypedRateLimiter[any]
	// pacerKey is a sentinel key used for pacing. We call When/Forget on this key
	// for each successfully processed item to avoid exponential backoff accumulation.
	pacerKey any
}

// NewEvictionWorker creates a new EvictionWorker with dynamic rate limiting.
func NewEvictionWorker(opts EvictionWorkerOptions) util.AsyncWorker {
	rateLimiter := NewGracefulEvictionRateLimiter[any](
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
func (w *evictionWorker) processNextWorkItem(ctx context.Context) bool {
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
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
			}
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
