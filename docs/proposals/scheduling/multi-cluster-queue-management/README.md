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

This assumes namespaces are provisioned by a cluster admin, not self-served by tenants. Making the shard unit the namespace moves the gaming lever up one level rather than removing it outright: a tenant that can create namespaces at will can still buy itself more scheduling turns by spreading the same work across more of them. That is a weaker lever than a single opt-in object — namespace creation is normally an admin-gated, auditable act tied to quota and RBAC — but clusters that delegate namespace creation to tenants should pair this feature with Phase 2's admin-controlled weighting rather than relying on equal per-namespace turns alone.

### Goals

- Make the scheduler's `activeQ`, `backoffQ`, and `unschedulableBindings` per-namespace by default, with no object required.
- Introduce a namespace-scoped `TenantQueue` API (`policy.karmada.io/v1alpha1`) so a namespace can opt into `StrictFIFO` ordering for its own queue.
- Support `BestEffortFIFO` (default) and `StrictFIFO` ordering modes.
- Maintain backwards compatibility: behavior for existing clusters is unchanged until the `TenantQueueManagement` feature gate is enabled.

### Non-Goals

- Changes to the `backoffQ` or `unschedulableBindings` data structures themselves. The `ActiveQueue` interface does gain a non-blocking pop — see [Pop() with Kueue-Inspired Heads Pattern](#pop-with-kueue-inspired-heads-pattern).
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

**Routing:** the namespace is extracted from the `NamespacedKey` of each `QueuedBindingInfo`. If no queue exists yet for that namespace, one is created on demand with `BestEffortFIFO` before the binding is pushed. If a `TenantQueue` object exists for the namespace, its strategy is applied to that queue instead. `ClusterResourceBinding` objects (no namespace) always go to `__default__`. Namespace queues that stay idle for longer than a GC interval are torn down and recreated lazily on the next push — see [Idle Queue Lifecycle](#idle-queue-lifecycle) and [Risks and Mitigations](#risks-and-mitigations).

**Flush loops:** `prioritySchedulingQueue.Run()` today starts two goroutines per queue — a 1s `flushBackoffQCompleted` and a 30s `flushUnschedulableBindingsLeftover`. Starting those per shard would mean two goroutines and a one-second ticker for every namespace the control plane has ever scheduled, which is not acceptable once sharding is the default. Instead, `TenantSchedulingQueue` owns a single pair of flush loops on the same two intervals; each tick walks the shard map and runs the corresponding flush on each shard. Shards are therefore inert data structures with no lifecycle of their own, which also removes the `Run()`/`Close()` race that per-shard goroutines would create against GC (`Close()` closes the shard's `stop` channel, so a GC racing a shutdown would double-close it).

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

This ensures each tenant gets one scheduling turn per cycle regardless of how many bindings they have queued, preventing burst monopolization. The scheduler runs a single worker goroutine, so `Pop()` has one consumer and a collected head is always served before the next collection round.

**Non-blocking pop.** `ActiveQueue.Pop()` blocks on a per-queue `sync.Cond` until that queue has an item, which is the wrong primitive once there are many shards. `ActiveQueue` gains a `TryPop() (*QueuedBindingInfo, bool)` that returns immediately when the shard is empty; blocking `Pop()` is left untouched so the unsharded path is unaffected.

**Blocking without polling.** Because each shard has its own `sync.Cond`, `TenantSchedulingQueue` cannot wait on any one of them, and a retry loop over every shard would burn CPU proportional to the namespace count. `TenantSchedulingQueue` therefore owns one condition variable of its own, broadcast whenever any shard transitions from empty to non-empty (push, backoff flush, unschedulable flush, or a StrictFIFO block being cleared). Step 3 waits on that condition; a collection round only walks the shard map after it has been signalled that there is something to find.

### StrictFIFO Mode

Both modes sort by **priority descending, then by a timestamp ascending**. They differ in which timestamp they use and in what happens when the head fails:

- **`BestEffortFIFO`** (default): orders by `QueuedBindingInfo.Timestamp`, the existing activeQ key, which is reset to "now" every time the binding is enqueued or re-enqueued. If the head-of-queue binding fails scheduling, it is moved to `backoffQ` or `unschedulableBindings`. The next binding in that tenant's queue is tried in the following cycle.
- **`StrictFIFO`**: orders by `QueuedBindingInfo.InitialAttemptTimestamp`, which is set on first enqueue and never updated. If the head-of-queue binding fails scheduling, the **entire tenant queue is blocked** — no later binding from that tenant is considered until that same binding is re-promoted to `activeQ`. This is head-of-line (HOL) blocking, matching Kueue's semantics.

`StrictFIFO` cannot reuse `Timestamp`: since it is reset on every re-enqueue, a binding that fails, backs off, and returns to `activeQ` sorts *behind* everything queued in the meantime. The queue would unblock and then immediately hand out a different binding, which is the opposite of the ordering guarantee StrictFIFO exists to provide. `InitialAttemptTimestamp` survives requeues and gives arrival order, matching Kueue's use of creation time. Implementation note: `activeQ` is a heap built with a `Less` function supplied at construction (`heap.NewWithRecorder`), so this is a per-shard `Less` chosen from the shard's strategy, not a change to the shared `Less` in `pkg/scheduler/internal/queue/types.go`. Backoff duration continues to be computed from `Timestamp` in both modes.

HOL blocking is tracked via a flag on the tenant entry that records the identity of the binding currently blocking it. The flag is cleared only when *that specific binding* lands back in `activeQ` (backoff expiry, unschedulable flush, or cluster state change) — an unrelated new binding pushed into the same tenant's `activeQ` must not clear it. Clearing on any push would let a tenant route around its own head-of-line block simply by submitting new work, defeating the guarantee.

### Idle Queue Lifecycle

A shard is eligible for garbage collection once it has been **idle** for longer than the GC interval. Idle means all five of the following hold:

1. `activeQ` is empty,
2. `backoffQ` is empty,
3. `unschedulableBindings` is empty,
4. the shard has no **in-flight** bindings, and
5. the shard is not currently blocked by StrictFIFO.

Conditions 4 and 5 matter as much as the first three. `activequeue` tracks two sets beyond its heap — `processingBindings` (popped and currently being scheduled) and `dirtyBindings` (pushed while processing, to be requeued by `Done()`). A binding that has been popped and handed to the worker is in *none* of the three queues, so a GC predicate that only checks emptiness would tear down a shard that is actively scheduling. Two things break if it does: the `Done()` call at the end of the scheduling cycle lands on a discarded queue, and any update that arrived during the cycle is lost with the `dirtyBindings` entry instead of being requeued. Condition 5 preserves the StrictFIFO block across the idle window, since the block is per-shard state that would otherwise be silently dropped and recreated as unblocked.

Reaping is done by the same shard-map walk as the flush loops, so GC costs nothing extra in goroutines. A reaped namespace is recreated lazily, with `BestEffortFIFO` or with its `TenantQueue` strategy if one exists, on its next push.

### Observability

`pkg/scheduler/metrics/queue` exposes a single `pending_bindings` gauge vector labelled only by queue type (`active`, `backoff`, `unschedulable`), and every `prioritySchedulingQueue` holds a recorder writing to it. Sharding breaks that in both directions and must be handled as part of Phase 1:

- `unschedulableBindings.clear()` calls `MetricRecorder.Clear()`, which is a `Set(0)` on the shared gauge. Tearing down one shard would zero the unschedulable count for every other tenant.
- Conversely, discarding a shard that still holds counted items never `Dec()`s them, so the gauge ratchets upward.

With GC in the picture both happen routinely, so the recorders cannot stay global-by-construction. Phase 1 makes the shard's contribution explicit: each shard tracks its own counts and the `TenantSchedulingQueue` publishes the sum, so a shard close subtracts exactly what that shard held rather than zeroing or leaking. Per-namespace metric labels are deliberately *not* added — label cardinality would grow with namespace count, which is the very dimension this proposal makes unbounded. Per-tenant visibility (queue depth, turns taken, starvation) is left to Phase 2, where it can be introduced together with weighting and with an explicit cardinality bound.

---

## Ordering Semantics Comparison

| Property | BestEffortFIFO | StrictFIFO |
|---|---|---|
| `activeQ` sort key | Priority desc, then `Timestamp` asc (last enqueue) | Priority desc, then `InitialAttemptTimestamp` asc (first enqueue) |
| Head-of-queue blocked? | Skip, try next binding | Block entire tenant queue (HOL blocking) |
| Throughput | Higher | Lower (head-of-line blocking) |
| Ordering guarantee | Best effort; a requeued binding goes to the back of its priority band | Arrival order within a priority band, preserved across requeues |
| Typical use case | Interactive / heterogeneous batch | Sequential pipelines, strict ordering |

---

## Design Notes

### Why Namespace-Scoped

`TenantQueue` is namespace-scoped because tenant = namespace = `FederatedResourceQuota` scope in Karmada's model. This eliminates the need for a selector field and allows namespace admins to manage their own queue settings. The namespace identity is sufficient to route bindings without any indirection.

### Why Automatic Sharding Instead of Opt-In Isolation

An earlier version of this proposal made `TenantQueue` the trigger for isolation: namespaces without one shared a single `__default__` queue. That design reintroduces the exact problem in the Motivation — a namespace could unilaterally grant itself a larger share of scheduling turns just by creating an object, since it would then compete one-per-cycle against every other unisolated namespace combined. Sharding by namespace unconditionally removes that lever; `TenantQueue` is scoped down to configuring ordering strategy only. This holds only as far as namespace creation is admin-gated — see the caveat in [Motivation](#motivation).

### Comparison to Kueue

Kueue has a three-level hierarchy: `LocalQueue` (namespaced) → `ClusterQueue` (cluster-scoped) → `Cohort`. Karmada's model merges these into a single namespace-scoped `TenantQueue`, since quota enforcement lives in `FederatedResourceQuota` rather than in the queue itself. There is no borrowing, no resource flavors, and no cohort concept. Unlike Kueue, where a workload without a matching `LocalQueue` cannot be admitted at all, every Karmada namespace is schedulable without any object.

Cross-tenant scheduling fairness is achieved by the Heads pattern (one binding per tenant per cycle). Kueue adds DRS (Dominant Resource Share) tournament ordering on top of this for the fair-sharing iterator; Karmada Phase 1 uses simple round-robin across tenant heads.

### Feature Gate

Gated behind `TenantQueueManagement` (alpha, disabled by default). It builds on `PriorityBasedScheduling`, which has since graduated to beta and is **enabled by default**, so that dependency is satisfied out of the box and `TenantQueueManagement` is the only switch an administrator has to flip. The scheduler still verifies the dependency at startup and refuses to enable `TenantQueueManagement` if `PriorityBasedScheduling` has been explicitly disabled, since the sharded queues wrap `prioritySchedulingQueue` and have no meaning on the legacy rate-limiting workqueue path.

This also raises the bar for the change: because the priority queue is now the default code path rather than an opt-in one, a regression in the sharded queue reaches any cluster that enables the alpha gate, not just clusters that had already opted into priority scheduling.

---

## Relationship to Existing Features

| Feature | Relationship |
|---|---|
| `PriorityBasedScheduling` feature gate | Required, and beta / on by default as of v1.20. Per-namespace isolation is automatic on top of it; `TenantQueue` additionally configures ordering strategy. |
| `FederatedResourceQuota` | Aligns scope and API group: one queue per namespace mirrors one `FederatedResourceQuota` per namespace, and both live under `policy.karmada.io`. |
| [Binding preemption](../binding-preemption/binding-preemption.md) | Complementary, but the two read priority differently — see [Risks and Mitigations](#risks-and-mitigations). |

---

## Risks and Mitigations

- **Priority is tenant-local for queueing but global for preemption.** Round-robin across tenants means a low-priority binding in one namespace can be popped before a higher-priority binding in another namespace that hasn't had its turn yet in the current cycle. This is called out explicitly as a Non-Goal rather than fixed in Phase 1; cross-tenant weighting is deferred to Phase 2. The wrinkle is that [binding preemption](../binding-preemption/binding-preemption.md) reads the same `SchedulePriority` field globally: once a binding is popped, its priority governs preemption against bindings on member clusters *regardless of which namespace those victims belong to*. So a tenant's priority value buys it cross-tenant power at preemption time but only tenant-local ordering at queueing time. That is not a correctness conflict — this risk affects *how soon* a binding is attempted, not what it can preempt once it is — but the two features should be documented together so operators are not surprised that raising a priority speeds up preemption without speeding up dequeue. Phase 2's weighting is where the two can be reconciled.
- **Unbounded queue cardinality.** Creating a shard per namespace ever observed, with no cleanup, would leak resources in clusters with high namespace churn. Two mitigations: the flush loops are centralized on the `TenantSchedulingQueue` rather than per shard, so a shard costs a map entry and three (mostly empty) containers instead of two goroutines and a one-second ticker; and idle shards are reaped. See [Idle Queue Lifecycle](#idle-queue-lifecycle) for the idleness predicate — note in particular that emptiness alone is not a safe test — and the Open Questions below for the interval.
- **Shared queue-depth metrics are not shard-aware.** The existing `pending_bindings` gauge is global and is zeroed by `Clear()`, so naive sharding would corrupt it on every shard teardown. See [Observability](#observability).
- **StrictFIFO block must not be clearable by unrelated work.** Keying the block flag to the specific blocked binding's identity (rather than "something happened") is a deliberate, testable invariant — enforced with a unit test that pushes an unrelated new binding into a blocked StrictFIFO tenant and asserts the queue stays blocked.

---

## Test Plan

- Unit tests for `TenantSchedulingQueue`: lazy per-namespace queue creation, routing by namespace, round-robin fairness across a mix of explicit and implicit queues, `ClusterResourceBinding` routing to `__default__`, and that `Pop()` blocks rather than spins when every shard is empty and wakes on a push to any shard.
- Unit tests for `StrictFIFO`: block set on scheduling failure, block held across an unrelated push to the same tenant, block cleared only when the specific blocked binding re-enters `activeQ`, and that a binding which fails and completes backoff returns to the head of its priority band rather than the tail (the `InitialAttemptTimestamp` ordering).
- Unit tests for idle-shard GC: a shard with an in-flight binding is not reaped; `Done()` after a quiescent period still requeues a binding that was updated mid-cycle; a StrictFIFO-blocked shard is not reaped; a reaped shard is recreated with the right strategy on the next push.
- Unit tests for metrics accounting: closing one shard leaves other shards' contributions to `pending_bindings` intact, and the gauge returns to zero after all shards drain.
- Unit tests for the `TenantQueue` validating webhook: singleton name enforcement.
- Integration test simulating a burst from one namespace alongside steady traffic from others, asserting no namespace is starved beyond one scheduling cycle.
- Benchmark establishing the per-shard cost at high namespace counts, to back the GC interval default.

---

## Implementation Plan

### Phase 1: Queue Sharding with BestEffortFIFO and StrictFIFO (Alpha)

1. Add `TenantQueue` API type under `policy.karmada.io/v1alpha1` (namespace-scoped, singleton name `queue`).
2. Add validating webhook to enforce the singleton name.
3. Implement `TenantSchedulingQueue` wrapping multiple `prioritySchedulingQueue` instances, created lazily per namespace with `BestEffortFIFO` by default, with the backoff and unschedulable flush loops centralized on the wrapper rather than started per shard.
4. Add `TryPop()` to `ActiveQueue` and implement Heads-pattern `Pop()` with round-robin across tenant queues, blocking on a wrapper-owned condition variable signalled by shard empty-to-non-empty transitions.
5. Implement `StrictFIFO` with a per-shard `Less` keyed on `InitialAttemptTimestamp` and a block flag keyed to the specific blocked binding, cleared only when that binding re-enters `activeQ`.
6. Add informer watch for `TenantQueue` in the scheduler to apply strategy overrides; route bindings by namespace with lazy queue creation.
7. Make `pending_bindings` accounting shard-aware so shard teardown neither zeroes nor leaks the gauge.
8. Add idle namespace-queue garbage collection, using the idleness predicate in [Idle Queue Lifecycle](#idle-queue-lifecycle).
9. Feature gate: `TenantQueueManagement` (disabled by default), with a startup check that `PriorityBasedScheduling` has not been explicitly disabled.

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

3. **Idle namespace-queue GC interval**: what threshold balances resource cleanup in high-churn clusters against the cost of recreating a shard for a namespace that resumes activity shortly after being reaped? With centralized flush loops an idle shard is cheap, which argues for a generous interval. Needs a default backed by benchmarking, exposed as a scheduler flag rather than hardcoded.
