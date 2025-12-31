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

package binder

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// DefaultBindWorkers is the default number of async bind workers
	DefaultBindWorkers = 32
	// DefaultBindQueueSize is the default size of the bind queue
	DefaultBindQueueSize = 10000
)

// BindResult represents the result of a scheduling decision to be bound
type BindResult struct {
	// Binding is the ResourceBinding to bind
	Binding *workv1alpha2.ResourceBinding
	// ClusterBinding is the ClusterResourceBinding to bind
	ClusterBinding *workv1alpha2.ClusterResourceBinding
	// ScheduleResult is the scheduling result (target clusters)
	ScheduleResult []workv1alpha2.TargetCluster
	// Placement is the serialized placement
	Placement string
	// Condition is the scheduling condition to set
	Condition metav1.Condition
	// AffinityName is the observed affinity name (optional)
	AffinityName string
	// Timestamp is when the scheduling decision was made
	Timestamp time.Time
}

// AsyncBinder handles asynchronous binding of scheduling results
type AsyncBinder struct {
	karmadaClient karmadaclientset.Interface
	eventRecorder record.EventRecorder

	// bindQueue is the channel for bind requests
	bindQueue chan *BindResult

	// assumeCache tracks assumed bindings (scheduled but not yet persisted)
	assumeCache *AssumeCache

	// workers is the number of concurrent bind workers
	workers int
}

// AssumeCache tracks bindings that have been scheduled but not yet persisted
type AssumeCache struct {
	mu sync.RWMutex
	// assumed tracks binding keys that are assumed (scheduled but not persisted)
	assumed map[string]*BindResult
}

// NewAssumeCache creates a new AssumeCache
func NewAssumeCache() *AssumeCache {
	return &AssumeCache{
		assumed: make(map[string]*BindResult),
	}
}

// Assume marks a binding as assumed
func (c *AssumeCache) Assume(key string, result *BindResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.assumed[key] = result
	klog.V(4).Infof("Assumed binding %s", key)
}

// Confirm removes a binding from assumed state (successfully persisted)
func (c *AssumeCache) Confirm(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.assumed, key)
	klog.V(4).Infof("Confirmed binding %s", key)
}

// Forget removes a binding from assumed state (failed to persist)
func (c *AssumeCache) Forget(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.assumed, key)
	klog.V(4).Infof("Forgot binding %s", key)
}

// IsAssumed checks if a binding is in assumed state
func (c *AssumeCache) IsAssumed(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.assumed[key]
	return ok
}

// Get returns the assumed bind result for a key
func (c *AssumeCache) Get(key string) (*BindResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.assumed[key]
	return result, ok
}

// NewAsyncBinder creates a new AsyncBinder
func NewAsyncBinder(karmadaClient karmadaclientset.Interface, eventRecorder record.EventRecorder, workers int) *AsyncBinder {
	if workers <= 0 {
		workers = DefaultBindWorkers
	}
	return &AsyncBinder{
		karmadaClient: karmadaClient,
		eventRecorder: eventRecorder,
		bindQueue:     make(chan *BindResult, DefaultBindQueueSize),
		assumeCache:   NewAssumeCache(),
		workers:       workers,
	}
}

// Run starts the async binder workers
func (b *AsyncBinder) Run(ctx context.Context) {
	klog.Infof("Starting async binder with %d workers", b.workers)
	for i := 0; i < b.workers; i++ {
		go b.bindWorker(ctx, i)
	}
	<-ctx.Done()
	klog.Info("Shutting down async binder")
}

// bindWorker processes bind requests from the queue
func (b *AsyncBinder) bindWorker(ctx context.Context, workerID int) {
	klog.V(4).Infof("Bind worker %d started", workerID)
	for {
		select {
		case <-ctx.Done():
			klog.V(4).Infof("Bind worker %d stopped", workerID)
			return
		case result := <-b.bindQueue:
			b.doBind(ctx, result)
		}
	}
}

// AsyncBind submits a bind request to be processed asynchronously
func (b *AsyncBinder) AsyncBind(result *BindResult) error {
	key := b.getBindingKey(result)

	// Mark as assumed immediately
	b.assumeCache.Assume(key, result)
	result.Timestamp = time.Now()

	// Submit to queue (non-blocking with timeout)
	select {
	case b.bindQueue <- result:
		klog.V(4).Infof("Submitted async bind for %s", key)
		return nil
	default:
		// Queue is full, remove from assume cache and return error
		b.assumeCache.Forget(key)
		return fmt.Errorf("bind queue is full, cannot submit bind for %s", key)
	}
}

// getBindingKey returns a unique key for the binding
func (b *AsyncBinder) getBindingKey(result *BindResult) string {
	if result.Binding != nil {
		return fmt.Sprintf("%s/%s", result.Binding.Namespace, result.Binding.Name)
	}
	if result.ClusterBinding != nil {
		return result.ClusterBinding.Name
	}
	return ""
}

// doBind performs the actual binding operation
func (b *AsyncBinder) doBind(ctx context.Context, result *BindResult) {
	key := b.getBindingKey(result)
	start := time.Now()

	var err error
	if result.Binding != nil {
		err = b.bindResourceBinding(ctx, result)
	} else if result.ClusterBinding != nil {
		err = b.bindClusterResourceBinding(ctx, result)
	}

	latency := time.Since(start)
	if err != nil {
		klog.Errorf("Failed to bind %s after %v: %v", key, latency, err)
		b.assumeCache.Forget(key)
		// TODO: Consider requeueing for retry
	} else {
		klog.V(4).Infof("Successfully bound %s in %v (queue wait: %v)", key, latency, start.Sub(result.Timestamp))
		b.assumeCache.Confirm(key)
	}
}

// bindResourceBinding binds a ResourceBinding
func (b *AsyncBinder) bindResourceBinding(ctx context.Context, result *BindResult) error {
	rb := result.Binding

	// Step 1: Patch spec.clusters
	if err := b.patchResourceBindingSpec(ctx, rb, result.Placement, result.ScheduleResult); err != nil {
		b.recordScheduleResultEvent(rb, nil, result.ScheduleResult, err)
		return err
	}

	// Step 2: Patch status
	if err := b.patchResourceBindingStatus(ctx, rb, result.Condition, result.AffinityName); err != nil {
		return err
	}

	return nil
}

// bindClusterResourceBinding binds a ClusterResourceBinding
func (b *AsyncBinder) bindClusterResourceBinding(ctx context.Context, result *BindResult) error {
	crb := result.ClusterBinding

	// Step 1: Patch spec.clusters
	if err := b.patchClusterResourceBindingSpec(ctx, crb, result.Placement, result.ScheduleResult); err != nil {
		b.recordScheduleResultEvent(nil, crb, result.ScheduleResult, err)
		return err
	}

	// Step 2: Patch status
	if err := b.patchClusterResourceBindingStatus(ctx, crb, result.Condition, result.AffinityName); err != nil {
		return err
	}

	return nil
}

func (b *AsyncBinder) patchResourceBindingSpec(ctx context.Context, rb *workv1alpha2.ResourceBinding, placement string, clusters []workv1alpha2.TargetCluster) error {
	newBinding := rb.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = clusters

	patchBytes, err := helper.GenMergePatch(rb, newBinding)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = b.karmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Patch(
		ctx, rb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Failed to patch spec for ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
		return err
	}
	return nil
}

func (b *AsyncBinder) patchResourceBindingStatus(ctx context.Context, rb *workv1alpha2.ResourceBinding, condition metav1.Condition, affinityName string) error {
	updateRB := rb.DeepCopy()
	meta.SetStatusCondition(&updateRB.Status.Conditions, condition)
	if condition.Status == metav1.ConditionTrue {
		updateRB.Status.SchedulerObservedGeneration = rb.Generation
		currentTime := metav1.Now()
		updateRB.Status.LastScheduledTime = &currentTime
	}
	if affinityName != "" {
		updateRB.Status.SchedulerObservedAffinityName = affinityName
	}

	if reflect.DeepEqual(rb.Status, updateRB.Status) {
		return nil
	}

	patchBytes, err := helper.GenFieldMergePatch("status", rb.Status, updateRB.Status)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = b.karmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Patch(
		ctx, rb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch status for ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
		return err
	}
	return nil
}

func (b *AsyncBinder) patchClusterResourceBindingSpec(ctx context.Context, crb *workv1alpha2.ClusterResourceBinding, placement string, clusters []workv1alpha2.TargetCluster) error {
	newBinding := crb.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = clusters

	patchBytes, err := helper.GenMergePatch(crb, newBinding)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = b.karmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(
		ctx, crb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Failed to patch spec for ClusterResourceBinding(%s): %v", crb.Name, err)
		return err
	}
	return nil
}

func (b *AsyncBinder) patchClusterResourceBindingStatus(ctx context.Context, crb *workv1alpha2.ClusterResourceBinding, condition metav1.Condition, affinityName string) error {
	updateCRB := crb.DeepCopy()
	meta.SetStatusCondition(&updateCRB.Status.Conditions, condition)
	if condition.Status == metav1.ConditionTrue {
		updateCRB.Status.SchedulerObservedGeneration = crb.Generation
		currentTime := metav1.Now()
		updateCRB.Status.LastScheduledTime = &currentTime
	}
	if affinityName != "" {
		updateCRB.Status.SchedulerObservedAffinityName = affinityName
	}

	if reflect.DeepEqual(crb.Status, updateCRB.Status) {
		return nil
	}

	patchBytes, err := helper.GenFieldMergePatch("status", crb.Status, updateCRB.Status)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = b.karmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(
		ctx, crb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch status for ClusterResourceBinding(%s): %v", crb.Name, err)
		return err
	}
	return nil
}

func (b *AsyncBinder) recordScheduleResultEvent(rb *workv1alpha2.ResourceBinding, crb *workv1alpha2.ClusterResourceBinding, _ []workv1alpha2.TargetCluster, err error) {
	// Performance optimization: Only record events for failures.
	// Success events create significant API load in high-throughput scenarios (10000+ resources).
	if err == nil {
		return
	}

	// Skip if eventRecorder is nil to prevent panic
	if b.eventRecorder == nil {
		klog.V(4).Infof("eventRecorder is nil, skipping event recording")
		return
	}

	msg := err.Error()

	if rb != nil {
		// Record event on ResourceBinding - use defer/recover to prevent panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					klog.Warningf("Recovered from panic while recording event for ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, r)
				}
			}()
			b.eventRecorder.Event(rb, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, msg)
		}()

		// Only record event on resource template if UID is set and Name is not empty
		if rb.Spec.Resource.UID != "" && rb.Spec.Resource.Name != "" {
			func() {
				defer func() {
					if r := recover(); r != nil {
						klog.Warningf("Recovered from panic while recording event for resource template %s/%s: %v", rb.Spec.Resource.Namespace, rb.Spec.Resource.Name, r)
					}
				}()
				ref := &corev1.ObjectReference{
					Kind:       rb.Spec.Resource.Kind,
					APIVersion: rb.Spec.Resource.APIVersion,
					Namespace:  rb.Spec.Resource.Namespace,
					Name:       rb.Spec.Resource.Name,
					UID:        rb.Spec.Resource.UID,
				}
				b.eventRecorder.Event(ref, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, msg)
			}()
		}
	}

	if crb != nil {
		// Record event on ClusterResourceBinding - use defer/recover to prevent panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					klog.Warningf("Recovered from panic while recording event for ClusterResourceBinding %s: %v", crb.Name, r)
				}
			}()
			b.eventRecorder.Event(crb, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, msg)
		}()

		// Only record event on resource template if UID is set and Name is not empty
		if crb.Spec.Resource.UID != "" && crb.Spec.Resource.Name != "" {
			func() {
				defer func() {
					if r := recover(); r != nil {
						klog.Warningf("Recovered from panic while recording event for resource template %s/%s: %v", crb.Spec.Resource.Namespace, crb.Spec.Resource.Name, r)
					}
				}()
				ref := &corev1.ObjectReference{
					Kind:       crb.Spec.Resource.Kind,
					APIVersion: crb.Spec.Resource.APIVersion,
					Namespace:  crb.Spec.Resource.Namespace,
					Name:       crb.Spec.Resource.Name,
					UID:        crb.Spec.Resource.UID,
				}
				b.eventRecorder.Event(ref, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, msg)
			}()
		}
	}
}

// GetAssumeCache returns the assume cache for external use
func (b *AsyncBinder) GetAssumeCache() *AssumeCache {
	return b.assumeCache
}

// QueueLength returns the current length of the bind queue
func (b *AsyncBinder) QueueLength() int {
	return len(b.bindQueue)
}

// MarshalPlacement serializes a placement to JSON string
func MarshalPlacement(placement interface{}) (string, error) {
	bytes, err := json.Marshal(placement)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
