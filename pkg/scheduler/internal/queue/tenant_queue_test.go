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

// splitKey is a test helper that splits a namespaced key.
func splitKey(key string) (namespace, name string, err error) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:], nil
		}
	}
	return "", key, nil
}
