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

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const defaultTenantName = "__default__"

// tenantActiveQueue embeds activequeue and adds a non-blocking TryPop
// method used by TenantSchedulingQueue to collect heads without blocking.
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

// newTenantPriorityQueue creates a prioritySchedulingQueue whose activeQ is a
// tenantActiveQueue, returning both so the caller can use TryPop via the
// tenantActiveQueue without modifying the prioritySchedulingQueue type.
func newTenantPriorityQueue(opts ...Option) (*prioritySchedulingQueue, *tenantActiveQueue) {
	q := newPrioritySchedulingQueue(opts...)
	taq := &tenantActiveQueue{q.activeQ.(*activequeue)}
	q.activeQ = taq
	return q, taq
}

// QueueingStrategy determines how bindings are ordered and whether
// head-of-line blocking is applied within a tenant queue.
type QueueingStrategy string

const (
	// BestEffortFIFO skips a blocked head and tries the next binding.
	BestEffortFIFO QueueingStrategy = "BestEffortFIFO"
	// StrictFIFO blocks the entire tenant queue when the head fails.
	StrictFIFO QueueingStrategy = "StrictFIFO"
)

// tenantEntry represents a single tenant queue with its metadata.
type tenantEntry struct {
	name     string
	queue    *prioritySchedulingQueue
	activeQ  *tenantActiveQueue
	strategy QueueingStrategy
	// blocked is set when the head of a StrictFIFO queue fails scheduling.
	// While blocked, collectHeads() skips this tenant.
	blocked bool
}

// TenantSchedulingQueue wraps multiple prioritySchedulingQueue instances,
// one per tenant, with round-robin Pop() semantics following Kueue's
// Heads() pattern. It implements the SchedulingQueue interface.
type TenantSchedulingQueue struct {
	mu sync.Mutex

	// tenants is the ordered list of tenant queues for iteration.
	tenants []*tenantEntry
	// tenantMap provides O(1) lookup by tenant name.
	tenantMap map[string]*tenantEntry
	// namespaceToTenant maps namespace name -> tenant name.
	namespaceToTenant map[string]string

	// rrIndex is the current round-robin index for Pop().
	rrIndex int
	// heads holds a batch of bindings collected from tenant queues,
	// one per tenant. Pop() returns from this batch before collecting again.
	heads []*QueuedBindingInfo
	// headIndex tracks the current position within the heads batch.
	headIndex int

	// cond is broadcast when any tenant queue gets a new item in activeQ
	// or when the queue is stopped.
	cond *sync.Cond
	// stopped indicates the queue is shutting down.
	stopped bool
}

// NewTenantSchedulingQueue creates a TenantSchedulingQueue with a default queue.
func NewTenantSchedulingQueue(opts ...Option) *TenantSchedulingQueue {
	tq := &TenantSchedulingQueue{
		tenantMap:         make(map[string]*tenantEntry),
		namespaceToTenant: make(map[string]string),
	}
	tq.cond = sync.NewCond(&tq.mu)

	// Always create the default queue for unmatched namespaces.
	defaultQ, taq := newTenantPriorityQueue(opts...)
	entry := &tenantEntry{
		name:     defaultTenantName,
		queue:    defaultQ,
		activeQ:  taq,
		strategy: BestEffortFIFO,
	}
	defaultQ.onActiveQPush = func() { tq.cond.Broadcast() }
	tq.tenants = append(tq.tenants, entry)
	tq.tenantMap[defaultTenantName] = entry

	return tq
}

// resolveTenant returns the tenant name for a given namespaced key.
// ClusterResourceBindings (no namespace) always go to the default tenant.
func (tq *TenantSchedulingQueue) resolveTenant(namespacedKey string) string {
	namespace, _, _ := cache.SplitMetaNamespaceKey(namespacedKey)
	if namespace == "" {
		return defaultTenantName
	}
	tq.mu.Lock()
	tenant, ok := tq.namespaceToTenant[namespace]
	tq.mu.Unlock()
	if !ok {
		return defaultTenantName
	}
	return tenant
}

// getTenantQueue returns the inner queue for the given tenant name.
func (tq *TenantSchedulingQueue) getTenantQueue(tenantName string) *prioritySchedulingQueue {
	tq.mu.Lock()
	entry, ok := tq.tenantMap[tenantName]
	tq.mu.Unlock()
	if !ok {
		entry = tq.tenantMap[defaultTenantName]
	}
	return entry.queue
}

// Push adds a binding to the appropriate tenant's active queue.
func (tq *TenantSchedulingQueue) Push(bindingInfo *QueuedBindingInfo) {
	tenant := tq.resolveTenant(bindingInfo.NamespacedKey)
	q := tq.getTenantQueue(tenant)
	q.Push(bindingInfo)
	// onActiveQPush callback on the inner queue broadcasts tq.cond
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

		// Collect a new batch of heads.
		tq.heads = tq.heads[:0]
		tq.headIndex = 0
		tq.collectHeadsLocked()

		if len(tq.heads) > 0 {
			item := tq.heads[tq.headIndex]
			tq.headIndex++
			return item, false
		}

		// All queues empty, wait for new items.
		tq.cond.Wait()
	}
}

// collectHeadsLocked pops one binding from each non-blocked tenant queue.
// Must be called with tq.mu held. Releases the lock temporarily while
// calling TryPop on inner queues.
func (tq *TenantSchedulingQueue) collectHeadsLocked() {
	n := len(tq.tenants)
	for i := 0; i < n; i++ {
		idx := (tq.rrIndex + i) % n
		entry := tq.tenants[idx]
		if entry.blocked {
			continue
		}
		// Release outer lock while popping from inner queue to avoid deadlock.
		tq.mu.Unlock()
		item, ok := entry.activeQ.TryPop()
		tq.mu.Lock()
		if ok && item != nil {
			tq.heads = append(tq.heads, item)
		}
	}
	// Advance round-robin index for next collection cycle.
	if len(tq.heads) > 0 {
		tq.rrIndex = (tq.rrIndex + len(tq.heads)) % n
	}
}

// PushUnschedulableIfNotPresent pushes an unschedulable binding back to the
// appropriate tenant's queue. For StrictFIFO tenants, blocks the queue.
func (tq *TenantSchedulingQueue) PushUnschedulableIfNotPresent(bindingInfo *QueuedBindingInfo) {
	tenantName := tq.resolveTenant(bindingInfo.NamespacedKey)
	q := tq.getTenantQueue(tenantName)
	q.PushUnschedulableIfNotPresent(bindingInfo)

	tq.mu.Lock()
	if entry, ok := tq.tenantMap[tenantName]; ok && entry.strategy == StrictFIFO {
		entry.blocked = true
		klog.V(4).InfoS("StrictFIFO tenant queue blocked", "tenant", tenantName, "binding", bindingInfo.NamespacedKey)
	}
	tq.mu.Unlock()
}

// PushBackoffIfNotPresent pushes a failed binding back to the appropriate
// tenant's queue. For StrictFIFO tenants, blocks the queue.
func (tq *TenantSchedulingQueue) PushBackoffIfNotPresent(bindingInfo *QueuedBindingInfo) {
	tenantName := tq.resolveTenant(bindingInfo.NamespacedKey)
	q := tq.getTenantQueue(tenantName)
	q.PushBackoffIfNotPresent(bindingInfo)

	tq.mu.Lock()
	if entry, ok := tq.tenantMap[tenantName]; ok && entry.strategy == StrictFIFO {
		entry.blocked = true
		klog.V(4).InfoS("StrictFIFO tenant queue blocked", "tenant", tenantName, "binding", bindingInfo.NamespacedKey)
	}
	tq.mu.Unlock()
}

// Done marks a binding as done processing in the appropriate tenant's queue.
func (tq *TenantSchedulingQueue) Done(bindingInfo *QueuedBindingInfo) {
	tenantName := tq.resolveTenant(bindingInfo.NamespacedKey)
	q := tq.getTenantQueue(tenantName)
	q.Done(bindingInfo)
}

// Forget removes a binding from the appropriate tenant's backoff queue.
func (tq *TenantSchedulingQueue) Forget(bindingInfo *QueuedBindingInfo) {
	tenantName := tq.resolveTenant(bindingInfo.NamespacedKey)
	q := tq.getTenantQueue(tenantName)
	q.Forget(bindingInfo)
}

// Len returns the total number of bindings across all tenant active queues.
func (tq *TenantSchedulingQueue) Len() int {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	total := 0
	for _, entry := range tq.tenants {
		total += entry.queue.Len()
	}
	return total
}

// Run starts flush goroutines for all tenant queues.
func (tq *TenantSchedulingQueue) Run() {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	for _, entry := range tq.tenants {
		entry.queue.Run()
	}
}

// Close shuts down all tenant queues and wakes any blocked Pop() callers.
func (tq *TenantSchedulingQueue) Close() {
	tq.mu.Lock()
	tq.stopped = true
	tq.mu.Unlock()
	tq.cond.Broadcast()

	tq.mu.Lock()
	defer tq.mu.Unlock()
	for _, entry := range tq.tenants {
		entry.queue.Close()
	}
}

// AddTenant creates a new tenant queue with the given strategy.
// If the tenant already exists, this is a no-op.
func (tq *TenantSchedulingQueue) AddTenant(name string, strategy QueueingStrategy, opts ...Option) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if _, exists := tq.tenantMap[name]; exists {
		return
	}

	q, taq := newTenantPriorityQueue(opts...)
	entry := &tenantEntry{
		name:     name,
		queue:    q,
		activeQ:  taq,
		strategy: strategy,
	}
	q.onActiveQPush = func() {
		tq.mu.Lock()
		if entry.strategy == StrictFIFO && entry.blocked {
			entry.blocked = false
			klog.V(4).InfoS("StrictFIFO tenant queue unblocked", "tenant", name)
		}
		tq.mu.Unlock()
		tq.cond.Broadcast()
	}

	tq.tenants = append(tq.tenants, entry)
	tq.tenantMap[name] = entry
	q.Run()
	klog.V(2).InfoS("Added tenant queue", "tenant", name, "strategy", strategy)
}

// RemoveTenant removes a tenant queue. Bindings in the removed queue are
// not moved — they will drain naturally.
func (tq *TenantSchedulingQueue) RemoveTenant(name string) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if name == defaultTenantName {
		return // never remove the default tenant
	}
	entry, ok := tq.tenantMap[name]
	if !ok {
		return
	}
	entry.queue.Close()
	delete(tq.tenantMap, name)

	// Remove from ordered slice.
	for i, e := range tq.tenants {
		if e.name == name {
			tq.tenants = append(tq.tenants[:i], tq.tenants[i+1:]...)
			break
		}
	}

	// Clean up namespace mappings pointing to this tenant.
	for ns, tenant := range tq.namespaceToTenant {
		if tenant == name {
			delete(tq.namespaceToTenant, ns)
		}
	}

	// Reset round-robin index if it's now out of bounds.
	if tq.rrIndex >= len(tq.tenants) {
		tq.rrIndex = 0
	}

	klog.V(2).InfoS("Removed tenant queue", "tenant", name)
}

// UpdateNamespaceMapping maps a namespace to a tenant. Pass an empty tenant
// name to remove the mapping (namespace will fall back to default).
func (tq *TenantSchedulingQueue) UpdateNamespaceMapping(namespace, tenant string) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if tenant == "" {
		delete(tq.namespaceToTenant, namespace)
	} else {
		tq.namespaceToTenant[namespace] = tenant
	}
}

// SetNamespaceMappings replaces all namespace-to-tenant mappings for a tenant.
func (tq *TenantSchedulingQueue) SetNamespaceMappings(tenant string, namespaces []string) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Remove old mappings for this tenant.
	for ns, t := range tq.namespaceToTenant {
		if t == tenant {
			delete(tq.namespaceToTenant, ns)
		}
	}

	// Set new mappings.
	newNs := sets.New[string](namespaces...)
	for ns := range newNs {
		tq.namespaceToTenant[ns] = tenant
	}
}
