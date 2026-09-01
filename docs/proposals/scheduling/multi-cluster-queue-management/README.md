---
title: Multi-Cluster Queue Management

authors:
- "@hzheng182"
- "@CaesarTY"

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

Today these three queues are global singletons. This proposal makes them **per-namespace by default**: every namespace gets its own isolated set of queues automatically, with no object required. A namespace-scoped `TenantQueue` object is optional and only lets a namespace admin change *how* their queue orders bindings (`BestEffortFIFO` vs `StrictFIFO`) — it does not gate isolation itself. Since tenant = namespace = `FederatedResourceQuota` scope, no separate namespace selector is needed.

---

## Motivation

As Karmada is increasingly adopted for AI training and batch workloads, multiple teams share the same Karmada control plane. With a single global queue:

- A global priority queue incentivizes tenants to set artificially high priorities on their jobs to get scheduled first, even when their jobs are not genuinely high-priority. This erodes the usefulness of priority as a scheduling signal.
- Even within the same priority level, a tenant submitting a burst of jobs can block jobs from other tenants for an extended period, since all bindings compete in the same `activeQ`.
- There is no way to enforce different ordering modes per tenant (e.g., strict FIFO for a pipeline team, best-effort FIFO for an interactive team).

Isolation has to be automatic rather than opt-in: if a tenant had to create an object to get its own queue, that object becomes exactly the kind of unilateral advantage the first bullet describes — a namespace could grant itself a bigger share of scheduling turns simply by asking. Sharding every namespace by default removes that incentive; `TenantQueue` is left to do only what it needs to do, which is let a namespace choose its ordering strategy.

### Goals

- Make the scheduler's `activeQ`, `backoffQ`, and `unschedulableBindings` per-namespace by default, with no object required.
- Introduce a namespace-scoped `TenantQueue` API (`policy.karmada.io/v1alpha1`) so a namespace can opt into `StrictFIFO` ordering for its own queue.
- Support `BestEffortFIFO` (default) and `StrictFIFO` ordering modes.
- Maintain backwards compatibility: behavior for existing clusters is unchanged until the feature gate is enabled.

### Non-Goals

- Changes to the `backoffQ` or `unschedulableBindings` data structures themselves.
- Weighted round-robin (planned for a future phase, controlled by cluster admins).
- Cross-tenant priority ordering. Priority determines order *within* a tenant's own queue; Phase 1's round-robin does not weigh tenants against each other by priority. See [Risks and Mitigations](#risks-and-mitigations).

---

## Proposal

### New API: `TenantQueue`

`TenantQueue` is a **namespace-scoped** resource with a singleton name `queue`. Every namespace already has its own scheduling queue, created automatically the first time one of its `ResourceBinding` objects is scheduled, using `BestEffortFIFO`. A namespace admin creates a `TenantQueue` named `queue` only to change that queue's ordering strategy — creating or deleting the object never creates or removes isolation. A validating webhook rejects objects with any other name.

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

`TenantQueue` lives in `policy.karmada.io/v1alpha1` alongside `FederatedResourceQuota` rather than in a new API group — see [Alternatives](#alternatives).

#### Example

```yaml
# Namespace admin opts into strict ordering for their pipeline jobs.
# team-a already had its own queue before this object existed.
apiVersion: policy.karmada.io/v1alpha1
kind: TenantQueue
metadata:
  name: queue
  namespace: team-a
spec:
  queueingStrategy: StrictFIFO
---
# Another namespace uses the default (BestEffortFIFO); no TenantQueue needed,
# it still gets its own isolated queue automatically.
```

`ClusterResourceBinding` objects have no namespace and cannot own a per-namespace queue. They are routed to a single built-in `__default__` queue, which participates in the same round-robin as every namespace's queue, getting one scheduling turn per cycle.

---

## Scheduler Changes

### Queue Sharding

The `prioritySchedulingQueue` today is a single struct. The scheduler is refactored to maintain a `TenantSchedulingQueue` that wraps multiple inner `prioritySchedulingQueue` instances, one per namespace, created lazily on first use.

```
TenantSchedulingQueue
  ├── "team-a"     → prioritySchedulingQueue{activeQ, backoffQ, unschedulableBindings}  [StrictFIFO]      (explicit TenantQueue)
  ├── "team-b"     → prioritySchedulingQueue{activeQ, backoffQ, unschedulableBindings}  [BestEffortFIFO]  (explicit TenantQueue)
  ├── "team-c"     → prioritySchedulingQueue{activeQ, backoffQ, unschedulableBindings}  [BestEffortFIFO]  (implicit, no TenantQueue)
  └── __default__  → prioritySchedulingQueue{...}                                                          // ClusterResourceBindings only
```

`TenantSchedulingQueue` implements the existing `SchedulingQueue` interface, so the rest of the scheduler is unchanged.

**Routing:** the namespace is extracted from the `NamespacedKey` of each `QueuedBindingInfo`. If no queue exists yet for that namespace, one is created on demand with `BestEffortFIFO` before the binding is pushed. If a `TenantQueue` object exists for the namespace, its strategy is applied to that queue instead. `ClusterResourceBinding` objects (no namespace) always go to `__default__`. Namespace queues that stay empty, with no `TenantQueue` object, for longer than a GC interval are torn down and recreated lazily on the next push — see [Risks and Mitigations](#risks-and-mitigations).

**Scheduling sequence example** — with `team-a` (StrictFIFO) and `team-b` (BestEffortFIFO) configured via `TenantQueue`, and `team-c` scheduled purely on its implicit default queue:

```
Cycle 1 — collectHeads():
  team-a       → head: team-a-binding-1
  team-b       → head: team-b-binding-1
  team-c       → head: team-c-binding-1   (implicit queue, no TenantQueue object)

Pop() returns: team-a-binding-1, team-b-binding-1, team-c-binding-1  (one per Pop() call)

Cycle 2 — collectHeads() (team-a blocked because team-a-binding-1 failed):
  team-a       → skipped (StrictFIFO, blocked)
  team-b       → head: team-b-binding-2
  team-c       → head: team-c-binding-2

Pop() returns: team-b-binding-2, team-c-binding-2
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
- **`StrictFIFO`**: if the head-of-queue binding fails scheduling, the **entire tenant queue is blocked** — no later binding from that tenant is considered until that same binding is re-promoted to `activeQ`. This is head-of-line (HOL) blocking, matching Kueue's semantics.

HOL blocking is tracked via a flag on the tenant entry that records the identity of the binding currently blocking it. The flag is cleared only when *that specific binding* lands back in `activeQ` (backoff expiry, unschedulable flush, or cluster state change) — an unrelated new binding pushed into the same tenant's `activeQ` must not clear it. Clearing on any push would let a tenant route around its own head-of-line block simply by submitting new work, defeating the guarantee.

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

### Why Automatic Sharding Instead of Opt-In Isolation

An earlier version of this proposal made `TenantQueue` the trigger for isolation: namespaces without one shared a single `__default__` queue. That design reintroduces the exact problem in the Motivation — a namespace could unilaterally grant itself a larger share of scheduling turns just by creating an object, since it would then compete one-per-cycle against every other unisolated namespace combined. Sharding by namespace unconditionally removes that lever; `TenantQueue` is scoped down to configuring ordering strategy only.

### Comparison to Kueue

Kueue has a three-level hierarchy: `LocalQueue` (namespaced) → `ClusterQueue` (cluster-scoped) → `Cohort`. Karmada's model merges these into a single namespace-scoped `TenantQueue`, since quota enforcement lives in `FederatedResourceQuota` rather than in the queue itself. There is no borrowing, no resource flavors, and no cohort concept. Unlike Kueue, where a workload without a matching `LocalQueue` cannot be admitted at all, every Karmada namespace is schedulable without any object.

Cross-tenant scheduling fairness is achieved by the Heads pattern (one binding per tenant per cycle). Kueue adds DRS (Dominant Resource Share) tournament ordering on top of this for the fair-sharing iterator; Karmada Phase 1 uses simple round-robin across tenant heads.

### Feature Gate

Gated behind `TenantQueueManagement` (alpha, disabled by default). Requires `PriorityBasedScheduling` to also be enabled.

---

## Relationship to Existing Features

| Feature | Relationship |
|---|---|
| `PriorityBasedScheduling` feature gate | Required. Per-namespace isolation is automatic on top of it; `TenantQueue` additionally configures ordering strategy. |
| `FederatedResourceQuota` | Aligns scope and API group: one queue per namespace mirrors one `FederatedResourceQuota` per namespace, and both live under `policy.karmada.io`. |

---

## Risks and Mitigations

- **Priority is tenant-local, not global.** Round-robin across tenants means a low-priority binding in one namespace can be popped before a higher-priority binding in another namespace that hasn't had its turn yet in the current cycle. This is called out explicitly as a Non-Goal rather than fixed in Phase 1; cross-tenant weighting is deferred to Phase 2. Preemption is unaffected by queue turn — once a binding is popped, its priority still governs preemption against bindings on member clusters, so this risk only affects *how soon* a binding is attempted, not what it can preempt once it is.
- **Unbounded queue cardinality.** Creating a queue (plus its two background flush goroutines) per namespace ever observed, with no cleanup, would leak resources in clusters with high namespace churn. Mitigation: an idle namespace queue with no `TenantQueue` object and an empty `activeQ`/`backoffQ`/`unschedulableBindings` for longer than a GC interval is torn down and recreated lazily on the next push. The GC interval is an open question, see below.
- **StrictFIFO block must not be clearable by unrelated work.** Keying the block flag to the specific blocked binding's identity (rather than "something happened") is a deliberate, testable invariant — enforced with a unit test that pushes an unrelated new binding into a blocked StrictFIFO tenant and asserts the queue stays blocked.

---

## Test Plan

- Unit tests for `TenantSchedulingQueue`: lazy per-namespace queue creation, routing by namespace, round-robin fairness across a mix of explicit and implicit queues, and idle-queue GC.
- Unit tests for `StrictFIFO`: block set on scheduling failure, block held across an unrelated push to the same tenant, block cleared only when the specific blocked binding re-enters `activeQ`.
- Unit tests for the `TenantQueue` validating webhook: singleton name enforcement.
- Integration test simulating a burst from one namespace alongside steady traffic from others, asserting no namespace is starved beyond one scheduling cycle.

---

## Implementation Plan

### Phase 1: Queue Sharding with BestEffortFIFO and StrictFIFO (Alpha)

1. Add `TenantQueue` API type under `policy.karmada.io/v1alpha1` (namespace-scoped, singleton name `queue`).
2. Add validating webhook to enforce the singleton name.
3. Implement `TenantSchedulingQueue` wrapping multiple `prioritySchedulingQueue` instances, created lazily per namespace with `BestEffortFIFO` by default.
4. Implement Heads-pattern `Pop()` with round-robin across tenant queues.
5. Implement `StrictFIFO` with a per-tenant block flag keyed to the specific blocked binding, cleared only when that binding re-enters `activeQ`.
6. Add informer watch for `TenantQueue` in the scheduler to apply strategy overrides; route bindings by namespace with lazy queue creation.
7. Add idle namespace-queue garbage collection.
8. Feature gate: `TenantQueueManagement` (disabled by default).

### Phase 2: Weighted Round-Robin (Alpha)

Cluster admins configure per-tenant weights (e.g., via a separate cluster-scoped resource or annotation on `FederatedResourceQuota`). The Heads-pattern `Pop()` is extended to weight tenants proportionally to their allocated quota, addressing the cross-tenant priority gap noted in Risks and Mitigations.

### Phase 3: Stabilization (Beta)

1. Promote `TenantQueue` API to `v1beta1`.
2. Graduation of `TenantQueueManagement` feature gate to beta.

---

## Alternatives

1. **Opt-in isolation via a shared `__default__` queue** (the original design in this proposal). Namespaces without a `TenantQueue` would share one queue. Rejected: it lets a single namespace unilaterally increase its own scheduling share simply by creating an object, recreating the gaming incentive the proposal sets out to remove. See [Design Notes](#why-automatic-sharding-instead-of-opt-in-isolation).
2. **A new `scheduling.karmada.io` API group.** Considered so scheduling-related types have a dedicated home. Rejected for Phase 1: a new API group is a larger footprint than one alpha type justifies — CRD packaging, RBAC, `karmadactl`, and aggregated-apiserver wiring — and `policy.karmada.io` already hosts `FederatedResourceQuota`, which this proposal already aligns scope with.
3. **A cluster-scoped queue-topology resource** instead of a namespace-scoped `TenantQueue`, giving cluster admins full ownership of sharding and weights. Deferred rather than rejected: Phase 1 only needs per-namespace strategy selection, and Phase 2's weighted round-robin already plans a cluster-admin-facing control that can absorb topology ownership at that point if needed.

---

## Open Questions

1. **Weighted round-robin?** The current round-robin gives each tenant equal scheduling turns. Should tenants with larger `FederatedResourceQuota` allocations get proportionally more turns? This would be cluster-admin controlled (not configurable in `TenantQueue` itself). Deferred to Phase 2.

2. **ClusterResourceBinding**: `ClusterResourceBinding` objects are cluster-scoped (no namespace). They always use the global `__default__` queue. The proposal does not change their handling.

3. **Idle namespace-queue GC interval**: what threshold balances resource cleanup in high-churn clusters against the cost of recreating a queue (and losing its `blocked` state) for a namespace that resumes activity shortly after being GC'd? Needs a default backed by benchmarking, exposed as a scheduler flag rather than hardcoded.
