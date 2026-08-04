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

package queue

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/karmada-io/karmada/pkg/scheduler/internal/heap"
	metrics "github.com/karmada-io/karmada/pkg/scheduler/metrics/queue"
)

// ActiveQueue defines the interface of activeQ related operations.
// It is a priority work queue for bindings waiting to be scheduled:
// bindings are popped in priority order, duplicates are ignored, and
// the same binding is never processed by two workers at the same time.
type ActiveQueue interface {
	// Push adds a binding to the queue, marking it as needing processing.
	// Pushing a binding that is already queued is a no-op. Pushing a binding
	// that is being processed only marks it dirty, so it will be re-queued
	// automatically when Done is called.
	Push(bindingInfo *QueuedBindingInfo)

	// Pop blocks until a binding is available, then returns the one with the
	// highest priority. It returns shutdown = true if the queue is shutting
	// down and the caller should exit. The caller must call Done with the
	// returned binding after finishing processing it.
	Pop() (*QueuedBindingInfo, bool)

	// Len returns the number of bindings currently waiting in the queue,
	// for informational purposes only (e.g. metrics or logging).
	Len() int

	// Done tells the queue that processing of the binding has finished.
	// If the binding was pushed again while being processed, it will be
	// re-added to the queue for re-processing.
	Done(bindingInfo *QueuedBindingInfo)

	// Has returns true if the binding is marked as needing processing,
	// i.e. it is waiting in the queue, or it was updated while being processed.
	Has(key string) bool

	// ShutDown shuts down the queue: new Push calls are ignored, and blocked
	// Pop callers return immediately with shutdown = true.
	ShutDown()
}

// NewActiveQueue builds a instance of ActiveQueue.
func NewActiveQueue(metricRecorder metrics.MetricRecorder) ActiveQueue {
	q := &activequeue{
		activeBindings:     heap.NewWithRecorder[*QueuedBindingInfo](BindingKeyFunc, Less, metricRecorder),
		dirtyBindings:      sets.Set[string]{},
		processingBindings: sets.Set[string]{},
		cond:               sync.NewCond(&sync.Mutex{}),
	}

	return q
}

// activequeue implements ActiveQueue. It is a priority work queue: bindings
// are popped in priority order (highest first) instead of FIFO order.
// Besides ordering, it gives three guarantees:
//   - No duplicates: the same binding is never queued twice (dirtyBindings).
//   - No concurrent processing: the same binding is never handled by two
//     workers at the same time (processingBindings).
//   - No lost updates: a binding pushed while being processed will be
//     re-queued exactly once after Done() is called.
type activequeue struct {
	// activeBindings is a priority heap that holds bindings waiting to be
	// scheduled, ordered by priority. Pop() always returns the binding with
	// the highest priority. Every element in this heap must also be in the
	// dirtyBindings set, and must not be in the processingBindings set.
	activeBindings *heap.Heap[*QueuedBindingInfo]

	// dirtyBindings marks all bindings that need to be processed. It is used
	// for deduplication: pushing a binding that is already dirty is a no-op,
	// so the same binding never shows up in the queue twice. It also remembers
	// bindings that got updated while being processed, so they can be re-queued
	// once Done() is called.
	dirtyBindings sets.Set[string]

	// processingBindings holds bindings that are currently being processed by
	// scheduler workers. It prevents the same binding from being scheduled by
	// two workers at the same time: while a binding is in this set, a new Push()
	// only marks it dirty instead of adding it to the heap. When Done() removes
	// a binding from this set, it is re-added to the heap if it is still dirty.
	processingBindings sets.Set[string]

	// cond protects all fields above and is used to wake up Pop() callers
	// that are blocked waiting for a binding to arrive.
	cond *sync.Cond

	// shuttingDown, when true, makes Push() a no-op and causes blocked Pop()
	// callers to return with shutdown = true.
	shuttingDown bool
}

// Push marks the binding as needing processing. If the binding is already
// queued (dirty), this is a no-op. If the binding is currently being
// processed, it is only marked dirty and will be re-queued by Done().
func (q *activequeue) Push(bindingInfo *QueuedBindingInfo) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown {
		return
	}
	if q.dirtyBindings.Has(bindingInfo.NamespacedKey) {
		return
	}

	now := time.Now()
	bindingInfo.Timestamp = now
	if bindingInfo.InitialAttemptTimestamp == nil {
		bindingInfo.InitialAttemptTimestamp = &now
	}
	q.dirtyBindings.Insert(bindingInfo.NamespacedKey)
	if q.processingBindings.Has(bindingInfo.NamespacedKey) {
		return
	}

	q.activeBindings.AddOrUpdate(bindingInfo)
	q.cond.Signal()
}

// Len returns the current queue length, for informational purposes only. You
// shouldn't e.g. gate a call to Push() or Pop() on Len() being a particular
// value, that can't be synchronized properly.
func (q *activequeue) Len() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return q.activeBindings.Len()
}

// Pop blocks until it can return an item to be processed. If shutdown = true,
// the caller should end their goroutine. You must call Done with item when you
// have finished processing it.
func (q *activequeue) Pop() (bindingInfo *QueuedBindingInfo, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for q.activeBindings.Len() == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	if q.activeBindings.Len() == 0 {
		// We must be shutting down.
		return nil, true
	}

	bindingInfo, _ = q.activeBindings.Pop()
	bindingInfo.Attempts++
	q.processingBindings.Insert(bindingInfo.NamespacedKey)
	q.dirtyBindings.Delete(bindingInfo.NamespacedKey)

	return bindingInfo, false
}

// Done marks item as done processing, and if it has been marked as dirty again
// while it was being processed, it will be re-added to the queue for
// re-processing.
func (q *activequeue) Done(bindingInfo *QueuedBindingInfo) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.processingBindings.Delete(bindingInfo.NamespacedKey)
	if q.dirtyBindings.Has(bindingInfo.NamespacedKey) {
		bindingInfo.Timestamp = time.Now()
		q.activeBindings.AddOrUpdate(bindingInfo)
		q.cond.Signal()
	} else if q.processingBindings.Len() == 0 {
		q.cond.Signal()
	}
}

// ShutDown will cause q to ignore all new items added to it and
// immediately instruct the worker goroutines to exit.
func (q *activequeue) ShutDown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.shuttingDown = true
	q.cond.Broadcast()
}

// Has tells if binding exists in the queue.
func (q *activequeue) Has(key string) bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return q.dirtyBindings.Has(key)
}
