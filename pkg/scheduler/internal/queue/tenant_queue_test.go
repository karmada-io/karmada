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

	// Higher priority should come first.
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
	tq.AddTenant("team-a", BestEffortFIFO)
	tq.AddTenant("team-b", BestEffortFIFO)
	tq.SetNamespaceMappings("team-a", []string{"ns-a"})
	tq.SetNamespaceMappings("team-b", []string{"ns-b"})
	tq.Run()
	defer tq.Close()

	// Push bindings for different tenants.
	tq.Push(newBindingInfo("ns-a", "a1", 10))
	tq.Push(newBindingInfo("ns-b", "b1", 10))

	// Both tenants should be served.
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
	tq.AddTenant("team-a", BestEffortFIFO)
	tq.AddTenant("team-b", BestEffortFIFO)
	tq.SetNamespaceMappings("team-a", []string{"ns-a"})
	tq.SetNamespaceMappings("team-b", []string{"ns-b"})
	tq.Run()
	defer tq.Close()

	// Push 3 bindings for team-a, 1 for team-b.
	tq.Push(newBindingInfo("ns-a", "a1", 10))
	tq.Push(newBindingInfo("ns-a", "a2", 10))
	tq.Push(newBindingInfo("ns-a", "a3", 10))
	tq.Push(newBindingInfo("ns-b", "b1", 10))

	// First batch: should get one from each tenant (heads pattern).
	got1, _ := tq.Pop()
	tq.Done(got1)
	got2, _ := tq.Pop()
	tq.Done(got2)

	// Verify we got one from each tenant in the first batch.
	namespaces := map[string]bool{}
	ns1, _, _ := splitKey(got1.NamespacedKey)
	ns2, _, _ := splitKey(got2.NamespacedKey)
	namespaces[ns1] = true
	namespaces[ns2] = true

	// The first batch should include items from both the default queue and tenant queues.
	// Since ns-a and ns-b each have their own tenant, plus the default,
	// we should see items from at least 2 different sources.
	if len(namespaces) < 2 {
		t.Errorf("expected items from at least 2 namespaces in first batch, got %v", namespaces)
	}
}

func TestTenantSchedulingQueue_ClusterResourceBindingGoesToDefault(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("team-a", BestEffortFIFO)
	tq.SetNamespaceMappings("team-a", []string{"ns-a"})
	tq.Run()
	defer tq.Close()

	// ClusterResourceBinding has no namespace.
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
	tq.AddTenant("team-a", BestEffortFIFO)
	tq.SetNamespaceMappings("team-a", []string{"ns-a"})
	tq.Run()
	defer tq.Close()

	// ns-unknown is not mapped to any tenant.
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

	// Initially no tenants besides default.
	if len(tq.tenants) != 1 {
		t.Fatalf("expected 1 tenant (default), got %d", len(tq.tenants))
	}

	tq.AddTenant("team-a", BestEffortFIFO)
	if len(tq.tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(tq.tenants))
	}

	// Adding the same tenant again is a no-op.
	tq.AddTenant("team-a", StrictFIFO)
	if len(tq.tenants) != 2 {
		t.Fatalf("expected 2 tenants after duplicate add, got %d", len(tq.tenants))
	}

	tq.RemoveTenant("team-a")
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
	tq.AddTenant("strict-team", StrictFIFO)
	tq.AddTenant("besteffort-team", BestEffortFIFO)
	tq.SetNamespaceMappings("strict-team", []string{"ns-strict"})
	tq.SetNamespaceMappings("besteffort-team", []string{"ns-best"})
	tq.Run()
	defer tq.Close()

	// Push items to both queues.
	tq.Push(newBindingInfo("ns-strict", "s1", 10))
	tq.Push(newBindingInfo("ns-strict", "s2", 10))
	tq.Push(newBindingInfo("ns-best", "b1", 10))

	// Pop first batch — should get heads from both tenants + default.
	got1, _ := tq.Pop()
	got2, _ := tq.Pop()

	// Find which is from strict-team.
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

	// Simulate scheduling failure for the strict binding — push to unschedulable.
	tq.PushUnschedulableIfNotPresent(strictBinding)

	// Verify strict-team is blocked.
	tq.mu.Lock()
	entry := tq.tenantMap["strict-team"]
	blocked := entry.blocked
	tq.mu.Unlock()
	if !blocked {
		t.Error("expected strict-team to be blocked after PushUnschedulableIfNotPresent")
	}

	// Push more items to strict-team.
	tq.Push(newBindingInfo("ns-best", "b2", 10))

	// Pop should only return from besteffort-team (strict is blocked).
	got3, _ := tq.Pop()
	tq.Done(got3)
	ns3, _, _ := splitKey(got3.NamespacedKey)
	if ns3 == "ns-strict" {
		t.Error("strict-team should be blocked, but got an item from it")
	}
}

func TestTenantSchedulingQueue_StrictFIFOUnblocking(t *testing.T) {
	tq := NewTenantSchedulingQueue()
	tq.AddTenant("strict-team", StrictFIFO)
	tq.SetNamespaceMappings("strict-team", []string{"ns-strict"})
	tq.Run()
	defer tq.Close()

	b1 := newBindingInfo("ns-strict", "s1", 10)
	tq.Push(b1)

	got, _ := tq.Pop()
	tq.Done(got)

	// Simulate failure.
	tq.PushUnschedulableIfNotPresent(got)

	tq.mu.Lock()
	blocked := tq.tenantMap["strict-team"].blocked
	tq.mu.Unlock()
	if !blocked {
		t.Fatal("expected strict-team to be blocked")
	}

	// Re-push the binding (simulating backoff completion → moveToActiveQ).
	// The onActiveQPush callback should unblock.
	tq.Push(got)

	tq.mu.Lock()
	blocked = tq.tenantMap["strict-team"].blocked
	tq.mu.Unlock()
	if blocked {
		t.Error("expected strict-team to be unblocked after re-push to activeQ")
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

	// Give Pop() time to block.
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
	tq.AddTenant("team-a", BestEffortFIFO)
	tq.SetNamespaceMappings("team-a", []string{"ns-a"})
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
