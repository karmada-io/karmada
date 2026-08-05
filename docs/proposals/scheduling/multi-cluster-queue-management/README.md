---
title: Multi-Cluster Queue Management

authors:
- "@hzheng182"

reviewers:
- TBD

approvers:
- TBD

creation-date: 2026-04-09
---

# Multi-Cluster Queue Management

## Summary

This proposal introduces **per-tenant queue sharding** for Karmada's existing scheduler queue system, enabling multi-tenant isolation without introducing new heavyweight abstractions.

Karmada's scheduler already maintains three internal queues for `ResourceBinding` objects:

- **`activeQ`** — priority heap of bindings ready to be scheduled
- **`backoffQ`** — bindings waiting out an exponential backoff after a failed scheduling attempt
- **`unschedulableBindings`** — bindings that could not be scheduled and are awaiting a cluster state change

Today these three queues are global singletons. This proposal introduces a namespace-scoped `TenantQueue` API object that enables **opt-in per-tenant queue isolation**. Namespaces that create a `TenantQueue` get their own isolated set of queues; namespaces without one continue to share a global default queue (backward compatible). Since tenant = namespace = `FederatedResourceQuota` scope, no separate namespace selector is needed — one `TenantQueue` per namespace governs the queue behavior for all `ResourceBinding` objects in that namespace.

---

## Motivation

As Karmada is increasingly adopted for AI training and batch workloads, multiple teams share the same Karmada control plane. With a single global queue:

- A global priority queue incentivizes tenants to set artificially high priorities on their jobs to get scheduled first, even when their jobs are not genuinely high-priority. This erodes the usefulness of priority as a scheduling signal.
- Even within the same priority level, a tenant submitting a burst of jobs can block jobs from other tenants for an extended period, since all bindings compete in the same `activeQ`.
- There is no way to enforce different ordering modes per tenant (e.g., strict FIFO for a pipeline team, best-effort FIFO for an interactive team).

Making the existing queues per-tenant, with a `TenantQueue` object in each namespace, solves these problems with minimal new API surface.

### Goals

- Make the scheduler's `activeQ`, `backoffQ`, and `unschedulableBindings` per-tenant.
- Introduce a namespace-scoped `TenantQueue` API (`scheduling.karmada.io/v1alpha1`) for per-tenant queue configuration.
- Support `BestEffortFIFO` and `StrictFIFO` ordering modes.
- Maintain backwards compatibility: namespaces without a `TenantQueue` use global defaults.

### Non-Goals

- Changes to the `backoffQ` or `unschedulableBindings` data structures themselves.
- Weighted round-robin (planned for a future phase, controlled by cluster admins).

---

## Proposal

### New API: `TenantQueue`

`TenantQueue` is a **namespace-scoped** resource with a singleton name `queue`. A namespace admin creates a `TenantQueue` named `queue` in their namespace to configure scheduling queue behavior for that namespace's `ResourceBinding` objects. A validating webhook rejects objects with any other name. The namespace itself defines the tenant — no selector is needed.

```go
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=tenantqueues,scope=Namespaced,shortName=tq,categories={karmada-io}

// TenantQueue configures per-tenant scheduling queue settings.
// One TenantQueue per namespace. ResourceBindings in the same namespace
// are routed to this queue for scheduling.
type TenantQueue struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec TenantQueueSpec `json:"spec"`
}

type TenantQueueSpec struct {
    // QueueingStrategy controls the ordering and blocking behavior of
    // bindings in the active queue.
    // +kubebuilder:default=BestEffortFIFO
    // +kubebuilder:validation:Enum=BestEffortFIFO;StrictFIFO
    // +optional
    QueueingStrategy QueueingStrategy `json:"queueingStrategy,omitempty"`
}

// QueueingStrategy defines the ordering and blocking behavior of bindings in the active queue.
type QueueingStrategy string

const (
    // BestEffortFIFO skips unschedulable head bindings and tries the next one.
    BestEffortFIFO QueueingStrategy = "BestEffortFIFO"
    // StrictFIFO blocks the entire tenant queue when the head binding fails (head-of-line blocking).
    StrictFIFO     QueueingStrategy = "StrictFIFO"
)
```

#### Example

```yaml
# Namespace admin opts into strict ordering for their pipeline jobs
apiVersion: scheduling.karmada.io/v1alpha1
kind: TenantQueue
metadata:
  name: queue
  namespace: team-a
spec:
  queueingStrategy: StrictFIFO
---
# Another namespace uses the default (BestEffortFIFO), no TenantQueue needed
```

Namespaces without a `TenantQueue` — as well as all `ClusterResourceBinding` objects (which have no namespace) — are routed to a built-in `__default__` queue that always uses `BestEffortFIFO`. The default queue participates in the same round-robin as named tenant queues, getting one scheduling turn per cycle.

---

## Scheduler Changes

### Queue Sharding

The `prioritySchedulingQueue` today is a single struct. The scheduler is refactored to maintain a `TenantSchedulingQueue` that wraps multiple inner `prioritySchedulingQueue` instances, one per tenant (namespace), keyed by namespace name.

```
TenantSchedulingQueue
  ├── "team-a"  → prioritySchedulingQueue{activeQ, backoffQ, unschedulableBindings}  [StrictFIFO]
  ├── "team-b"  → prioritySchedulingQueue{activeQ, backoffQ, unschedulableBindings}  [BestEffortFIFO]
  └── __default__ → prioritySchedulingQueue{...}  // unmatched namespaces + ClusterResourceBindings
```

`TenantSchedulingQueue` implements the existing `SchedulingQueue` interface, so the rest of the scheduler is unchanged.

**Routing:** the namespace is extracted from the `NamespacedKey` of each `QueuedBindingInfo`. If the namespace has a registered tenant queue, the binding goes there; otherwise it goes to `__default__`. `ClusterResourceBinding` objects (no namespace) always go to `__default__`.

**Scheduling sequence example** — with `team-a` (StrictFIFO) and `team-b` (BestEffortFIFO) configured, and `team-c` without a `TenantQueue`:

```
Cycle 1 — collectHeads():
  __default__  → head: team-c-binding-1   (team-c has no TenantQueue)
  team-a       → head: team-a-binding-1
  team-b       → head: team-b-binding-1

Pop() returns: team-c-binding-1, team-a-binding-1, team-b-binding-1  (one per Pop() call)

Cycle 2 — collectHeads() (team-a blocked because team-a-binding-1 failed):
  __default__  → head: team-c-binding-2
  team-a       → skipped (StrictFIFO, blocked)
  team-b       → head: team-b-binding-2

Pop() returns: team-c-binding-2, team-b-binding-2
```

### Pop() with Kueue-Inspired Heads Pattern

Rather than a simple round-robin that pops one item at a time, `TenantSchedulingQueue.Pop()` uses a **batch-then-serve** approach inspired by Kueue's `Heads()` pattern:

1. **Collect heads**: call non-blocking `TryPop()` on each non-blocked tenant queue, gathering one binding per tenant into a `heads` slice.
2. **Serve heads**: return items from `heads` in order (advancing a round-robin index) until exhausted.
3. **Repeat**: when `heads` is empty, go back to step 1. Block if all queues are empty.

This ensures each tenant gets one scheduling turn per cycle regardless of how many bindings they have queued, preventing burst monopolization.

### StrictFIFO Mode

Both modes order bindings identically: **priority descending, then enqueue timestamp ascending** (using `QueuedBindingInfo.Timestamp`, set when the binding is enqueued or re-enqueued). The difference is in blocking behavior:

- **`BestEffortFIFO`** (default): if the head-of-queue binding fails scheduling, it is moved to `backoffQ` or `unschedulableBindings`. The next binding in that tenant's queue is tried in the following cycle.
- **`StrictFIFO`**: if the head-of-queue binding fails scheduling, the **entire tenant queue is blocked** — no later binding from that tenant is considered until the head is re-promoted to `activeQ`. This is head-of-line (HOL) blocking, matching Kueue's semantics.

HOL blocking is tracked via a `blocked bool` flag on the tenant entry. The flag is cleared by an `onActiveQPush` callback on the inner queue, which fires whenever a binding is moved back to `activeQ` (backoff expiry, unschedulable flush, cluster state change).

---

## Ordering Semantics Comparison

| Property | BestEffortFIFO | StrictFIFO |
|---|---|---|
| `activeQ` sort key | Priority desc, then enqueue timestamp asc | Priority desc, then enqueue timestamp asc |
| Head-of-queue blocked? | Skip, try next binding | Block entire tenant queue (HOL blocking) |
| Throughput | Higher | Lower (head-of-line blocking) |
| Ordering guarantee | Best effort | Deterministic within tenant |
| Typical use case | Interactive / heterogeneous batch | Sequential pipelines, strict ordering |

---

## Design Notes

### Why Namespace-Scoped

`TenantQueue` is namespace-scoped because tenant = namespace = `FederatedResourceQuota` scope in Karmada's model. This eliminates the need for a selector field and allows namespace admins to manage their own queue settings. The namespace identity is sufficient to route bindings without any indirection.

### Comparison to Kueue

Kueue has a three-level hierarchy: `LocalQueue` (namespaced) → `ClusterQueue` (cluster-scoped) → `Cohort`. Karmada's model merges these into a single namespace-scoped `TenantQueue`, since quota enforcement lives in `FederatedResourceQuota` rather than in the queue itself. There is no borrowing, no resource flavors, and no cohort concept.

Cross-tenant scheduling fairness is achieved by the Heads pattern (one binding per tenant per cycle). Kueue adds DRS (Dominant Resource Share) tournament ordering on top of this for the fair-sharing iterator; Karmada Phase 1 uses simple round-robin across tenant heads.

### Feature Gate

Gated behind `TenantQueueManagement` (alpha, disabled by default). Requires `PriorityBasedScheduling` to also be enabled.

---

## Relationship to Existing Features

| Feature | Relationship |
|---|---|
| `PriorityBasedScheduling` feature gate | Required. `TenantQueue` adds per-tenant isolation on top. |
| `FederatedResourceQuota` | Aligns scope: one `TenantQueue` per namespace, one `FederatedResourceQuota` per namespace. |

---

## Implementation Plan

### Phase 1: Queue Sharding with BestEffortFIFO and StrictFIFO (Alpha)

1. Add `TenantQueue` API type under `scheduling.karmada.io/v1alpha1` (namespace-scoped, singleton name `queue`).
2. Add validating webhook to enforce the singleton name.
3. Implement `TenantSchedulingQueue` wrapping multiple `prioritySchedulingQueue` instances.
4. Implement Heads-pattern `Pop()` with round-robin across tenant queues.
5. Implement `StrictFIFO` with `blocked` flag and `onActiveQPush` unblocking callback.
6. Add informer watch for `TenantQueue` in the scheduler; route bindings by namespace.
7. Feature gate: `TenantQueueManagement` (disabled by default).

### Phase 2: Weighted Round-Robin (Alpha)

Cluster admins configure per-tenant weights (e.g., via a separate cluster-scoped resource or annotation on `FederatedResourceQuota`). The Heads-pattern `Pop()` is extended to weight tenants proportionally to their allocated quota.

### Phase 3: Stabilization (Beta)

1. Promote `TenantQueue` API to `v1beta1`.
2. Graduation of `TenantQueueManagement` feature gate to beta.

---

## Open Questions

1. **Weighted round-robin?** The current round-robin gives each tenant equal scheduling turns. Should tenants with larger `FederatedResourceQuota` allocations get proportionally more turns? This would be cluster-admin controlled (not configurable in `TenantQueue` itself). Deferred to Phase 2.

2. **ClusterResourceBinding**: `ClusterResourceBinding` objects are cluster-scoped (no namespace). They always use the global default queue. The proposal does not change their handling.
