/*
Copyright The Karmada Authors.

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

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	schedulingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/scheduling/v1alpha1"
)

const defaultTenantName = "__default__"

// drainableActiveQueue is implemented by active queues that can hand back every
// binding they are currently holding.
type drainableActiveQueue interface {
	Drain() []*QueuedBindingInfo
}

// tenantActiveQueue embeds activequeue and adds the non-blocking TryPop and Drain
// methods used by TenantSchedulingQueue to collect heads and to rescue pending
// bindings when a tenant goes away.
type tenantActiveQueue struct {
	*activequeue
}

// TryPop returns the head of the queue without blocking. Returns nil, false
// if the queue is empty or shutting down.
func (q *tenantActiveQueue) TryPop() (bindingInfo *QueuedBindingInfo, ok bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.activeBindings.Len() == 0 || q.shuttingDown {
		return nil, false
	}

	bindingInfo, _ = q.activeBindings.Pop()
	bindingInfo.Attempts++
	q.processingBindings.Insert(bindingInfo.NamespacedKey)
	q.dirtyBindings.Delete(bindingInfo.NamespacedKey)

	return bindingInfo, true
}

// Drain removes and returns every queued binding without marking any of them as
// being processed. Bindings already handed out by Pop/TryPop are left alone.
func (q *tenantActiveQueue) Drain() []*QueuedBindingInfo {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	var drained []*QueuedBindingInfo
	for q.activeBindings.Len() > 0 {
		bindingInfo, err := q.activeBindings.Pop()
		if err != nil {
			break
		}
		q.dirtyBindings.Delete(bindingInfo.NamespacedKey)
		drained = append(drained, bindingInfo)
	}
	return drained
}

// newTenantPriorityQueue creates a prioritySchedulingQueue whose activeQ is a
// tenantActiveQueue, returning both so the caller can use TryPop via the
// tenantActiveQueue without modifying the prioritySchedulingQueue type.
func newTenantPriorityQueue(opts ...Option) (*prioritySchedulingQueue, *tenantActiveQueue) {
	q := newPrioritySchedulingQueue(opts...)
	taq := &tenantActiveQueue{q.activeQ.(*activequeue)}
	q.activeQ = taq
	return q, taq
}

// tenantEntry represents a single tenant queue with its metadata.
type tenantEntry struct {
	name     string
	queue    *prioritySchedulingQueue
	activeQ  *tenantActiveQueue
	strategy schedulingv1alpha1.QueueingStrategy
	// blocked is set when the head of a StrictFIFO queue fails scheduling.
	// While blocked, collectHeads() skips this tenant. It is cleared as soon as
	// anything lands back in the tenant's activeQ.
	blocked bool
	// started records whether queue.Run() has already been called, so that a
	// tenant added before TenantSchedulingQueue.Run() does not end up with two
	// sets of flush goroutines.
	started bool
}

// TenantSchedulingQueue wraps multiple prioritySchedulingQueue instances,
// one per tenant, with round-robin Pop() semantics following Kueue's
// Heads() pattern. It implements the SchedulingQueue interface.
//
// # Locking
//
// mu guards the tenant set and the heads batch. The inner queues have their own
// locks, and pushing into an inner queue calls back into onActiveQPush, which
// takes mu. To keep that from deadlocking, mu is NEVER held across a call into an
// inner queue: every method that needs to touch inner queues snapshots the
// entries it cares about, releases mu, and only then calls in.
type TenantSchedulingQueue struct {
	mu sync.Mutex

	// tenants is the ordered list of tenant queues for iteration.
	tenants []*tenantEntry
	// tenantMap provides O(1) lookup by tenant name (= namespace name).
	tenantMap map[string]*tenantEntry

	// rrIndex is the current round-robin index for Pop().
	rrIndex int
	// heads holds a batch of bindings collected from tenant queues,
	// one per tenant. Pop() returns from this batch before collecting again.
	heads []*QueuedBindingInfo
	// headIndex tracks the current position within the heads batch.
	headIndex int

	// pushGeneration is bumped under mu every time a binding lands in some
	// tenant's activeQ. Pop() samples it before collecting heads and re-checks it
	// afterwards: collecting releases mu, so a push can slip in and broadcast
	// before Pop() reaches cond.Wait(). Comparing generations turns that lost
	// wakeup into another round of collection.
	pushGeneration uint64

	// cond is broadcast when any tenant queue gets a new item in activeQ
	// or when the queue is stopped.
	cond *sync.Cond
	// stopped indicates the queue is shutting down.
	stopped bool
	// running records whether Run() has been called, so tenants added later are
	// started immediately and tenants added earlier are started exactly once.
	running bool

	// options are the queue options every tenant queue is created with.
	options []Option
}

// NewTenantSchedulingQueue creates a TenantSchedulingQueue with a default queue.
func NewTenantSchedulingQueue(opts ...Option) *TenantSchedulingQueue {
	tq := &TenantSchedulingQueue{
		tenantMap: make(map[string]*tenantEntry),
		options:   opts,
	}
	tq.cond = sync.NewCond(&tq.mu)

	// Always create the default queue for unmatched namespaces.
	tq.mu.Lock()
	tq.addTenantLocked(defaultTenantName, schedulingv1alpha1.BestEffortFIFO)
	tq.mu.Unlock()

	return tq
}

// addTenantLocked creates and registers a tenant entry. Must be called with mu
// held. The returned entry is not started; the caller decides when to Run it,
// outside the lock.
func (tq *TenantSchedulingQueue) addTenantLocked(name string, strategy schedulingv1alpha1.QueueingStrategy) *tenantEntry {
	q, taq := newTenantPriorityQueue(tq.options...)
	entry := &tenantEntry{
		name:     name,
		queue:    q,
		activeQ:  taq,
		strategy: strategy,
	}
	q.onActiveQPush = func() { tq.onActiveQPush(entry) }

	tq.tenants = append(tq.tenants, entry)
	tq.tenantMap[name] = entry
	return entry
}

// onActiveQPush is invoked by an inner queue whenever a binding lands in its
// activeQ. It runs while that queue holds its own lock, so it must not call back
// into any inner queue.
func (tq *TenantSchedulingQueue) onActiveQPush(entry *tenantEntry) {
	tq.mu.Lock()
	if entry.blocked {
		entry.blocked = false
		klog.V(4).InfoS("StrictFIFO tenant queue unblocked", "tenant", entry.name)
	}
	tq.pushGeneration++
	tq.mu.Unlock()
	tq.cond.Broadcast()
}

// resolveTenant returns the tenant entry a binding belongs to. When pin is set,
// the decision is recorded on the binding so that later Done/Forget calls reach
// the same queue even if the tenant set changed in the meantime.
func (tq *TenantSchedulingQueue) resolveTenant(bindingInfo *QueuedBindingInfo, pin bool) *tenantEntry {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	// A binding that has already been routed keeps its queue for its whole
	// lifecycle. If that tenant is gone, fall back to the default queue.
	if bindingInfo.tenant != "" {
		if entry, ok := tq.tenantMap[bindingInfo.tenant]; ok {
			return entry
		}
		return tq.tenantMap[defaultTenantName]
	}

	// ClusterResourceBindings have no namespace and always use the default queue.
	namespace, _, _ := cache.SplitMetaNamespaceKey(bindingInfo.NamespacedKey)
	entry, ok := tq.tenantMap[namespace]
	if namespace == "" || !ok {
		entry = tq.tenantMap[defaultTenantName]
	}
	if pin {
		bindingInfo.tenant = entry.name
	}
	return entry
}

// Push adds a binding to the appropriate tenant's active queue.
func (tq *TenantSchedulingQueue) Push(bindingInfo *QueuedBindingInfo) {
	// resolveTenant releases mu before we call into the inner queue, whose push
	// calls back into onActiveQPush.
	tq.resolveTenant(bindingInfo, true).queue.Push(bindingInfo)
}

// Pop removes and returns the next binding using round-robin across tenants.
// It blocks if all tenant queues are empty.
func (tq *TenantSchedulingQueue) Pop() (*QueuedBindingInfo, bool) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	for {
		if tq.stopped {
			return nil, true
		}

		// Return from existing heads batch if available.
		if tq.headIndex < len(tq.heads) {
			item := tq.heads[tq.headIndex]
			tq.headIndex++
			return item, false
		}

		// Collect a new batch of heads. collectHeadsLocked releases mu while it
		// talks to the inner queues, so sample the generation first.
		generation := tq.pushGeneration
		tq.heads = tq.heads[:0]
		tq.headIndex = 0
		tq.collectHeadsLocked()

		if len(tq.heads) > 0 {
			item := tq.heads[tq.headIndex]
			tq.headIndex++
			return item, false
		}

		// Something was pushed while we were collecting and its broadcast landed
		// before we got here, so collect again rather than wait on it.
		if tq.pushGeneration != generation {
			continue
		}

		// All queues empty, wait for new items.
		tq.cond.Wait()
	}
}

// collectHeadsLocked pops one binding from each non-blocked tenant queue.
// Must be called with mu held. The lock is released while the inner queues are
// polled, so it works off a snapshot of the tenant list rather than indexing
// into tq.tenants, which another goroutine may shrink meanwhile.
func (tq *TenantSchedulingQueue) collectHeadsLocked() {
	n := len(tq.tenants)
	if n == 0 {
		return
	}
	candidates := make([]*tenantActiveQueue, 0, n)
	for i := 0; i < n; i++ {
		entry := tq.tenants[(tq.rrIndex+i)%n]
		if entry.blocked {
			continue
		}
		candidates = append(candidates, entry.activeQ)
	}
	// Start the next cycle one tenant further along so that, when queues are only
	// partially occupied, no tenant is consistently polled first.
	tq.rrIndex = (tq.rrIndex + 1) % n

	tq.mu.Unlock()
	collected := make([]*QueuedBindingInfo, 0, len(candidates))
	for _, activeQ := range candidates {
		if item, ok := activeQ.TryPop(); ok && item != nil {
			collected = append(collected, item)
		}
	}
	tq.mu.Lock()

	tq.heads = append(tq.heads, collected...)
}

// PushUnschedulableIfNotPresent pushes an unschedulable binding back to the
// appropriate tenant's queue. For StrictFIFO tenants, blocks the queue.
func (tq *TenantSchedulingQueue) PushUnschedulableIfNotPresent(bindingInfo *QueuedBindingInfo) {
	entry := tq.resolveTenant(bindingInfo, true)
	entry.queue.PushUnschedulableIfNotPresent(bindingInfo)
	tq.blockIfStrictFIFO(entry, bindingInfo)
}

// PushBackoffIfNotPresent pushes a failed binding back to the appropriate
// tenant's queue. For StrictFIFO tenants, blocks the queue.
func (tq *TenantSchedulingQueue) PushBackoffIfNotPresent(bindingInfo *QueuedBindingInfo) {
	entry := tq.resolveTenant(bindingInfo, true)
	entry.queue.PushBackoffIfNotPresent(bindingInfo)
	tq.blockIfStrictFIFO(entry, bindingInfo)
}

// blockIfStrictFIFO applies head-of-line blocking after a binding failed to schedule.
func (tq *TenantSchedulingQueue) blockIfStrictFIFO(entry *tenantEntry, bindingInfo *QueuedBindingInfo) {
	if entry.strategy != schedulingv1alpha1.StrictFIFO {
		return
	}
	tq.mu.Lock()
	defer tq.mu.Unlock()
	// The tenant may have been removed while we were pushing.
	if _, ok := tq.tenantMap[entry.name]; !ok {
		return
	}
	entry.blocked = true
	klog.V(4).InfoS("StrictFIFO tenant queue blocked", "tenant", entry.name, "binding", bindingInfo.NamespacedKey)
}

// Done marks a binding as done processing in the appropriate tenant's queue.
func (tq *TenantSchedulingQueue) Done(bindingInfo *QueuedBindingInfo) {
	tq.resolveTenant(bindingInfo, false).queue.Done(bindingInfo)
}

// Forget removes a binding from the appropriate tenant's backoff queue.
func (tq *TenantSchedulingQueue) Forget(bindingInfo *QueuedBindingInfo) {
	tq.resolveTenant(bindingInfo, false).queue.Forget(bindingInfo)
}

// Len returns the total number of bindings across all tenant active queues.
func (tq *TenantSchedulingQueue) Len() int {
	total := 0
	for _, entry := range tq.snapshotTenants() {
		total += entry.queue.Len()
	}
	return total
}

// Run starts flush goroutines for all tenant queues.
func (tq *TenantSchedulingQueue) Run() {
	tq.mu.Lock()
	tq.running = true
	toStart := make([]*tenantEntry, 0, len(tq.tenants))
	for _, entry := range tq.tenants {
		if !entry.started {
			entry.started = true
			toStart = append(toStart, entry)
		}
	}
	tq.mu.Unlock()

	for _, entry := range toStart {
		entry.queue.Run()
	}
}

// Close shuts down all tenant queues and wakes any blocked Pop() callers.
func (tq *TenantSchedulingQueue) Close() {
	tq.mu.Lock()
	if tq.stopped {
		tq.mu.Unlock()
		return
	}
	tq.stopped = true
	entries := make([]*tenantEntry, len(tq.tenants))
	copy(entries, tq.tenants)
	tq.mu.Unlock()
	tq.cond.Broadcast()

	for _, entry := range entries {
		entry.queue.Close()
	}
}

// snapshotTenants returns a copy of the tenant list so callers can iterate over
// it without holding mu while calling into the inner queues.
func (tq *TenantSchedulingQueue) snapshotTenants() []*tenantEntry {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	entries := make([]*tenantEntry, len(tq.tenants))
	copy(entries, tq.tenants)
	return entries
}

// AddTenant creates a new tenant queue with the given strategy.
// If the tenant already exists, this is a no-op.
func (tq *TenantSchedulingQueue) AddTenant(name string, strategy schedulingv1alpha1.QueueingStrategy) {
	tq.mu.Lock()
	if _, exists := tq.tenantMap[name]; exists {
		tq.mu.Unlock()
		return
	}
	entry := tq.addTenantLocked(name, strategy)
	// Only start the queue if the parent is already running; otherwise Run()
	// picks it up, so a tenant discovered during informer cache sync does not end
	// up with two sets of flush goroutines.
	start := tq.running
	entry.started = start
	tq.mu.Unlock()

	if start {
		entry.queue.Run()
	}
	klog.V(2).InfoS("Added tenant queue", "tenant", name, "strategy", strategy)
}

// RemoveTenant removes a tenant queue. Bindings still waiting in the removed
// queue are moved to the default queue so that deleting a TenantQueue does not
// strand them. Bindings already handed out by Pop keep their routing and are
// completed against the default queue once the tenant entry is gone.
func (tq *TenantSchedulingQueue) RemoveTenant(name string) {
	if name == defaultTenantName {
		return // never remove the default tenant
	}

	tq.mu.Lock()
	entry, ok := tq.tenantMap[name]
	if !ok {
		tq.mu.Unlock()
		return
	}
	delete(tq.tenantMap, name)
	for i, e := range tq.tenants {
		if e.name == name {
			tq.tenants = append(tq.tenants[:i], tq.tenants[i+1:]...)
			break
		}
	}
	if tq.rrIndex >= len(tq.tenants) {
		tq.rrIndex = 0
	}
	defaultEntry := tq.tenantMap[defaultTenantName]
	stopped := tq.stopped
	tq.mu.Unlock()

	if stopped {
		// Close() already shut every tenant queue down; closing again would panic.
		klog.V(2).InfoS("Removed tenant queue", "tenant", name)
		return
	}

	// The entry is already unlinked, so nothing new can be routed to it. Rescue
	// whatever is still pending before shutting it down.
	pending := entry.queue.drainPending()
	entry.queue.Close()

	for _, bindingInfo := range pending {
		bindingInfo.tenant = defaultEntry.name
		defaultEntry.queue.Push(bindingInfo)
	}

	klog.V(2).InfoS("Removed tenant queue", "tenant", name, "requeuedBindings", len(pending))
}
