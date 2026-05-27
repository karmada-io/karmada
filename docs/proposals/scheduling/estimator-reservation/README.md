---
title: Scheduling Overcommit Protection
authors:
- "@XiShanYongYe-Chang"
reviewers:
- "@RainbowMango"
- "@GitHubxsy"
- "@mszacillo"
- "@zhzhuang-zju"
- "@vie-serendipity"
approvers:
- "@RainbowMango"
creation-date: 2026-04-15

---

# Scheduling Overcommit Protection

## Summary

The Karmada scheduler processes tasks sequentially, but it relies on the `karmada-scheduler-estimator` (Estimator) to figure out available cluster capacity. The Estimator calculates this by looking at Pods that are already bound to a node (`pod.Spec.NodeName != ""`).

Because of this, there's a timing gap. Once Karmada assigns a workload to a cluster, it takes time for the workloads to be distributed, created, and eventually bound to nodes in the member cluster. If new scheduling requests come in during this window, the Estimator is using a "stale snapshot"—it doesn't know about the resources Karmada *just* handed out. This leads to over-committing cluster resources.

To fix this, we're introducing **Scheduling Overcommit Protection**:
1. **Track Assumed Workloads in Scheduler**: Once the scheduler makes a decision, it caches this decision as "assumed" resources locally.
2. **Deduct Assumed Workloads in Estimator**: When querying for available replicas, the scheduler sends these assumed workloads to the Estimator. The Estimator deducts them from its local view (using the First Fit algorithm) before returning the capacity.

## Motivation

The karmada-scheduler processes the scheduling queue serially with a single worker, but its
throughput far exceeds the rate at which member clusters synchronize resource state and the rate at
which Pods are created and bound to nodes. When two ResourceBindings are scheduled in rapid
succession, the following problem occurs:

```
Timeline  Scheduler Worker                          Estimator Snapshot (ClusterA)
T1        Binding1: MaxAvailableReplicas → 10       Snapshot: 0 Pods bound, available = 10
T2        Binding1: decision → assign 10 → ClusterA
T3        Binding1: Patch karmada-apiserver ✓
T4        Binding2: MaxAvailableReplicas → 10       Snapshot: still 0 Pods (Binding1's Pods not yet bound)
T5        Binding2: decision → assign 10 → ClusterA
T6        Binding2: Patch karmada-apiserver ✓       ClusterA is now over-committed!
...
T+N s     work distribution → member cluster creates Pods (Pending) → kube-scheduler binds to node
          → Estimator observes NodeName, node snapshot finally updated
```

After completing the Patch at T3, the scheduler immediately processes Binding2. At this point,
Binding1's Pods have not even been created in the member cluster, let alone bound to a node by
kube-scheduler.

The same cluster capacity is consumed by multiple scheduling decisions in succession, leading to
over-commitment of cluster resources.

This proposal solves over-commitment by introducing an "assume and deduct" flow between the scheduler and the Estimator.

### Goals

- Prevent cluster resources from being over-committed across multiple consecutive scheduling
  decisions targeting the same cluster.

### Non-Goals

- The Estimator produces node-selection results identical to kube-scheduler.
- Address resource contention in `Duplicated` scheduling mode (that mode does not involve replica
  count calculation).
- Eliminate the propagation delay between the API Server and the Estimator's node snapshot.

## Proposal

### Feature Gate

This feature introduces a new feature gate: `SchedulingOvercommitProtection`.
- **Default State**: Enabled (Beta) / Disabled (Alpha) depending on graduation plan.
- **Component**: `karmada-scheduler` and `karmada-scheduler-estimator`.
- **Behavior**: When enabled, the scheduler tracks optimistic constraints in memory and attaches them down to Estimator's RPC invocations, and the Estimator will perform resource deduction on the presumed usages.

### User Stories

#### Story 1

As a platform operator, I create two Deployments simultaneously with an `Aggregated` scheduling
policy, and the target cluster can only accommodate one of them. I expect the second Deployment to
be scheduled to another eligible cluster (if one exists), or to receive a clear resource-shortage
error — not for both to be scheduled to the same cluster, leaving Pods silently Pending.

## Design Details

### Overall Architecture

```
karmada-scheduler (leader election, single active instance)
│
├── AssigningResourceBindingCache               [EXTENDED]
│     Added: bindingKey → BindingAssumption{entries, expiresAt}
│
├── calculateMultiTemplateAvailableSets()         [MODIFIED]
│     Passes the current cluster's submitted-but-not-yet-landed assumptions
│     as assumedWorkloads in the ComponentSetEstimationRequest
│
├── patchScheduleResultForResourceBinding()      [MODIFIED]
│   After successful Patch → delta = newReplicas - oldReplicas
│   delta > 0  → Assume(bindingKey, clusterName, entry)  // REPLACE semantics
│   delta <= 0 → no-op
│   cluster removed → ReleaseClusterAssumption(bindingKey, clusterName)
│
event_handler.go                                [MODIFIED]
ResourceBinding AggregatedStatus Healthy → ReleaseClusterAssumption(bindingKey, clusterName)
TTL GC fallback

karmada-scheduler-estimator
│
├── gRPC MaxAvailableComponentSets()[MODIFIED: MaxAvailableComponentSetsRequest adds assumedWorkloads]
│
├── nodeResource plugin (EXTENDED)
│     1. Obtain available nodes (cloned, Allocatable = Allocatable - Requested)
│     2. Convert assumedWorkloads to Component list; consume node resources via FF algorithm
│     3. Compute available replicas on the post-deduction node state
│
└── resourcequota plugin (EXTENDED)
      1. Compute total resource consumption of assumedWorkloads (replicas × resourceRequest)
      2. Subtract assumed consumption from ResourceQuota available capacity
      3. Compute available replicas on the post-deduction quota state
```

### 1. Extend gRPC Interface (`pkg/estimator/pb/generated.proto`)

Add `AssumedWorkload` message and an `assumedWorkloads` field to
`MaxAvailableComponentSetsRequest` in the proto definition:

```protobuf
message AssumedWorkload {
    // namespace is the namespace to which this assumption belongs.
    // The resourcequota plugin uses this field to determine which ResourceQuota objects
    // should have this assumption's resource consumption deducted from their available capacity.
    optional string namespace = 1;
    // components contains per-component replica and resource requirements.
    repeated Component components = 2;
}

// MaxAvailableComponentSetsRequest extended: new assumedWorkloads field.
message MaxAvailableComponentSetsRequest {
    ...
    // assumedWorkloads represents the scheduling commitments that have been
    // patched to the API server but not yet reflected in the member cluster's real resource state.
    repeated AssumedWorkload assumedWorkloads = 4;
}
```

> After modifying the `.proto` file, regenerate the Go code:
> `hack/update-estimator-protobuf.sh`.

Also update the estimator client interface (`pkg/estimator/client/interface.go`). The
`ReplicaEstimator` interface's `MaxAvailableComponentSets` method already uses
`ComponentSetEstimationRequest` which is designed for field extension without signature changes.
Add a `AssumedWorkloads` field to that request struct:

```go
// ComponentSetEstimationRequest extended: new AssumedWorkloads field.
type ComponentSetEstimationRequest struct {
    ...

    // AssumedWorkloads carries in-flight scheduling commitments that have not yet been
    // reflected in the estimator's node snapshot.
    // +optional
    AssumedWorkloads []pb.AssumedWorkload
}
```

> `SchedulerEstimator` (`pkg/estimator/client/accurate.go`) implements this interface. It receives
> `assumedWorkloads` from the scheduler and populates the corresponding gRPC request field
> directly, without performing any cache read/write of its own.

### 2. Scheduler Side: Extend `AssigningResourceBindingCache` (`pkg/scheduler/cache/cache.go`)

`AssigningResourceBindingCache` already tracks ResourceBindings that have been "submitted to the
API Server but not yet caught up by the Informer". Its lifecycle closely matches the assumption
lifecycle required by this proposal, so we extend it directly rather than introducing a new cache
structure.

**Extension strategy: add an independent `assumptions` field alongside the existing `items`
field, with two separate lifecycle management paths:**

- `items` (existing): stores full ResourceBinding objects, serving the `WorkloadAffinity` feature.
  Deleted immediately when the Informer catches up and `FullyApplied = True`. Lifecycle logic is
  **unchanged**.
- `assumptions` (new): stores incremental assumption data per ResourceBinding, serving this
  proposal. Released via `ReleaseAssumption()` when the ResourceBinding's AggregatedStatus
  reaches Healthy, or via TTL GC as a fallback. Lifecycle is **independent of** `items`.

```go
// BindingAssumption stores the submitted-but-not-yet-landed replica increments for a
// ResourceBinding across its target clusters.
type BindingAssumption struct {
    // entries: clusterName → AssumedWorkload
    // The entry key is always clusterName.
    entries   map[string]*AssumedWorkload
    // ExpiresAt is the TTL deadline for this binding's assumption, set when the entry is
    // written or updated (default: now + 5 minutes). Under normal conditions the assumption is
    // proactively released when the ResourceBinding's AggregatedStatus for the corresponding
    // cluster reaches Healthy via ForgetAssumption().
    // However, if a downstream failure (e.g. persistent work distribution failure) prevents the
    // Healthy state from ever being reached, the proactive release is never triggered.
    // ExpiresAt serves as a safety net: a background GC() periodically scans and removes all
    // expired entries, ensuring that stale assumptions do not linger in the cache and skew
    // subsequent scheduling estimates.
    ExpiresAt time.Time
}

// AssumedWorkload represents the submitted-but-not-yet-landed scheduling assumption for a
// specific cluster.
type AssumedWorkload struct {
    // Namespace is the namespace of the assumed workload. GetAssumedWorkloads() copies this
    // value into AssumedWorkload.Namespace so that the resourcequota plugin can determine
    // which ResourceQuota objects should have this assumption deducted.
    Namespace string
    // Components contains per-component replica and resource requirements.
    // Each Component.Replicas is set to the replica increment (delta).
    Components []workv1alpha2.Component
}

// AssigningResourceBindingCache (extended)
type AssigningResourceBindingCache struct {
    ...

    // assumptions: bindingKey → *BindingAssumption (new field, serves this proposal)
    assumptions map[string]*BindingAssumption
    // ttl is the lifetime of an assumption entry, used by Assume() to compute ExpiresAt.
    // Default 5 minutes; configurable via scheduler flag --assumption-ttl.
    ttl          time.Duration
}
```

**Why TTL is necessary.**
Releasing an assumption depends on the entire downstream pipeline completing successfully. If a
downstream failure prevents the workload from ever reaching `Healthy` state, the assumption can
never be proactively released, continuously skewing subsequent scheduling estimates. Example
failure scenarios (non-exhaustive):

1. **Persistent Work distribution failure**: If the work controller cannot apply the manifest to
   the member cluster (e.g. RBAC misconfiguration, member cluster unreachable, APIServer
   connection timeout), the resource is never created in the member cluster, the ResourceBinding's
   AggregatedStatus for that cluster never transitions to `Healthy`, and the assumption is never
   proactively released.
2. **Workload fails to generate Pods**: The manifest has been successfully applied to the member
   cluster, but the workload controller cannot create Pods (e.g. the Deployment references a
   non-existent ServiceAccount, resource quota is exhausted causing the Pod to be rejected by
   admission, LimitRange validation fails). Since no Pod is ever created, kube-scheduler cannot
   perform node binding, the Estimator snapshot does not reflect any resource consumption, and
   the `Healthy` state is never reached.
3. **Pods fail to run normally**: Pods have been created and bound to nodes but never reach
   Running+Ready (e.g. image pull failure, CrashLoopBackOff, readiness probe persistent failure,
   init container stuck). The `Healthy` condition cannot be satisfied, so the assumption lingers
   in the cache. Note: in this scenario, the Estimator snapshot **already** reflects the Pod's
   resource consumption (because `pod.Spec.NodeName != ""`), so the assumption is effectively
   no longer necessary — TTL expiration cleans it up automatically.

Without TTL, any of the above scenarios would cause the assumption to persist indefinitely,
continuously deducting from the cluster's estimated available capacity and progressively reducing
the scheduler's willingness to place workloads on that cluster — effectively creating a resource
"leak" in the scheduling view. TTL acts as a bounded-time safety net: even in the worst case, a
stale assumption is automatically cleaned up within a predictable time window (default 5 minutes),
ensuring the scheduling view self-heals.

**New methods:**

| Method                                                          | Description                                                                                                                                        |
|-----------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| `Assume(bindingKey, clusterName string, entry AssumedWorkload)` | Called when delta > 0 after a successful Patch; refreshes ExpiresAt; writes with REPLACE semantics                                                 |
| `ReleaseClusterAssumption (bindingKey, clusterName string)`     | Called immediately when a cluster is removed from the placement; deletes that cluster's assumption entry                                           |
| `ReleaseReservation(bindingKey string)`                         | Triggered when the ResourceBinding's `AggregatedStatus` entries for all reserved clusters reach `Healthy`; deletes all assumptions for the binding |
| `GetAssumedWorkloads(clusterName string) []AssumedWorkload`     | Called by `calculateMultiTemplateAvailableSets()` when constructing the gRPC request                                                               |
| `GC()`                                                          | Runs in the background every minute; removes all entries in `assumptions` whose `ExpiresAt` has passed                                             |

### 3. Scheduler Side: Modify `calculateMultiTemplateAvailableSets()` (`pkg/scheduler/core/estimation.go`)

`AssumedWorkloads` is obtained by the scheduler before calling the Estimator via
`AssigningResourceBindings().GetAssumedWorkloads(cluster)`. It is populated in
`ComponentSetEstimationRequest.AssumedWorkloads` and forwarded to the gRPC request body by
`SchedulerEstimator`.

> Note: `calAvailableReplicas` (`pkg/scheduler/core/util.go`) is the scheduling entry point that
> dispatches to `calculateMultiTemplateAvailableSets` for multi-template workloads.
> `calAvailableReplicas` is currently a plain function with no direct access to `schedulerCache`.
> It can be refactored into a method on Scheduler, or the assumption data can be threaded through
> as a parameter further up the call chain.

### 4. Scheduler Side: Modify `patchScheduleResultForResourceBinding()` (`pkg/scheduler/scheduler.go`)

After a successful API Server Patch, update the assumption based on
`delta = newReplicas - oldReplicas`:

- **delta > 0**: Construct an `AssumedWorkload` and call `Assume()` with REPLACE semantics to
  record the newly committed but not-yet-landed replicas (increments only, to avoid
  double-counting with already-running replicas in the Estimator snapshot):
  iterate `spec.Components`, compute the replica increment for each Component
  (new allocated − old allocated), and populate `entry.Components`.
- **delta ≤ 0**: No-op; any existing assumption is cleaned up by the TTL GC within 5 minutes.
- **Cluster removed from placement**: Call `ReleaseClusterReservation()` to immediately clear the
  old assumption for that cluster.

### 5. Scheduler Side: Release Assumptions (`pkg/scheduler/event_handler.go`)

#### Choosing the Release Trigger

When to release assumptions is a critical design decision. The core question is: **when can the
Estimator's node snapshot be expected to naturally reflect the resource consumption of the
workload, making the assumption no longer necessary?**

The Estimator's Pod informer caches only Pods where `pod.Spec.NodeName != ""`. Once a Pod is bound
to a node by the member cluster's kube-scheduler — regardless of whether it is Running — its
resource consumption is counted in `node.Requested`. Therefore, the assumption can be safely
released when: **the member cluster's kube-scheduler has bound the Pod to a node and the Estimator
informer has observed that event**.

Three options are considered:

**Option A: `FullyApplied = True` + fixed delay**

`FullyApplied = True` means the manifest has been successfully applied to the member cluster
kube-apiserver, but the Pod may not yet have been bound to a node by kube-scheduler. A fixed delay
`Δt` is introduced to provide a buffer for Pod binding and Estimator informer propagation.

```
Pros: Simple to implement; no additional resource watches required.
Cons:
1. Δt is an empirical value that is hard to set correctly: too small (e.g. 5s) leaves a window
   under resource pressure or slow image pulls; too large (e.g. 60s) causes assumptions to
   unnecessarily consume estimation headroom for an extended period.
2. Actual scheduling latency varies significantly across clusters, scales, and network environments;
   a single fixed value cannot cover all scenarios.
```

**Option B: Watch member cluster Pod `NodeName` assignment (most precise)**

Trigger release precisely by detecting through the member cluster Pod informer that a Pod has been
bound to a node.

```
Pros: Most accurate release timing; shortest protection window; no empirical values needed.
Cons: Requires the Karmada control plane to directly observe member cluster Pod state — high
      architectural invasiveness and implementation complexity.
```

**Option C: Watch ResourceBinding status changes, check `AggregatedStatus[].Health` (adopted)**

The resource interpreter provides a `Healthy` hook to determine whether a resource on the member
cluster has reached a healthy state. Work health status is aggregated onto the ResourceBinding by
the `RBStatusController` into `ResourceBindingStatus.AggregatedStatus[].Health`. By watching
ResourceBinding status changes and checking the `Health` field of each `AggregatedStatusItem`:
when the AggregatedStatus entry for a given cluster is `Healthy`, the workload's Pods on that
cluster are all Running, and it is safe to release the assumption.

**Assessment:**

From a safety standpoint, this option guarantees that releasing the assumption does not cause
over-commitment — when `health=Healthy`, all Pods are Running, whereas the Estimator only
requires `pod.Spec.NodeName != ""` to count resource consumption, so the Estimator snapshot will
already reflect the actual consumption well before this point. Option C's safety argument holds,
but it is more conservative than strictly necessary: the event we actually need to wait for is
NodeName assignment, not Pod readiness — the gap between these two events covers image pulling,
container startup, etc., and may span several minutes. Nevertheless, compared with Option A's
reliance on a hard-to-calibrate empirical `Δt`, Option C's use of a clear business-semantic signal
makes its behavior predictable and tuning-free, which provides greater engineering reliability.

Implementation considerations:
1. karmada-scheduler already watches ResourceBinding objects, so no additional informer is needed;
   the release logic can be added to the existing ResourceBinding update event handler.
2. In multi-cluster scenarios, a single ResourceBinding's AggregatedStatus contains per-cluster
   health entries; release is performed per cluster as each cluster reaches Healthy.
3. When a resource has no registered `Healthy` hook, the `health` value defaults to Healthy,
   causing premature release; fall back to TTL GC in that case (see Risks and Mitigations).

```
Pros: Release is driven by a clear business-semantic signal; behavior is deterministic and
      requires no tuning; safety is guaranteed; no additional informer needed (reuses existing
      ResourceBinding watch).
Cons:
1. More conservative than strictly required by the Estimator; assumption window may be several
   minutes when Pod startup is slow.
```

### 6. Estimator Side: Assumption Deduction

Assumption deduction logic is implemented inside the existing estimation framework plugins
(`pkg/estimator/server/framework/plugins/`). Two plugins are currently involved: **nodeResource**
and **resourcequota**. The plugin interface is extended with a uniform `assumedWorkloads`
parameter; each plugin independently performs deduction in its `EstimateComponents`
method before computing the final estimate.

> For the autoscale plugin currently under discussion (https://github.com/karmada-io/karmada/issues/7375),
its assumed workload deduction logic is largely consistent with the nodeResource plugin. Adaptation
will be addressed separately once that plugin is added.

#### 6.1 `AssumedWorkload` and `Component` Representation

`AssumedWorkload` uses `Components` (a `[]pb.Component` list) as its representation.
The Estimator can use `rw.Components` directly — no additional conversion is needed.

#### 6.2 nodeResource Plugin Deduction

The `nodeResource` plugin uses the First Fit (FF) algorithm to consume assumed resources at the
**node level**. Implementation is in
`pkg/estimator/server/framework/plugins/noderesource/noderesource.go`.

**`EstimateComponents` flow:**

1. Call `getNodesAvailableResources(snapshot)` to obtain a cloned node list. Each node's
   `Allocatable` has already been adjusted to `Allocatable − Requested`, reflecting the true
   available resources.
2. If `assumedWorkloads` is non-empty, collect the `Components` from each entry into a flat list,
   and call `SchedulingSimulator{nodes}.SimulateScheduling(assumedWorkloadComponents, 1)` to
   consume assumed resources on nodes (the FF algorithm selects nodes in first-fit order and
   modifies `node.Allocatable`).
3. On the post-deduction node state, call
   `SchedulingSimulator{nodes}.SimulateScheduling(components, math.MaxInt32)` to obtain the number
   of complete component sets that can be scheduled.

**Key design notes:**

- `SchedulingSimulator` (existing implementation) is reused directly — **no new simulation logic
  is introduced**.
- The FF algorithm consumes resources by modifying `node.Allocatable`; subsequent estimation also
  reads `node.Allocatable`, keeping the two phases consistent.
- The node list is cloned in `getNodesAvailableResources`; deduction does not affect the original
  snapshot.
- When the number of reserved replicas exceeds the total available capacity across all nodes, the
  FF algorithm stops naturally (no negative values are produced), and the estimated result is 0 —
  conservative but safe.

#### 6.3 resourcequota Plugin Deduction

The `resourcequota` plugin deducts reserved resource consumption at the **quota level**.
Implementation is in
`pkg/estimator/server/framework/plugins/resourcequota/resourcequota.go`.

**Deduction approach:**

For each `AssumedWorkload`, compute the total resource consumption and subtract it from the
ResourceQuota's available capacity:

`totalConsumed = Σ (Component.Replicas × Component.ResourceRequest)` summed over all components

After subtracting `totalConsumed` from the available resources (`Hard − Used`), proceed with
`MaxDivided` estimation. If any resource dimension becomes negative after deduction, clamp it to 0
(quota exhausted; estimated result is 0).

**`EstimateComponents` flow:**

1. List all ResourceQuotas in the namespace.
2. For each ResourceQuota, compute `freeResources = Hard − Used`.
3. Iterate over `assumedWorkloads`; for entries whose `Namespace` matches the quota's namespace
   and that match this quota's scope (e.g. priorityClass from each component's
   `ReplicaRequirements.PriorityClassName`), subtract the sum of each component's
   `Replicas × ResourceRequest` from `freeResources`.
4. Execute `MaxDivided(perSetRequirements)` on the post-deduction `freeResources` to obtain the
   available component set count under this quota constraint.

#### 6.4 GeneralEstimator Fallback Implementation

The `ReplicaEstimator` interface has two implementations: `SchedulerEstimator`
(`pkg/estimator/client/accurate.go`, backed by the `karmada-scheduler-estimator` gRPC service)
and `GeneralEstimator` (`pkg/estimator/client/general.go`). `GeneralEstimator` queries the member
cluster kube-apiserver directly for aggregate node capacity and usage; it has **no per-node
resource view** and therefore cannot perform FF node-level deduction.

For `GeneralEstimator`, **cluster-level aggregate deduction** (the approach in Alternative 2) is
used as a fallback:

- **`MaxAvailableComponentSets`**: Accumulate each Component's
  `Replicas × ResourceRequest` across all `assumedWorkloads`; subtract from cluster-wide
  available resources, then estimate the number of schedulable complete component sets.

Cluster-level aggregate deduction has a tendency toward over-conservatism (see Alternative 2
analysis), but it significantly reduces the probability of over-commitment compared to performing
no deduction at all. When `karmada-scheduler-estimator` is deployed for a cluster, the scheduler
prefers `SchedulerEstimator` (node-level FF deduction); `GeneralEstimator` serves only as a
fallback path when the estimator service is unavailable.

### 7. Assumption Lifecycle

```
ResourceBinding enters the scheduling queue
│
▼
calculateMultiTemplateAvailableSets()
  Scheduler obtains AssumedWorkloads; passes them in the gRPC request to the Estimator
      │
      ▼ (Estimator side — nodeResource plugin)
      Obtain cloned nodes (Allocatable = Allocatable - Requested)
      FF algorithm consumes node resources for assumedWorkloads
      Compute available replicas on post-deduction node state
      │
      ▼ (Estimator side — resourcequota plugin)
      Compute total resource consumption of assumedWorkloads
      Subtract assumed consumption from ResourceQuota available capacity
      Compute available replicas on post-deduction quota state
      │
      Take the minimum of both plugin results as the final available replica count
│
▼ (Scheduler side)
patchScheduleResultForResourceBinding()
Patch succeeds, delta > 0 → Assume(bindingKey, clusterName, entry)
│
├──────────────────────────────────────────────┐
▼                                              ▼
ResourceBinding AggregatedStatus Healthy       TTL expires (5 minutes)
ReleaseClusterAssumption(bindingKey,          GC removes assumptions entry
clusterName) (released per cluster)
│
▼
Assumption removed; cluster capacity is fully reflected in the Estimator's next snapshot
```

### Notes and Constraints

**FF deduction is inherently approximate.**
The FF (First Fit) algorithm places replicas greedily in node order. It does not guarantee
results identical to the member cluster kube-scheduler's actual scheduling decisions (Karmada does
not control which specific node a replica lands on within a cluster). However, FF consumes
resources on exactly one node per replica, avoiding the broadcast deduction across all
constraint-matching nodes, and is significantly more accurate than cluster-level aggregate
deduction.

**Assumption release and TTL.**
Assumptions are released per cluster when the corresponding ResourceBinding's
`AggregatedStatus[].Health` for that cluster reaches `Healthy` — using the workload being fully
healthy (all Pods Running+Ready) as the release signal ensures no over-commitment is introduced.
The scheduler already watches ResourceBinding objects, so no additional informer is needed; the
release logic is added to the existing ResourceBinding update event handler. The TTL (default
5 minutes) serves as a fallback for scenarios where health status never transitions normally.

**CRD workloads require `InterpretHealth` hook for effective assumption.**
The assumption release mechanism relies on the `Healthy` signal from the resource interpreter. For
Kubernetes built-in resources (Deployment, StatefulSet, Job, DaemonSet, etc.), Karmada provides
default `InterpretHealth` implementations. However, for CRD workloads, if no `InterpretHealth` hook
is registered (neither via the default interpreter nor a customized/configurable interpreter), the
resource interpreter defaults to reporting the resource as healthy. This causes the ResourceBinding's
AggregatedStatus to reach `Healthy` shortly after distribution — before Pods are actually running
and bound to nodes — effectively rendering the assumption ineffective for that workload. CRD users
who rely on assumption-based scheduling protection should ensure their resource types have a proper
`InterpretHealth` implementation.

**Assumption state is lost on scheduler restart.**
After a restart the in-process cache is cleared; the `assumptions` in
`AssigningResourceBindingCache` are not rebuilt from the informer resync. In the window between a
restart and the point where member cluster Pods are Running and observed by the Estimator's node
snapshot, new concurrent scheduling decisions may see stale cluster capacity, resulting in a brief
over-commitment risk. As Pods come up one by one, the Estimator snapshot naturally reflects actual
resource consumption and the situation self-heals. This is a low-probability edge case and
represents an acceptable engineering trade-off.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| FF deduction is overly conservative, causing legitimate workloads to be rejected | FF consumes resources on exactly one node per replica; greedy placement is more accurate than cluster-level aggregate deduction; assumption count is bounded (TTL fallback) |
| Assumption released too late for slow-starting workloads, causing overly pessimistic estimates | Resources that take longer to become healthy (e.g., long image pulls, slow readiness probes) keep the assumption active even after their Pods have been assigned to nodes. During this window, the resource consumption is counted twice — once in the Estimator's node snapshot and once in the assumed workload deduction — making the scheduler underestimate the cluster's available capacity. **Mitigation**: This is a conservative trade-off that avoids over-commitment; the double-counting window is bounded by the TTL (default 5 minutes) and self-resolves once health is reached. The secondary verification extension (see Future Extensions) can further narrow this window. |
| Premature release for CRD workloads without `InterpretHealth` implementation | When a CRD resource type has no registered `InterpretHealth` hook, the resource interpreter defaults to reporting healthy status. This causes the ResourceBinding's AggregatedStatus to reach Healthy shortly after the resource is distributed — before Pods are actually running and bound to nodes — effectively rendering the assumption ineffective. **Mitigation**: For CRD workloads that require assumption protection, users should implement the `InterpretHealth` hook (via customized or configurable interpreter) to accurately reflect the actual health status. Document this requirement clearly so CRD users are aware of the limitation. |
| Scheduler restart causes loss of submitted-but-not-yet-landed assumptions | As Pods come up, the Estimator snapshot naturally reflects actual consumption and self-heals; acceptable engineering trade-off |
| gRPC request size increases due to assumption payload, causing performance degradation | Each `AssumedWorkload` entry encodes to roughly 56–270 bytes depending on the number of components and resource dimensions. `GetAssumedWorkloads(clusterName)` returns only assumptions for a specific cluster, and the live assumption count is bounded by the TTL window (default 5 minutes). Even under extreme scheduling throughput, per-request payload stays well below the default 4 MB gRPC message size limit; this risk is negligible in practice. |

### Test Plan

**E2E tests:**
- Create two Deployments simultaneously with an `Aggregated` policy on a cluster that can only
  accommodate one; assert that only one is successfully scheduled, the other is either scheduled
  to a different cluster or produces a clear resource-shortage event, and no Pod is silently
  Pending.

## Alternatives

### Alternative 1: Notify Estimator to Reserve Resources After Scheduling

**Description:** After a scheduling decision succeeds, the scheduler sends an additional gRPC call
to the Estimator to reserve the allocated resources.

**Analysis:**

`karmada-scheduler-estimator` is a **stateless computation service**: each `MaxAvailableReplicas`
gRPC call independently computes results based on the current member cluster informer snapshot,
with no shared state between calls.

The Estimator runs in HA mode (multiple replicas serving concurrently, no leader election).
Connections between the scheduler and the estimator go through a Kubernetes Service and are
load-balanced to any replica.

If a Reserve call lands on replica A but the subsequent `MaxAvailableReplicas` call lands on
replica B, replica B has no knowledge of the assumption — there is no shared state between
replicas. Making this work would require an external shared store (Redis, etcd) or inter-replica
gossip, turning the estimator from a **stateless service** into a **stateful service**, which
fundamentally contradicts its core design philosophy.

### Alternative 2: Cluster-Level Aggregate Deduction on the Scheduler Side Only (Insufficient Precision)

**Description:** The scheduler maintains an assumption cache and subtracts an equivalent number of
replicas at the cluster level after calling the Estimator, without modifying the Estimator
interface.

**Analysis:**

Cluster-level aggregate deduction has insufficient precision when the resource profiles of the
reserved and current workloads differ significantly. The scheduler does not know which specific
node an assumed workload landed on, and can only make a conservative estimate using ceiling
division:

```
ClusterA: Node1 (10 CPU available), Node2 (2 CPU available), 3 CPU per replica
Estimator result: floor(10/3) + floor(2/3) = 3 + 0 = 3

1 replica reserved (3 CPU, actually on Node1):
- True available: floor(7/3) + floor(2/3) = 2 + 0 = 2
- This option: equiv = ceil(3/3) = 1, adjusted = 3 - 1 = 2  ✓ (happens to be correct here)

1 CPU reserved (also on Node1):
- True available: floor(9/3) + floor(2/3) = 3 + 0 = 3
- This option: equiv = ceil(1/3) = 1, adjusted = 3 - 1 = 2  ✗ (over-conservative, off by 1)
```

The larger the difference between the resource profiles of the reserved and current workloads, the
greater the error.

This alternative can serve as a degraded fallback strategy when the Estimator is unavailable (e.g.
when the Estimator version does not support the new interface). In that scenario, the scheduler can
detect whether the Estimator supports the `AssumedWorkloads` field and fall back to cluster-level
deduction if not.


## Future Extensions

### Estimator-Side Secondary Verification

The current assumed workload deduction mechanism has a **double-counting window**: between the time a
Pod is bound to a node (reflected in the Estimator's `NodeInfo.Requested`) and the time the
ResourceBinding's AggregatedStatus reaches `Healthy` (assumption released), the Pod's resource
consumption is counted twice — once in the node snapshot and once in the FF simulation deduction.
This causes cluster available capacity to be underestimated, especially for slow-starting
workloads where the window can last several minutes.

A secondary verification step can be introduced on the Estimator side: before performing
assumed workload deduction, check how many Pods of the assumed workload have already been bound to
nodes, and reduce the effective deduction amount accordingly. This would require additional
information in the gRPC interface (e.g., the workload's previous Components before modification)
to distinguish old Pods from newly assumed ones, and the calculation should operate at the
resource-quantity level (cpu, memory) rather than replica counts to correctly handle
multi-component workloads.

