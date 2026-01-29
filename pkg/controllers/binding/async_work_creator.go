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

package binding

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
)

const (
	// DefaultAsyncWorkWorkers is the default number of async work creation workers
	DefaultAsyncWorkWorkers = 64
	// DefaultAsyncWorkQueueSize is the default size of the work creation queue
	DefaultAsyncWorkQueueSize = 10000
	// DefaultAssumeCacheCleanupInterval is the interval for cleaning up stale assume cache entries
	DefaultAssumeCacheCleanupInterval = 5 * time.Minute
	// DefaultAssumeCacheMaxAge is the maximum age for assume cache entries before cleanup
	DefaultAssumeCacheMaxAge = 10 * time.Minute
)

// RequeueFunc is a callback function to trigger requeue of a binding when work creation fails
type RequeueFunc func(bindingKey string)

// WorkTask represents a work creation task
type WorkTask struct {
	// BindingKey is the namespace/name of the ResourceBinding
	BindingKey string
	// WorkMeta contains the metadata for the Work object
	WorkMeta metav1.ObjectMeta
	// Workload is the resource to be distributed
	Workload *unstructured.Unstructured
	// Options for work creation
	SuspendDispatching          *bool
	PreserveResourcesOnDeletion *bool
	// Binding reference for event recording
	Binding client.Object
	// Timestamp when task was created
	Timestamp time.Time
}

// AsyncWorkCreator handles asynchronous creation of Work objects
// This follows the kube-scheduler pattern of separating scheduling decisions
// from the actual binding (persistence) operation.
type AsyncWorkCreator struct {
	client        client.Client
	eventRecorder record.EventRecorder

	// workQueue is the channel for work creation tasks
	workQueue chan *WorkTask

	// assumeCache tracks works that are assumed (submitted but not yet persisted)
	assumeCache *WorkAssumeCache

	// workers is the number of concurrent work creation workers
	workers int

	// requeueFunc is called when work creation fails to trigger binding re-reconcile
	requeueFunc RequeueFunc

	// metrics
	queuedTasks  int64
	createdWorks int64
	failedWorks  int64
	mu           sync.Mutex
}

// WorkAssumeCache tracks Work objects that have been scheduled for creation
// but not yet persisted to the API server. This allows the binding controller
// to skip re-processing bindings that are already being handled.
type WorkAssumeCache struct {
	mu sync.RWMutex
	// assumed maps binding key to list of work keys being created
	assumed map[string]*AssumedWorkSet
}

// AssumedWorkSet tracks the set of works being created for a binding
type AssumedWorkSet struct {
	works     map[string]*WorkTask // workKey -> task
	timestamp time.Time
}

// NewWorkAssumeCache creates a new WorkAssumeCache
func NewWorkAssumeCache() *WorkAssumeCache {
	return &WorkAssumeCache{
		assumed: make(map[string]*AssumedWorkSet),
	}
}

// Assume marks works for a binding as assumed (pending creation)
func (c *WorkAssumeCache) Assume(bindingKey string, tasks []*WorkTask) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workSet := &AssumedWorkSet{
		works:     make(map[string]*WorkTask),
		timestamp: time.Now(),
	}
	for _, task := range tasks {
		workKey := fmt.Sprintf("%s/%s", task.WorkMeta.Namespace, task.WorkMeta.Name)
		workSet.works[workKey] = task
	}
	c.assumed[bindingKey] = workSet
	klog.V(4).Infof("Assumed %d works for binding %s", len(tasks), bindingKey)
}

// IsAssumed checks if a binding has assumed works pending creation
func (c *WorkAssumeCache) IsAssumed(bindingKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.assumed[bindingKey]
	return ok
}

// ConfirmWork marks a single work as successfully created
func (c *WorkAssumeCache) ConfirmWork(bindingKey, workKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if workSet, ok := c.assumed[bindingKey]; ok {
		delete(workSet.works, workKey)
		// If all works for this binding are confirmed, remove the binding entry
		if len(workSet.works) == 0 {
			delete(c.assumed, bindingKey)
			klog.V(4).Infof("All works confirmed for binding %s", bindingKey)
		}
	}
}

// ForgetWork marks a work creation as failed, removing it from assumed state
func (c *WorkAssumeCache) ForgetWork(bindingKey, workKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if workSet, ok := c.assumed[bindingKey]; ok {
		delete(workSet.works, workKey)
		// If all works are processed (confirmed or forgotten), remove binding entry
		if len(workSet.works) == 0 {
			delete(c.assumed, bindingKey)
		}
	}
	klog.V(4).Infof("Forgot work %s for binding %s", workKey, bindingKey)
}

// ForgetBinding removes all assumed works for a binding (used when binding is deleted or needs full re-sync)
func (c *WorkAssumeCache) ForgetBinding(bindingKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.assumed, bindingKey)
}

// GetPendingCount returns the number of pending works for a binding
func (c *WorkAssumeCache) GetPendingCount(bindingKey string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if workSet, ok := c.assumed[bindingKey]; ok {
		return len(workSet.works)
	}
	return 0
}

// CleanupStale removes stale entries from the assume cache that have exceeded maxAge.
// This prevents memory leaks when bindings are deleted while works are still assumed.
func (c *WorkAssumeCache) CleanupStale(maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleaned := 0
	now := time.Now()
	for key, workSet := range c.assumed {
		if now.Sub(workSet.timestamp) > maxAge {
			delete(c.assumed, key)
			cleaned++
			klog.V(4).Infof("Cleaned up stale assume cache entry for binding %s (age: %v)", key, now.Sub(workSet.timestamp))
		}
	}
	return cleaned
}

// Size returns the number of bindings in the assume cache
func (c *WorkAssumeCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.assumed)
}

// NewAsyncWorkCreator creates a new AsyncWorkCreator
// requeueFunc is called when work creation fails to trigger the binding to be re-reconciled
func NewAsyncWorkCreator(c client.Client, eventRecorder record.EventRecorder, workers int, requeueFunc RequeueFunc) *AsyncWorkCreator {
	if workers <= 0 {
		workers = DefaultAsyncWorkWorkers
	}
	return &AsyncWorkCreator{
		client:        c,
		eventRecorder: eventRecorder,
		workQueue:     make(chan *WorkTask, DefaultAsyncWorkQueueSize),
		assumeCache:   NewWorkAssumeCache(),
		workers:       workers,
		requeueFunc:   requeueFunc,
	}
}

// Run starts the async work creation workers and the assume cache cleanup goroutine
func (a *AsyncWorkCreator) Run(ctx context.Context) {
	klog.Infof("Starting async work creator with %d workers", a.workers)
	for i := 0; i < a.workers; i++ {
		go a.worker(ctx, i)
	}

	// Start assume cache cleanup goroutine
	go a.runAssumeCacheCleanup(ctx)

	<-ctx.Done()
	klog.Info("Shutting down async work creator")
}

// runAssumeCacheCleanup periodically cleans up stale entries from the assume cache
func (a *AsyncWorkCreator) runAssumeCacheCleanup(ctx context.Context) {
	ticker := time.NewTicker(DefaultAssumeCacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleaned := a.assumeCache.CleanupStale(DefaultAssumeCacheMaxAge)
			if cleaned > 0 {
				klog.V(2).Infof("Cleaned up %d stale assume cache entries, remaining: %d", cleaned, a.assumeCache.Size())
			}
		}
	}
}

// worker processes work creation tasks from the queue
func (a *AsyncWorkCreator) worker(ctx context.Context, workerID int) {
	klog.V(4).Infof("Async work creator worker %d started", workerID)
	for {
		select {
		case <-ctx.Done():
			klog.V(4).Infof("Async work creator worker %d stopped", workerID)
			return
		case task := <-a.workQueue:
			a.createWork(ctx, task)
		}
	}
}

// Submit submits work creation tasks for a binding
// Returns error if queue is full
func (a *AsyncWorkCreator) Submit(bindingKey string, tasks []*WorkTask) error {
	if len(tasks) == 0 {
		return nil
	}

	// Mark as assumed before submitting to queue
	a.assumeCache.Assume(bindingKey, tasks)

	// Submit all tasks to queue
	for _, task := range tasks {
		task.BindingKey = bindingKey
		task.Timestamp = time.Now()

		select {
		case a.workQueue <- task:
			klog.V(4).Infof("Submitted work creation task for %s/%s", task.WorkMeta.Namespace, task.WorkMeta.Name)
		default:
			// Queue is full, forget all remaining tasks for this binding
			a.assumeCache.ForgetBinding(bindingKey)
			return fmt.Errorf("work queue is full, cannot submit tasks for binding %s", bindingKey)
		}
	}

	a.mu.Lock()
	a.queuedTasks += int64(len(tasks))
	a.mu.Unlock()

	return nil
}

// IsAssumed checks if a binding has pending work creation tasks
func (a *AsyncWorkCreator) IsAssumed(bindingKey string) bool {
	return a.assumeCache.IsAssumed(bindingKey)
}

// ForgetBinding removes assumed state for a binding (e.g., when binding is deleted)
func (a *AsyncWorkCreator) ForgetBinding(bindingKey string) {
	a.assumeCache.ForgetBinding(bindingKey)
}

// createWork performs the actual work creation
func (a *AsyncWorkCreator) createWork(ctx context.Context, task *WorkTask) {
	workKey := fmt.Sprintf("%s/%s", task.WorkMeta.Namespace, task.WorkMeta.Name)
	start := time.Now()

	// Build options
	var options []ctrlutil.WorkOption
	if task.SuspendDispatching != nil {
		options = append(options, ctrlutil.WithSuspendDispatching(*task.SuspendDispatching))
	}
	if task.PreserveResourcesOnDeletion != nil {
		options = append(options, ctrlutil.WithPreserveResourcesOnDeletion(*task.PreserveResourcesOnDeletion))
	}

	err := ctrlutil.CreateOrUpdateWork(ctx, a.client, task.WorkMeta, task.Workload, options...)

	latency := time.Since(start)
	queueWait := start.Sub(task.Timestamp)

	if err != nil {
		klog.Errorf("Failed to create work %s after %v (queue wait: %v): %v", workKey, latency, queueWait, err)
		a.assumeCache.ForgetWork(task.BindingKey, workKey)

		a.mu.Lock()
		a.failedWorks++
		a.mu.Unlock()

		// Record failure event
		if a.eventRecorder != nil && task.Binding != nil {
			a.eventRecorder.Event(task.Binding, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		}

		// Trigger requeue to retry the binding reconciliation
		// This ensures the binding will be re-processed and work creation will be retried
		if a.requeueFunc != nil {
			klog.V(2).Infof("Triggering requeue for binding %s due to work creation failure", task.BindingKey)
			a.requeueFunc(task.BindingKey)
		}
		return
	}

	klog.V(4).Infof("Successfully created work %s in %v (queue wait: %v)", workKey, latency, queueWait)
	a.assumeCache.ConfirmWork(task.BindingKey, workKey)

	a.mu.Lock()
	a.createdWorks++
	a.mu.Unlock()

	// Record metrics
	metrics.ObserveSyncWorkLatency(nil, start)
}

// GetStats returns statistics about the async work creator
func (a *AsyncWorkCreator) GetStats() (queued, created, failed int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queuedTasks, a.createdWorks, a.failedWorks
}

// QueueLength returns the current queue length
func (a *AsyncWorkCreator) QueueLength() int {
	return len(a.workQueue)
}
