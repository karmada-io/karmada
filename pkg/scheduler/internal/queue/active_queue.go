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
type ActiveQueue interface {
	Push(bindingInfo *QueuedBindingInfo)
	Pop() (*QueuedBindingInfo, bool)
	Len() int
	Done(bindingInfo *QueuedBindingInfo)
	Has(key string) bool
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

// activequeue is a priority work queue, which implements a ActiveQueue.
type activequeue struct {
	// activeBindings defines the order in which we will work on items. Every
	// element of queue should be in the dirtyBindings set and not in the
	// processing set.
	activeBindings *heap.Heap[*QueuedBindingInfo]

	// dirtyBindings defines all of the items that need to be processed.
	dirtyBindings sets.Set[string]

	// Things that are currently being processed are in the processingBindings set.
	// These things may be simultaneously in the dirtyBindings set. When we finish
	// processingBindings something and remove it from this set, we'll check if
	// it's in the dirtyBindings set, and if so, add it to the queue.
	processingBindings sets.Set[string]

	cond *sync.Cond

	shuttingDown bool
}

// Push marks item as needing processing.
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

// Has inform if bindingInfo exists in the queue.
func (q *activequeue) Has(key string) bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return q.dirtyBindings.Has(key)
}
