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
	"fmt"
	"sync"
	"testing"
	"time"

	schedulingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/scheduling/v1alpha1"
)

func newBindingInfo(namespace, name string, priority int32) *QueuedBindingInfo {
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	return &QueuedBindingInfo{
		NamespacedKey: key,
		Priority:      priority,
		Timestamp:     time.Now(),
	}
}

func TestTenantSchedulingQueue_SingleTenant(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.Run()
	defer tq.Close()

	b1 := newBindingInfo("ns1", "binding1", 10)
	b2 := newBindingInfo("ns1", "binding2", 20)

	tq.Push(b1)
	tq.Push(b2)

	// Higher priority should come first (default queue).
	got, shutdown := tq.Pop()
	if shutdown {
		t.Fatal("unexpected shutdown")
	}
	if got.NamespacedKey != "ns1/binding2" {
		t.Errorf("expected ns1/binding2, got %s", got.NamespacedKey)
	}
	tq.Done(got)

	got, shutdown = tq.Pop()
	if shutdown {
		t.Fatal("unexpected shutdown")
	}
	if got.NamespacedKey != "ns1/binding1" {
		t.Errorf("expected ns1/binding1, got %s", got.NamespacedKey)
	}
	tq.Done(got)
}

func TestTenantSchedulingQueue_MultiTenantRouting(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	// Tenant name = namespace name.
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.AddTenant("ns-b", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	tq.Push(newBindingInfo("ns-a", "a1", 10))
	tq.Push(newBindingInfo("ns-b", "b1", 10))

	got1, _ := tq.Pop()
	tq.Done(got1)
	got2, _ := tq.Pop()
	tq.Done(got2)

	keys := map[string]bool{got1.NamespacedKey: true, got2.NamespacedKey: true}
	if !keys["ns-a/a1"] || !keys["ns-b/b1"] {
		t.Errorf("expected both ns-a/a1 and ns-b/b1, got %v and %v", got1.NamespacedKey, got2.NamespacedKey)
	}
}

func TestTenantSchedulingQueue_RoundRobinFairness(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.AddTenant("ns-b", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	// Push 3 bindings for ns-a, 1 for ns-b.
	tq.Push(newBindingInfo("ns-a", "a1", 10))
	tq.Push(newBindingInfo("ns-a", "a2", 10))
	tq.Push(newBindingInfo("ns-a", "a3", 10))
	tq.Push(newBindingInfo("ns-b", "b1", 10))

	// First batch: one head per tenant.
	got1, _ := tq.Pop()
	tq.Done(got1)
	got2, _ := tq.Pop()
	tq.Done(got2)

	namespaces := map[string]bool{}
	ns1, _, _ := splitKey(got1.NamespacedKey)
	ns2, _, _ := splitKey(got2.NamespacedKey)
	namespaces[ns1] = true
	namespaces[ns2] = true

	if len(namespaces) < 2 {
		t.Errorf("expected items from at least 2 namespaces in first batch, got %v", namespaces)
	}
}

func TestTenantSchedulingQueue_ClusterResourceBindingGoesToDefault(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	// ClusterResourceBinding has no namespace — goes to default.
	crb := newBindingInfo("", "cluster-binding", 10)
	tq.Push(crb)

	got, _ := tq.Pop()
	tq.Done(got)
	if got.NamespacedKey != "cluster-binding" {
		t.Errorf("expected cluster-binding, got %s", got.NamespacedKey)
	}
}

func TestTenantSchedulingQueue_UnmatchedNamespaceGoesToDefault(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	// ns-unknown has no TenantQueue — falls back to default.
	b := newBindingInfo("ns-unknown", "binding1", 10)
	tq.Push(b)

	got, _ := tq.Pop()
	tq.Done(got)
	if got.NamespacedKey != "ns-unknown/binding1" {
		t.Errorf("expected ns-unknown/binding1, got %s", got.NamespacedKey)
	}
}

func TestTenantSchedulingQueue_AddRemoveTenant(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.Run()
	defer tq.Close()

	if len(tq.tenants) != 1 {
		t.Fatalf("expected 1 tenant (default), got %d", len(tq.tenants))
	}

	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	if len(tq.tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(tq.tenants))
	}

	// Duplicate add is a no-op.
	tq.AddTenant("ns-a", schedulingv1alpha1.StrictFIFO)
	if len(tq.tenants) != 2 {
		t.Fatalf("expected 2 tenants after duplicate add, got %d", len(tq.tenants))
	}

	tq.RemoveTenant("ns-a")
	if len(tq.tenants) != 1 {
		t.Fatalf("expected 1 tenant after remove, got %d", len(tq.tenants))
	}

	// Removing default is a no-op.
	tq.RemoveTenant(defaultTenantName)
	if len(tq.tenants) != 1 {
		t.Fatalf("expected 1 tenant after removing default, got %d", len(tq.tenants))
	}
}

func TestTenantSchedulingQueue_StrictFIFOBlocking(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-strict", schedulingv1alpha1.StrictFIFO)
	tq.AddTenant("ns-best", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	tq.Push(newBindingInfo("ns-strict", "s1", 10))
	tq.Push(newBindingInfo("ns-strict", "s2", 10))
	tq.Push(newBindingInfo("ns-best", "b1", 10))

	got1, _ := tq.Pop()
	got2, _ := tq.Pop()

	var strictBinding *QueuedBindingInfo
	for _, g := range []*QueuedBindingInfo{got1, got2} {
		ns, _, _ := splitKey(g.NamespacedKey)
		if ns == "ns-strict" {
			strictBinding = g
		}
	}
	tq.Done(got1)
	tq.Done(got2)

	if strictBinding == nil {
		t.Fatal("expected a binding from ns-strict in first batch")
	}

	tq.PushUnschedulableIfNotPresent(strictBinding)

	tq.mu.Lock()
	blocked := tq.tenantMap["ns-strict"].blocked
	tq.mu.Unlock()
	if !blocked {
		t.Error("expected ns-strict to be blocked after PushUnschedulableIfNotPresent")
	}

	tq.Push(newBindingInfo("ns-best", "b2", 10))

	got3, _ := tq.Pop()
	tq.Done(got3)
	ns3, _, _ := splitKey(got3.NamespacedKey)
	if ns3 == "ns-strict" {
		t.Error("ns-strict should be blocked, but got an item from it")
	}
}

func TestTenantSchedulingQueue_StrictFIFOUnblocking(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-strict", schedulingv1alpha1.StrictFIFO)
	tq.Run()
	defer tq.Close()

	b1 := newBindingInfo("ns-strict", "s1", 10)
	tq.Push(b1)

	got, _ := tq.Pop()
	tq.Done(got)

	tq.PushUnschedulableIfNotPresent(got)

	tq.mu.Lock()
	blocked := tq.tenantMap["ns-strict"].blocked
	tq.mu.Unlock()
	if !blocked {
		t.Fatal("expected ns-strict to be blocked")
	}

	tq.Push(got)

	tq.mu.Lock()
	blocked = tq.tenantMap["ns-strict"].blocked
	tq.mu.Unlock()
	if blocked {
		t.Error("expected ns-strict to be unblocked after re-push to activeQ")
	}
}

func TestTenantSchedulingQueue_Shutdown(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.Run()

	done := make(chan struct{})
	go func() {
		_, shutdown := tq.Pop()
		if !shutdown {
			t.Error("expected shutdown=true")
		}
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	tq.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Pop() did not return after Close()")
	}
}

func TestTenantSchedulingQueue_Len(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	tq.Push(newBindingInfo("ns-a", "a1", 10))
	tq.Push(newBindingInfo("ns-a", "a2", 10))
	tq.Push(newBindingInfo("ns-unknown", "u1", 10))

	if tq.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", tq.Len())
	}
}

// TestTenantSchedulingQueue_RemoveTenantRequeuesBindings verifies that deleting a
// TenantQueue does not strand the bindings still waiting in its queue.
func TestTenantSchedulingQueue_RemoveTenantRequeuesBindings(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.Run()
	defer tq.Close()

	// One binding in each of the three internal sub-queues.
	tq.Push(newBindingInfo("ns-a", "active", 10))
	tq.PushBackoffIfNotPresent(newBindingInfo("ns-a", "backoff", 10))
	tq.PushUnschedulableIfNotPresent(newBindingInfo("ns-a", "unschedulable", 10))

	tq.RemoveTenant("ns-a")

	if got := tq.Len(); got != 3 {
		t.Fatalf("expected all 3 bindings to be requeued on the default queue, got Len()=%d", got)
	}

	got := map[string]bool{}
	for i := 0; i < 3; i++ {
		bindingInfo, shutdown := tq.Pop()
		if shutdown {
			t.Fatal("unexpected shutdown")
		}
		got[bindingInfo.NamespacedKey] = true
		tq.Done(bindingInfo)
	}
	for _, key := range []string{"ns-a/active", "ns-a/backoff", "ns-a/unschedulable"} {
		if !got[key] {
			t.Errorf("binding %s was dropped by RemoveTenant", key)
		}
	}
}

// TestTenantSchedulingQueue_RoutingSurvivesTenantChange verifies that a binding
// popped from one queue is completed against that same queue even when the tenant
// set changes while it is in flight.
//
// If Done() is routed by re-resolving the namespace instead, it lands on the
// newly created tenant queue while the default queue is still holding the key in
// its processing set. Nothing ever clears it, so once the tenant queue goes away
// again and routing falls back to the default queue, every later push of that key
// is silently swallowed and the binding is never scheduled again.
func TestTenantSchedulingQueue_RoutingSurvivesTenantChange(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.Run()
	defer tq.Close()

	// ns-a has no TenantQueue yet, so this lands on the default queue.
	tq.Push(newBindingInfo("ns-a", "b1", 10))
	inFlight, shutdown := tq.Pop()
	if shutdown {
		t.Fatal("unexpected shutdown")
	}

	// A TenantQueue for ns-a shows up while the binding is being scheduled, and is
	// deleted again after it completes.
	tq.AddTenant("ns-a", schedulingv1alpha1.BestEffortFIFO)
	tq.Done(inFlight)
	tq.RemoveTenant("ns-a")

	// Routing is back to the default queue, which must not still believe the
	// binding is being processed.
	tq.Push(newBindingInfo("ns-a", "b1", 10))
	if got := tq.Len(); got != 1 {
		t.Fatalf("expected the re-pushed binding to be queued, got Len()=%d", got)
	}

	popped := make(chan string, 1)
	go func() {
		bindingInfo, _ := tq.Pop()
		if bindingInfo != nil {
			popped <- bindingInfo.NamespacedKey
			tq.Done(bindingInfo)
		}
	}()
	select {
	case key := <-popped:
		if key != "ns-a/b1" {
			t.Errorf("expected ns-a/b1, got %s", key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("binding was swallowed after the tenant set changed mid-flight")
	}
}

// TestTenantSchedulingQueue_PopWakesOnPushDuringCollect covers the wakeup race:
// collecting heads releases the outer lock, so a push landing in that window
// broadcasts before Pop() reaches cond.Wait(). Pop() must notice and re-collect
// rather than sleep on a non-empty queue.
func TestTenantSchedulingQueue_PopWakesOnPushDuringCollect(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	for i := 0; i < 200; i++ {
		tq.AddTenant(fmt.Sprintf("ns%d", i), schedulingv1alpha1.BestEffortFIFO)
	}
	tq.Run()
	defer tq.Close()

	popped := make(chan struct{}, 1)
	go func() {
		for {
			bindingInfo, shutdown := tq.Pop()
			if shutdown {
				return
			}
			tq.Done(bindingInfo)
			popped <- struct{}{}
		}
	}()

	// Give the popper time to settle into cond.Wait(), then feed it one binding at
	// a time so every push has to wake it up on its own.
	time.Sleep(10 * time.Millisecond)
	for round := 0; round < 200; round++ {
		tq.Push(newBindingInfo(fmt.Sprintf("ns%d", round), "only", 10))
		select {
		case <-popped:
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: Pop() did not return although a binding is queued (Len=%d)", round, tq.Len())
		}
	}
}

// TestTenantSchedulingQueue_ConcurrentTenantChurn exercises the queue the way the
// scheduler does: a single Pop loop against concurrent pushes while TenantQueue
// objects are created and deleted. Run under -race, it covers the lock ordering
// between the outer queue and the per-tenant queues, and the tenant list being
// mutated while heads are collected.
func TestTenantSchedulingQueue_ConcurrentTenantChurn(t *testing.T) {
	const tenants = 40

	tq := NewTenantSchedulingQueue()
	tq.Run()

	popperDone := make(chan struct{})
	go func() {
		defer close(popperDone)
		for {
			bindingInfo, shutdown := tq.Pop()
			if shutdown {
				return
			}
			tq.Done(bindingInfo)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < tenants; i++ {
			tq.AddTenant(fmt.Sprintf("ns%d", i), schedulingv1alpha1.BestEffortFIFO)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < tenants; i++ {
			for j := 0; j < 25; j++ {
				tq.Push(newBindingInfo(fmt.Sprintf("ns%d", i), fmt.Sprintf("b%d", j), 10))
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < tenants; i++ {
			tq.RemoveTenant(fmt.Sprintf("ns%d", i))
		}
	}()

	waited := make(chan struct{})
	go func() {
		wg.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock: producers did not finish within 30s")
	}

	tq.Close()
	select {
	case <-popperDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Pop() did not return after Close()")
	}
}

// TestTenantSchedulingQueue_TenantsStartedOnce guards against a tenant discovered
// before Run() ending up with two sets of flush goroutines.
func TestTenantSchedulingQueue_TenantsStartedOnce(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("ns-early", schedulingv1alpha1.BestEffortFIFO)

	tq.mu.Lock()
	early := tq.tenantMap["ns-early"].started
	tq.mu.Unlock()
	if early {
		t.Error("tenant added before Run() should not have been started yet")
	}

	tq.Run()
	defer tq.Close()

	tq.AddTenant("ns-late", schedulingv1alpha1.BestEffortFIFO)

	tq.mu.Lock()
	defer tq.mu.Unlock()
	for _, name := range []string{defaultTenantName, "ns-early", "ns-late"} {
		if !tq.tenantMap[name].started {
			t.Errorf("tenant %s was never started", name)
		}
	}
}

// splitKey is a test helper that splits a namespaced key.
func splitKey(key string) (namespace, name string, err error) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:], nil
		}
	}
	return "", key, nil
}
