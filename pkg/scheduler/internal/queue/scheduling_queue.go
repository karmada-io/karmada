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

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/scheduler/internal/heap"
	metrics "github.com/karmada-io/karmada/pkg/scheduler/metrics/queue"
)

const (
	// Scheduling queue names
	activeQ               = "Active"
	backoffQ              = "Backoff"
	unschedulableBindings = "Unschedulable"
)

const (
	// DefaultBindingMaxInUnschedulableBindingsDuration is the default value for the maximum
	// time a binding can stay in unschedulableBindings. If a binding stays in unschedulableBindings
	// for longer than this value, the binding will be moved from unschedulableBindings to
	// backoffQ or activeQ. If this value is empty, the default value (5min)
	// will be used.
	DefaultBindingMaxInUnschedulableBindingsDuration = 5 * time.Minute

	// DefaultBindingInitialBackoffDuration is the default value for the initial backoff duration
	// for unschedulable bindings.
	DefaultBindingInitialBackoffDuration = 1 * time.Second

	// DefaultBindingMaxBackoffDuration is the default value for the max backoff duration
	// for unschedulable bindings.
	DefaultBindingMaxBackoffDuration = 10 * time.Second
)

// SchedulingQueue is an interface for a queue to store bindings waiting to be scheduled.
// The interface follows a pattern similar to cache.FIFO and cache.Heap and
// makes it easy to use those data structures as a SchedulingQueue.
type SchedulingQueue interface {
	// Push pushes an new binding to activeQ.
	Push(bindingInfo *QueuedBindingInfo)

	// PushUnschedulableIfNotPresent pushes an unschedulable binding back to scheduling queue.
	PushUnschedulableIfNotPresent(bindingInfo *QueuedBindingInfo)

	// PushBackoffIfNotPresent pushes an failed binding back to scheduling queue.
	PushBackoffIfNotPresent(bindingInfo *QueuedBindingInfo)

	// Pop removes the head of the queue and returns it. It blocks if the
	// queue is empty and waits until a new binding is added to the queue.
	Pop() (*QueuedBindingInfo, bool)

	// Done must be called for binding returned by Pop. This allows the queue to
	// keep track of which bindings are currently being processed.
	Done(bindingInfo *QueuedBindingInfo)

	// Len returns the length of activeQ.
	Len() int

	// Forget indicates that an item is finished being retried.  Doesn't matter whether it's for perm failing
	// or for success, we'll remove it from backoffQ, but you still have to call `Done` on the queue.
	Forget(bindingInfo *QueuedBindingInfo)

	// Run starts the goroutines managing the queue.
	Run()

	// Close closes the SchedulingQueue so that the goroutine which is
	// waiting to pop items can exit gracefully.
	Close()
}

type schedulingQueueOptions struct {
	bindingInitialBackoffDuration             time.Duration
	bindingMaxBackoffDuration                 time.Duration
	bindingMaxInUnschedulableBindingsDuration time.Duration
}

// Option configures a PriorityQueue
type Option func(*schedulingQueueOptions)

// WithBindingInitialBackoffDuration sets binding initial backoff duration for SchedulingQueue.
func WithBindingInitialBackoffDuration(duration time.Duration) Option {
	return func(o *schedulingQueueOptions) {
		o.bindingInitialBackoffDuration = duration
	}
}

// WithBindingMaxBackoffDuration sets binding max backoff duration for SchedulingQueue.
func WithBindingMaxBackoffDuration(duration time.Duration) Option {
	return func(o *schedulingQueueOptions) {
		o.bindingMaxBackoffDuration = duration
	}
}

// WithBindingMaxInUnschedulableBindingsDuration sets bindingMaxInUnschedulableBindingsDuration for SchedulingQueue.
func WithBindingMaxInUnschedulableBindingsDuration(duration time.Duration) Option {
	return func(o *schedulingQueueOptions) {
		o.bindingMaxInUnschedulableBindingsDuration = duration
	}
}

var defaultSchedulingQueueOptions = schedulingQueueOptions{
	bindingInitialBackoffDuration:             DefaultBindingInitialBackoffDuration,
	bindingMaxBackoffDuration:                 DefaultBindingMaxBackoffDuration,
	bindingMaxInUnschedulableBindingsDuration: DefaultBindingMaxInUnschedulableBindingsDuration,
}

// NewSchedulingQueue builds a SchedulingQueue instance.
func NewSchedulingQueue(opts ...Option) SchedulingQueue {
	options := defaultSchedulingQueueOptions
	for _, opt := range opts {
		opt(&options)
	}

	bq := &prioritySchedulingQueue{
		stop:                          make(chan struct{}),
		bindingInitialBackoffDuration: options.bindingInitialBackoffDuration,
		bindingMaxBackoffDuration:     options.bindingMaxBackoffDuration,
		bindingMaxInUnschedulableBindingsDuration: options.bindingMaxInUnschedulableBindingsDuration,
		activeQ:               NewActiveQueue(metrics.NewActiveBindingsRecorder()),
		backoffQ:              heap.NewWithRecorder(BindingKeyFunc, Less, metrics.NewBackoffBindingsRecorder()),
		unschedulableBindings: newUnschedulableBindings(metrics.NewUnschedulableBindingsRecorder()),
	}

	return bq
}

// prioritySchedulingQueue implements a SchedulingQueue.
// The head of PriorityQueue is the highest priority pending binding. This structure
// has two sub queues and a additional data structure, namely: activeQ,
// backoffQ and unschedulableBindings.
//   - activeQ holds bindings that are being considered for scheduling.
//   - backoffQ holds bindings that moved from unschedulableBindings and will move to
//     activeQ when their backoff periods complete.
//   - unschedulableBindings holds bindings that were already attempted for scheduling and
//     are currently determined to be unschedulable.
type prioritySchedulingQueue struct {
	// stop is used to stop the goroutine which periodically flushes pending bindings.
	stop chan struct{}
	// lock takes precedence and should be taken first,
	// before any other locks in the queue.
	lock sync.RWMutex

	// binding initial backoff duration.
	bindingInitialBackoffDuration time.Duration
	// binding maximum backoff duration.
	bindingMaxBackoffDuration time.Duration
	// the maximum time a binding can stay in the unschedulableBindings.
	bindingMaxInUnschedulableBindingsDuration time.Duration

	// activeQ is a priority queue ordered by priority
	activeQ ActiveQueue
	// backoffQ is a heap ordered by backoff expiry. Bindings which have completed backoff
	// are popped from this heap before the scheduler looks at activeQ.
	backoffQ *heap.Heap[*QueuedBindingInfo]
	// unschedulableBindings holds bindings that have been tried and determined unschedulable.
	unschedulableBindings *UnschedulableBindings
}

// Run starts the goroutine to flush backoffQ and unschedulableBindings.
func (bq *prioritySchedulingQueue) Run() {
	go wait.Until(bq.flushBackoffQCompleted, 1.0*time.Second, bq.stop)
	go wait.Until(bq.flushUnschedulableBindingsLeftover, 30*time.Second, bq.stop)
}

// Close closes the scheduling queue.
func (bq *prioritySchedulingQueue) Close() {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	close(bq.stop)
	bq.activeQ.ShutDown()
	bq.unschedulableBindings.clear()
}

// flushBackoffQCompleted moves all bindings from backoffQ which have completed backoff in to activeQ
func (bq *prioritySchedulingQueue) flushBackoffQCompleted() {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	for {
		bInfo, ok := bq.backoffQ.Peek()
		if !ok || bInfo == nil {
			break
		}
		if bq.isBindingBackingoff(bInfo) {
			break
		}
		_, err := bq.backoffQ.Pop()
		if err != nil {
			klog.Error(err, "Unable to pop binding from backoff queue despite backoff completion", "binding", bInfo.NamespacedKey)
			break
		}
		bq.moveToActiveQ(bInfo)
	}
}

// isBindingBackingoff returns true if a binding is still waiting for its backoff timer.
// If this returns true, the binding should not be re-tried.
func (bq *prioritySchedulingQueue) isBindingBackingoff(bindingInfo *QueuedBindingInfo) bool {
	boTime := bq.getBackoffTime(bindingInfo)
	return boTime.After(time.Now())
}

// calculateBackoffDuration is a helper function for calculating the backoffDuration
// based on the number of attempts the binding has made.
func (bq *prioritySchedulingQueue) calculateBackoffDuration(bindingInfo *QueuedBindingInfo) time.Duration {
	if bindingInfo.Attempts == 0 {
		// When the Binding hasn't experienced any scheduling attempts,
		// they aren't obliged to get a backoff penalty at all.
		return 0
	}

	duration := bq.bindingInitialBackoffDuration
	for i := 1; i < bindingInfo.Attempts; i++ {
		// Use subtraction instead of addition or multiplication to avoid overflow.
		if duration > bq.bindingMaxBackoffDuration-duration {
			return bq.bindingMaxBackoffDuration
		}
		duration += duration
	}
	return duration
}

// getBackoffTime returns the time that bindingInfo completes backoff
func (bq *prioritySchedulingQueue) getBackoffTime(bindingInfo *QueuedBindingInfo) time.Time {
	duration := bq.calculateBackoffDuration(bindingInfo)
	backoffTime := bindingInfo.Timestamp.Add(duration)
	return backoffTime
}

// flushUnschedulableBindingsLeftover moves bindings which stay in unschedulableBindings
// longer than bindingMaxInUnschedulableBindingsDuration to activeQ.
func (bq *prioritySchedulingQueue) flushUnschedulableBindingsLeftover() {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	var bindingsToMove []*QueuedBindingInfo
	currentTime := time.Now()
	for _, bInfo := range bq.unschedulableBindings.bindingInfoMap {
		lastScheduleTime := bInfo.Timestamp
		if currentTime.Sub(lastScheduleTime) > bq.bindingMaxInUnschedulableBindingsDuration {
			bindingsToMove = append(bindingsToMove, bInfo)
		}
	}

	for _, bInfo := range bindingsToMove {
		bq.moveToActiveQ(bInfo)
	}
}

func (bq *prioritySchedulingQueue) Push(bindingInfo *QueuedBindingInfo) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	bq.moveToActiveQ(bindingInfo)
}

// Pop removes the head of the active queue and returns it. It blocks if the
func (bq *prioritySchedulingQueue) Pop() (*QueuedBindingInfo, bool) {
	// activeQ is empty and waits until a new item is added to the queue.
	return bq.activeQ.Pop()
}

func (bq *prioritySchedulingQueue) PushUnschedulableIfNotPresent(bindingInfo *QueuedBindingInfo) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	if bq.backoffQ.Has(bindingInfo) || bq.activeQ.Has(bindingInfo.NamespacedKey) {
		return
	}

	bq.unschedulableBindings.addOrUpdate(bindingInfo)
	klog.V(4).Info("Binding moved to an internal scheduling queue", "binding", bindingInfo.NamespacedKey, "queue", unschedulableBindings)
}

func (bq *prioritySchedulingQueue) PushBackoffIfNotPresent(bindingInfo *QueuedBindingInfo) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	if bq.unschedulableBindings.get(bindingInfo.NamespacedKey) != nil || bq.activeQ.Has(bindingInfo.NamespacedKey) {
		return
	}

	bq.backoffQ.AddOrUpdate(bindingInfo)
	klog.V(4).Info("Binding moved to an internal scheduling queue", "binding", bindingInfo.NamespacedKey, "queue", backoffQ)
}

// Done must be called for binding returned by Pop. This allows the queue to
// keep track of which bindings are currently being processed.
func (bq *prioritySchedulingQueue) Done(bindingInfo *QueuedBindingInfo) {
	bq.activeQ.Done(bindingInfo)
}

func (bq *prioritySchedulingQueue) Len() int {
	return bq.activeQ.Len()
}

func (bq *prioritySchedulingQueue) Forget(bindingInfo *QueuedBindingInfo) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	_ = bq.backoffQ.Delete(bindingInfo)
}

// moveToActiveQ tries to add binding to active queue and remove it from unschedulable and backoff queues.
func (bq *prioritySchedulingQueue) moveToActiveQ(bindingInfo *QueuedBindingInfo) {
	bq.activeQ.Push(bindingInfo)
	_ = bq.backoffQ.Delete(bindingInfo) // just ignore this not-found error
	bq.unschedulableBindings.delete(bindingInfo.NamespacedKey)
	klog.V(4).Info("Binding moved to an internal scheduling queue", "binding", bindingInfo.NamespacedKey, "queue", activeQ)
}

// UnschedulableBindings holds bindings that cannot be scheduled. This data structure
// is used to implement unschedulableBindings.
type UnschedulableBindings struct {
	// bindingInfoMap is a map key by a binding's full-name and the value is a pointer to the QueuedBindingInfo.
	bindingInfoMap map[string]*QueuedBindingInfo
	// unschedulableRecorder updates the counter when elements of an bindingInfoMap
	// get added or removed, and it does nothing if it's nil.
	unschedulableRecorder metrics.MetricRecorder
}

// addOrUpdate adds a binding to the unschedulable bindingInfoMap.
func (u *UnschedulableBindings) addOrUpdate(bindingInfo *QueuedBindingInfo) {
	if _, exists := u.bindingInfoMap[bindingInfo.NamespacedKey]; !exists {
		if u.unschedulableRecorder != nil {
			u.unschedulableRecorder.Inc()
		}
	}
	u.bindingInfoMap[bindingInfo.NamespacedKey] = bindingInfo
}

// delete deletes a binding from the unschedulable bindingInfoMap.
func (u *UnschedulableBindings) delete(bindingKey string) {
	if _, exists := u.bindingInfoMap[bindingKey]; exists {
		if u.unschedulableRecorder != nil {
			u.unschedulableRecorder.Dec()
		}
	}
	delete(u.bindingInfoMap, bindingKey)
}

// get returns the QueuedBindingInfo if a binding with the same key as the key of the given "binding"
// is found in the map. It returns nil otherwise.
func (u *UnschedulableBindings) get(bindingKey string) *QueuedBindingInfo {
	if bindingInfo, exists := u.bindingInfoMap[bindingKey]; exists {
		return bindingInfo
	}
	return nil
}

// clear removes all the entries from the unschedulable bindingInfoMap.
func (u *UnschedulableBindings) clear() {
	u.bindingInfoMap = make(map[string]*QueuedBindingInfo)
	if u.unschedulableRecorder != nil {
		u.unschedulableRecorder.Clear()
	}
}

// newUnschedulableBindings initializes a new object of UnschedulableBindings.
func newUnschedulableBindings(unschedulableRecorder metrics.MetricRecorder) *UnschedulableBindings {
	return &UnschedulableBindings{
		bindingInfoMap:        make(map[string]*QueuedBindingInfo),
		unschedulableRecorder: unschedulableRecorder,
	}
}
