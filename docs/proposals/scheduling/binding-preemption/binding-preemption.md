---
title: Binding Preemption for Karmada Scheduler

authors:
- "@seanlaii"

reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@zhzhuang-zju"
- "@whitewindmills"

approvers:

creation-date: 2026-03-22
---

# Binding Preemption for Karmada Scheduler

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
- [Background](#background)
- [Design Overview](#design-overview)
- [How to Enable Preemption](#how-to-enable-preemption)
- [API Changes](#api-changes)
- [Phase 1: Summary-Based Preemption](#phase-1-summary-based-preemption)
- [Phase 2: Estimator-Based Preemption](#phase-2-estimator-based-preemption)
- [Why ClusterAffinities Is Excluded](#why-clusteraffinities-is-excluded)
- [Alternative Designs Considered](#alternative-designs-considered)
- [Risks and Limitations](#risks-and-limitations)
- [Backward Compatibility](#backward-compatibility)
- [Rollout Plan](#rollout-plan)
- [Test Plan](#test-plan)
- [Observability](#observability)

---

## Summary

This proposal introduces **binding-level preemption** to the Karmada scheduler, scoped to **single-cluster scheduling scenarios**. When a high-priority ResourceBinding cannot be scheduled on its target cluster due to insufficient resources, the scheduler may evict lower-priority bindings from that cluster to make room.

The proposal is phased:
- **Phase 1**: Summary-based preemption using aggregate replica arithmetic for victim selection.
- **Phase 2**: Estimator-based preemption with node-level simulation for precise victim selection.

Priority is sourced from the existing `SchedulePriority` mechanism in `PropagationPolicy`, resolved via Kubernetes `PriorityClass`, and propagated to `ResourceBindingSpec.SchedulePriority.Priority`.

## Motivation

Karmada v1.13 introduced priority-based scheduling (`PriorityBasedScheduling` feature gate), which orders the scheduling queue by priority. However, a high-priority binding that arrives after cluster resources are consumed by low-priority bindings will remain pending indefinitely. This is especially painful for batch/AI workloads where GPU resources are scarce. Binding preemption closes this gap.

### Example: GPU Training Preemption

Consider a Karmada environment with a single GPU cluster (`gpu-cluster`, 8 GPUs) where all workloads target this cluster via `MaxGroups == 1`:

**Initial state** — the cluster is full with low-priority batch jobs:

| Binding | Workload | Priority | GPUs | PriorityClass |
|---|---|---|---|---|
| batch-job-1 | Deployment (2 replicas, 2 GPU each) | 100 | 4 | `batch` (preemptionPolicy: Never) |
| batch-job-2 | Deployment (2 replicas, 2 GPU each) | 100 | 4 | `batch` (preemptionPolicy: Never) |

Total used: 8/8 GPUs. No capacity remaining.

**A high-priority training job arrives**:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: training-job
spec:
  replicas: 2
  template:
    spec:
      containers:
      - resources:
          requests:
            nvidia.com/gpu: 2
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: training-policy
spec:
  schedulePriority:
    priorityClassSource: KubePriorityClass
    priorityClassName: critical-training  # priority: 1000, preemptionPolicy: PreemptLowerPriority
  placement:
    clusterAffinity:
      clusterNames: [gpu-cluster]
    spreadConstraints:
    - spreadByField: Cluster
      maxGroups: 1
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
    name: training-job
```

**Key points in the PropagationPolicy**:
- `priorityClassName: critical-training` — references the PriorityClass that has `preemptionPolicy: PreemptLowerPriority`. This is the opt-in.
- `maxGroups: 1` — declares single-cluster intent. Without this, the binding could reroute to other clusters on retry, making preemption unnecessary. **Preemption requires explicit `MaxGroups == 1`.**

**Without preemption**: `training-job` stays pending indefinitely — the cluster is full and priority only affects queue ordering, not resource allocation.

**With preemption (this proposal)**:

1. Scheduler detects `assignReplicas` failure for `training-job` (priority 1000).
2. Scheduler finds lower-priority bindings on `gpu-cluster`: `batch-job-1` (100), `batch-job-2` (100).
3. `training-job` needs 4 GPUs. Either batch job alone frees 4 GPUs — sufficient. The reprieve loop keeps the older job (`batch-job-1`) and selects `batch-job-2` as the victim.
4. Scheduler evicts `batch-job-2` from `gpu-cluster` via GracefulEviction and records a claim.
5. After `batch-job-2`'s resources are freed, `training-job` schedules successfully on `gpu-cluster`.
6. `batch-job-2` reschedules to another cluster (if available) or stays pending.

### Goals

1. Define the preemption model for Karmada bindings in single-cluster scenarios.
2. Reuse the existing `SchedulePriority` / `PriorityClass` mechanism for preemption ordering.
3. Add a `PreemptionPolicy` field to `ResourceBindingSpec.SchedulePriority`.
4. Implement preemption inside `Schedule()` when replica assignment fails.
5. Integrate with GracefulEviction for victim eviction.
6. Gate behind `PriorityBasedPreemptiveScheduling` (alpha, default off).

### Non-Goals

1. **Multi-cluster preemption**: Coordinated victim eviction across multiple clusters. Requires iterative recalculation.
2. **Duplicated mode preemption**: `assignByDuplicatedStrategy` assigns full replicas to all candidates and never returns an error. The preemption trigger is unreachable.
3. **StaticWeight preemption**: Divides by weight ratio without capacity checks. Never returns an error.
4. **Component scheduling preemption**: Multi-component workloads (`len(Components) > 1`) bypass `assignFunc` in `AssignReplicas` entirely. Supporting them as preemptors requires a different trigger mechanism. See [Component Scheduling Path](#component-scheduling-path) for the future extension path.
5. Preemption for ClusterResourceBinding.
6. Gang preemption.
7. Cross-scheduler preemption coordination.
8. PodDisruptionBudget-aware preemption at the Karmada level.
9. **ClusterAffinities preemption**: See [Why ClusterAffinities Is Excluded](#why-clusteraffinities-is-excluded).

## Background

### Current Scheduling Pipeline

```
ResourceBinding enqueued → Pop (priority-ordered) → findClustersThatFit (Filter)
→ prioritizeClusters (Score) → selectClusters (SpreadConstraints + capacity)
→ assignReplicas (Duplicated/Aggregated/StaticWeight/DynamicWeight) → Write result
```

There is no preemption phase. If scheduling fails, the binding enters `backoffQ` or `unschedulableBindings`.

**Where capacity is checked**: Filter plugins do **not** check resource capacity. Capacity is first evaluated in `selectClusters` via `calAvailableReplicas` (which queries the general-estimator and/or scheduler-estimator for `AllocatableReplicas`), then checked again in `assignReplicas` via `dynamicDivideReplicas`.

### Assignment Strategies and Error Behavior

| Strategy | Checks capacity? | Returns `UnschedulableError`? |
|---|---|---|
| **Aggregated** | Yes (via `dynamicDivideReplicas`) | Yes |
| **DynamicWeight** | Yes (via `dynamicDivideReplicas`) | Yes |
| **StaticWeight** | No — divides by weight ratio | Never |
| **Duplicated** | No — assigns full replicas to all | Never |

`dynamicDivideReplicas` is the **only place** in the assignment flow that returns `UnschedulableError`. This is the preemption trigger point.

### Existing Priority Scheduling

`PropagationPolicy.spec.schedulePriority` specifies a `PriorityClassSource` and `PriorityClassName`. The ResourceDetector resolves this to a numeric priority written to `ResourceBindingSpec.SchedulePriority.Priority`. The priority queue orders bindings by priority descending, FIFO for ties.

### Prior Art

| System | Preemption unit | Trigger | Post-preemption coordination |
|---|---|---|---|
| **Kubernetes** | Pod on a node | PostFilter (all nodes rejected) | NominatedNodeName + in-memory nominator |
| **Kueue** | Workload (atomic) | Insufficient ClusterQueue quota | preemptionExpectations (UID tracking) |
| **Volcano** | Task within Job | Preempt/Reclaim actions | Transaction commit/rollback |

Related proposals: [Binding Priority and Preemption](../binding-priority-preemption/README.md) (direct predecessor), [Policy Preemption](../policy-preemption/README.md), [Multiple Pod Templates Scheduling](../multi-podtemplate-support/multiple-pod-template-support.md).

**Note**: Policy preemption (`PolicyPreemption` feature gate) resolves "which policy controls a resource" at the ResourceDetector level. Binding preemption (this proposal) resolves "which workload gets cluster resources when scarce" at the scheduler level. They are complementary and independent.

## Design Overview

```
High-priority binding cannot schedule (assignReplicas fails)
        │
        ▼
┌─── Preemption Trigger ───────────────────────────────────────────────┐
│  Is preemption applicable?                                           │
│  (workload? single-component? ClusterAffinity? MaxGroups==1?         │
│   PreemptLowerPriority? feature gates enabled?)                      │
└───────────────────────┬──────────────────────────────────────────────┘
                        │ yes
                        ▼
┌─── Victim Selection ─────────────────────────────────────────────────┐
│  1. Find lower-priority bindings on target cluster (via index)       │
│  2. Check: evicting all candidates frees enough resources?           │
│  3. Reprieve highest-priority candidates first (minimum victim set)  │
│                                                                      │
│  Phase 1: Aggregate replica arithmetic                               │
│  Phase 2: Estimator gRPC → node-level simulation (preferred)         │
│           Falls back to Phase 1 if estimator unavailable             │
└───────────────────────┬──────────────────────────────────────────────┘
                        │ victims found
                        ▼
┌─── Preemption Claims ────────────────────────────────────────────────┐
│  Record in-memory claim on target cluster (10-min TTL)               │
│  → Prevents re-preemption on retry                                   │
│  → Reserves capacity (lower-priority bindings see reduced capacity)  │
│  → Replaced (not accumulated) if target cluster changes              │
└───────────────────────┬──────────────────────────────────────────────┘
                        │
                        ▼
┌─── Preemption Execution ─────────────────────────────────────────────┐
│  For each victim: GracefulEvictCluster (30s grace period)            │
│  Preemptor enters unschedulableBindings (waits for resources)        │
│  On retry: victim resources freed → preemptor schedules successfully │
└──────────────────────────────────────────────────────────────────────┘
```

**Key concepts**:

- **Preemption trigger**: `assignReplicas` failure (only Aggregated/DynamicWeight can fail).
- **Victim selection**: "Remove all, then reprieve" greedy heuristic (same as Kubernetes `SelectVictimsOnNode`).
- **Preemption claims**: In-memory capacity reservation (equivalent to Kubernetes' in-memory pod nominator).
- **Preemption execution**: Existing GracefulEviction mechanism. Per-cluster, all-or-nothing (consistent with Kubernetes per-pod preemption).
- **Cross-kind, cross-namespace**: Preemption is purely priority-based with no kind or namespace restrictions, consistent with Kubernetes.

## How to Enable Preemption

1. **Enable feature gates** on the Karmada scheduler: `--feature-gates=PriorityBasedScheduling=true,PriorityBasedPreemptiveScheduling=true`
2. **Create PriorityClasses** with `preemptionPolicy: PreemptLowerPriority` (for preemptors) or `preemptionPolicy: Never` (for non-preemptors). Note: Kubernetes defaults `preemptionPolicy` to `PreemptLowerPriority` when unset, so omitting the field enables preemption.
3. **Reference the PriorityClass** in PropagationPolicy via `schedulePriority.priorityClassSource: KubePriorityClass`.
4. **Declare single-cluster intent** with `maxGroups: 1` in `placement.spreadConstraints`.

## API Changes

### Add `PreemptionPolicy` to `SchedulePriority`

```go
type PreemptionPolicy string

const (
    PreemptLowerPriority PreemptionPolicy = "PreemptLowerPriority"
)

type SchedulePriority struct {
    Priority         int32            `json:"priority,omitempty"`
    PreemptionPolicy PreemptionPolicy `json:"preemptionPolicy,omitempty"`
}
```

- Defaults to unset (empty string), meaning the binding never preempts. No explicit "Never" constant is needed — the zero value naturally means "no preemption."
- The ResourceDetector maps `PreemptionPolicy` from the PriorityClass's `preemptionPolicy` field when `PriorityBasedPreemptiveScheduling` is enabled. Kubernetes defaults `preemptionPolicy` to `PreemptLowerPriority` when unset, so any PriorityClass without explicit `preemptionPolicy: Never` enables preemption at the binding level. The feature gate (alpha, default off) provides the primary opt-in control.

### New Constants and Feature Gate

```go
const (
    EvictionProducerScheduler      = "Scheduler"
    EvictionReasonBindingPreempted = "BindingPreempted"
)

// Feature gate (alpha, default off). Requires PriorityBasedScheduling.
PriorityBasedPreemptiveScheduling featuregate.Feature = "PriorityBasedPreemptiveScheduling"
```

### Extended ScheduleResult

```go
type ScheduleResult struct {
    SuggestedClusters []workv1alpha2.TargetCluster
    // Non-nil when preemption was performed.
    // When set, SuggestedClusters is nil — the preemptor is NOT yet scheduled.
    PreemptionResult  *PreemptionResult
}

type PreemptionResult struct {
    Cluster string
    Victims []VictimBinding
}

type VictimBinding struct {
    Namespace string
    Name      string
    Replicas  int32
    Priority  int32
}
```

## Phase 1: Summary-Based Preemption

### Scope and Applicability

Preemption is attempted only when **all** of the following hold:

1. The binding is a workload (`IsWorkload() == true`).
2. The binding is single-component (`len(Components) <= 1`). Multi-component workloads bypass `assignFunc` in `AssignReplicas` entirely — they cannot reach the preemption trigger. This is a **Phase 1 limitation**, not a permanent design constraint. See [Component Scheduling Path](#component-scheduling-path) for the extension path.
3. The binding uses `ClusterAffinity`, not `ClusterAffinities`. See [Why ClusterAffinities Is Excluded](#why-clusteraffinities-is-excluded).
4. The binding explicitly targets one cluster via `SpreadConstraints` (`MaxGroups == 1`).
5. `assignReplicas` returns an error. The primary target is Aggregated mode with single-cluster scheduling. DynamicWeight also uses `dynamicDivideReplicas` internally, but with `MaxGroups == 1` it behaves identically to Aggregated.

```go
func isPreemptionApplicable(spec *workv1alpha2.ResourceBindingSpec) bool {
    if !spec.IsWorkload() { return false }
    if len(spec.Components) > 1 { return false }
    if spec.Placement == nil { return false }
    if spec.Placement.ClusterAffinities != nil { return false }
    for _, sc := range spec.Placement.SpreadConstraints {
        if sc.SpreadByField == policyv1alpha1.SpreadByFieldCluster &&
            sc.MaxGroups == 1 {
            return true
        }
    }
    return false
}
```

The full preemption gate (`preemptionEnabled`) combines feature gates + `PreemptionPolicy == PreemptLowerPriority` + `isPreemptionApplicable`.

### Modified Schedule() Flow

Preemption is placed inside `genericScheduler.Schedule()` because the trigger is `assignReplicas` failure, and selected clusters with per-cluster capacity are local to `Schedule()`. The `ScheduleAlgorithm` interface is unchanged.

```
function Schedule(spec):
    feasibleClusters = findClustersThatFit(spec)
    clustersScore    = prioritizeClusters(feasibleClusters)

    // Wrap calAvailableReplicas with claim deductions so that
    // AllocatableReplicas reflects reserved capacity from existing claims.
    calcFunc = withClaimDeductions(calAvailableReplicas, preemptionClaims)
    groupClustersInfo = GroupClustersWithScore(clustersScore, spec, calcFunc)

    // Step 1: Normal cluster selection (with capacity check)
    selectedClusters, err = SelectBestClusters(spec.Placement, groupClustersInfo, spec.Replicas)

    if err AND preemptionEnabled(spec):
        // All clusters have insufficient capacity. Retry without capacity
        // filtering so that assignReplicas is reached and can trigger preemption.
        // InvalidReplicas is a sentinel value that tells SelectBestClusters to
        // skip the capacity check while still enforcing MaxGroups and spread constraints.
        selectedClusters, err = SelectBestClusters(spec.Placement, groupClustersInfo, InvalidReplicas)

    if err: return error

    // Step 2: Assign replicas
    clustersWithReplicas, err = assignReplicas(selectedClusters, spec, status)

    if err is UnschedulableError AND preemptionEnabled(spec):
        // Only trigger preemption on capacity-related failures (UnschedulableError
        // from dynamicDivideReplicas). Other errors (internal, status reconciliation)
        // should not cause evictions.
        targetCluster = selectedClusters[0]
        if NOT preemptionClaims.hasClaimOnCluster(bindingKey, targetCluster):
            victims, ok = selectVictims(targetCluster, spec, ...)
            if ok:
                preemptionClaims.set(bindingKey, claim)
                return {PreemptionResult: {targetCluster, victims}}
            else:
                preemptionClaims.clearBinding(bindingKey)
        return error  // preemption not feasible or already claimed

    return {SuggestedClusters: clustersWithReplicas}
```

**After `Schedule()` returns** (in the scheduler main loop):

- If `PreemptionResult != nil`: execute preemption (evict victims), then return `PreemptingError` → preemptor enters `unschedulableBindings` queue.
- If scheduling succeeds: clear the claim.

### Victim Selection Algorithm

The algorithm uses the **"remove all, then reprieve"** pattern from Kubernetes:

```
function selectVictims(targetCluster, preemptorSpec, clusterAvailableReplicas):
    // Step 1: Find candidates (shared between Phase 1 and Phase 2)
    candidates = filterPreemptionCandidates(targetCluster, preemptorSpec)
        // Filters: lower priority, not in graceful eviction, not suspended,
        //          has replicas on cluster, has ReplicaRequirements (Phase 1 only)

    // Step 2: Try Phase 2 (estimator) first.
    if estimator available for cluster:
        victims, ok, fallback = selectVictimsWithEstimator(candidates, preemptorSpec)
        if NOT fallback:
            return victims, ok  // Estimator gave a definitive answer — trust it
        // Estimator unavailable (gRPC error) — fall through to Phase 1

    // Phase 1 fallback (aggregate replica arithmetic).
    return selectVictimsByReplicaCount(candidates, preemptorSpec, clusterAvailableReplicas)

function selectVictimsByReplicaCount(candidates, preemptorSpec, clusterAvailableReplicas):
    // "deficit" = how many preemptor replicas exceed current cluster capacity.
    // Example: preemptor needs 10 replicas, cluster has room for 6 → deficit = 4.
    deficit = preemptorSpec.Replicas - clusterAvailableReplicas
    if deficit <= 0: return nil, false  // Cluster has room; no preemption needed

    // Feasibility check: can freed resources cover the gap?
    // (clusterAvailableReplicas already accounts for existing spare capacity;
    //  deficit is the gap between what the cluster offers and what the preemptor needs.)
    totalFreedResources = sum(c.ReplicaRequirements * c.replicas for each candidate)
    if calculateFittingReplicas(totalFreedResources, preemptorSpec.ReplicaRequirements) < deficit:
        return nil, false  // Even evicting all candidates is insufficient

    // Reprieve highest-priority candidates first (greedy minimum victim set).
    // Start with all candidates removed, then try adding each one back.
    sort(candidates, by: priority DESC, then creationTime ASC)
    currentFreed = totalFreedResources
    reprieved = [false] * len(candidates)
    for i, candidate in candidates:
        tentativeFreed = currentFreed - candidate.resources
        if calculateFittingReplicas(tentativeFreed, preemptorSpec.ReplicaRequirements) >= deficit:
            reprieved[i] = true       // This candidate is not needed as victim
            currentFreed = tentativeFreed  // Update freed resources for next iteration

    victims = candidates where reprieved[i] == false
    return victims, true
```

### Preemption Execution

Victim eviction uses the existing `GracefulEvictCluster` mechanism. For each victim, the scheduler:
1. Creates a `GracefulEvictionTask` with `PurgeModeGracefully`, reason `BindingPreempted`, producer `Scheduler`, and 30-second grace period (shorter than the default 10-minute failover eviction because users who opt into `PreemptLowerPriority` accept that lower-priority workloads may experience disruption).
2. Calls `GracefulEvictCluster` to remove the victim's cluster assignment and add the eviction task.
3. Patches the victim binding via `GenMergePatch`.
4. Emits events on both preemptor (`PreemptionInitiated`) and victim (`Preempted`).

If all victim patches fail, the claim is cleared immediately.

**Per-cluster eviction granularity**: `GracefulEvictCluster` removes the victim's entire cluster assignment — all replicas on that cluster are evicted, even if the preemptor only needs a fraction. This is consistent with Kubernetes per-pod preemption (all-or-nothing). Partial-replica eviction is deferred.

**Multi-cluster bindings as victims**: A victim assigned to multiple clusters is only evicted from the target cluster. Replicas on other clusters are unaffected.

### Post-Preemption Behavior

**Preemptor**: After preemption succeeds, the preemptor enters the `unschedulableBindings` queue with its claim active (TTL 10 min). It waits for victim resources to be freed:
1. Victim eviction begins (30-second grace period via GracefulEviction).
2. When the victim's cluster assignment is removed and `ResourceSummary` updates, the preemptor is moved to `activeQ` (via companion [PR #7369](https://github.com/karmada-io/karmada/pull/7369)). Without that PR, the preemptor waits for the 5-minute `unschedulableBindings` flush interval.
3. On retry: `assignReplicas` succeeds with the freed capacity → preemptor schedules → claim cleared.
4. If the claim expires (10 min) without success, the preemptor retries normally (may re-preempt if victims have returned).

**Victim**: After eviction, the victim's cluster assignment is removed and a `GracefulEvictionTask` is created. The binding controller re-queues the victim for scheduling:
1. The scheduler processes the victim's new scheduling attempt.
2. `withClaimDeductions` reduces the preemptor's claimed cluster capacity, directing the victim to non-claimed clusters.
3. If another cluster has room, the victim schedules there. If not, it stays pending until the claim expires or capacity becomes available.

### Preemption Claims

After preemption is initiated, three coordination problems arise. The **preemption claims** mechanism — an in-memory claim store on the scheduler — addresses all three. It is the cluster-level equivalent of Kubernetes' in-memory pod nominator.

#### Claim Data Structure

```go
type PreemptionClaimStore struct {
    mu        sync.RWMutex
    byBinding map[string]*preemptionClaim     // one claim per preemptor
    byCluster map[string][]*preemptionClaim   // all claims on a cluster
}

type preemptionClaim struct {
    bindingKey   string              // "Kind/Namespace/Name"
    cluster      string
    priority     int32
    resourceNeed corev1.ResourceList
    expiry       time.Time           // 10-minute TTL
}
```

The store is dual-indexed: `byBinding` ensures one claim per preemptor (replacement, not accumulation), while `byCluster` enables efficient per-cluster resource accounting.

#### Problems and Solutions

**Problem 1 — Preventing repeated preemption**: After preemption, the preemptor waits in `unschedulableBindings`. On retry, if resources aren't freed yet, `assignReplicas` fails again. The claim blocks re-preemption on the same cluster while active.

**Problem 2 — Reserving cluster capacity**: The evicted victim is re-queued and could schedule back to the same cluster before the preemptor retries. Claims are injected into `calAvailableReplicas` via `withClaimDeductions`:

```
function withClaimDeductions(baseFn, claims):
    return function(clusters, spec):
        result = baseFn(clusters, spec)
        for each cluster in result:
            for each claim on this cluster:
                // Deduct for equal-or-higher priority claims, except the claim owner.
                // This matches K8s RunFilterPluginsWithNominatedPods behavior.
                if claim.priority >= currentPriority AND claim.bindingKey != self:
                    cluster.Replicas -= claimedReplicas(claim.resourceNeed, spec)
        return result
```

Because `calAvailableReplicas` feeds into `AllocatableReplicas` (via `GroupClustersWithScore`), claim deductions flow through the **entire scheduling pipeline** — cluster scoring, cluster selection, and replica assignment — with a single injection point.

The `>=` (not `>`) comparison is intentional: equal-priority bindings must also see reduced capacity on the claimed cluster. If strict `>` were used, an equal-priority binding could consume the reserved resources before the preemptor retries, creating a livelock where both keep preempting each other. With `>=`, the FIFO ordering in the priority queue resolves ties — the first preemptor gets the reservation, and the second is directed elsewhere.

**Problem 3 — Handling claim migration**: If the claimed cluster becomes unfit (tainted, deleted), the claim is **replaced** (not accumulated) when the preemptor preempts on a different cluster.

#### Claim Lifecycle

1. **Set** when preemption succeeds. Replaces any existing claim for the same preemptor.
2. **Cleared on success** when the preemptor schedules.
3. **Cleared on deletion** when the preemptor's ResourceBinding is deleted (event handler).
4. **Auto-expired** after 10 minutes (lazy filtering). TTL exceeds the 5-min unschedulableBindings flush + 30s grace period + safety margin.

**Why in-memory, not API-backed**: Kubernetes' correctness depends on the in-memory nominator, not `pod.Status.NominatedNodeName`. We follow the same pattern. On scheduler restart, claims are lost; the preemptor may re-preempt once. The system is self-healing.

## Phase 2: Estimator-Based Preemption

Phase 2 replaces summary-based victim selection with node-level simulation via the scheduler estimator. The preemption trigger, scope, claims, and execution mechanisms remain unchanged from Phase 1.

### Why Estimator-Based Preemption Is Needed

Phase 1 preemption uses aggregate capacity data (derived from `ResourceSummary` via `calAvailableReplicas`) and cannot detect node-level constraints:

- Preemptor needs 8 CPU with node affinity to `gpu-pool` nodes.
- Victim A: 4 CPU on Node1 (gpu-pool). Victim B: 4 CPU on Node2 (cpu-pool).
- Summary says "8 CPU freed" → preemption appears feasible.
- Reality: only 4 CPU freed on gpu-pool nodes → preemptor still cannot schedule.

The estimator has a per-node snapshot and can simulate pod removal to accurately determine feasibility.

### What Changes from Phase 1

| Component | Phase 1 | Phase 2 | Change |
|---|---|---|---|
| Preemption trigger | `assignReplicas` failure | Same | None |
| Applicability checks | Same | Same | None |
| Candidate filtering | Shared | Shared | None |
| Candidate ordering | Shared | Shared | None |
| **Feasibility + reprieve** | Aggregate replica arithmetic | **Estimator RPC** | **Replace** |
| **Victim filter: `ReplicaRequirements`** | Required (for replica math) | **Relaxed** (estimator knows pod resources) | **Relax** |
| Claim mechanism | Same | Same | None |
| Claim deductions | Same | Same | None |
| Victim eviction | Same | Same | None |

Phase 2 replaces **only** the feasibility and reprieve logic inside `selectVictims`. The shared components — candidate filtering, ordering, claims, eviction — are reused unchanged.

**Dual-mode fallback**: When the estimator is unavailable (not deployed, gRPC error, timeout), `selectVictims` falls back to Phase 1 aggregate arithmetic. When the estimator returns `feasible: false`, the scheduler trusts this definitive answer and does **not** fall back to Phase 1 — the estimator has strictly more information (node-level) than Phase 1 (aggregate), so a Phase 1 override would risk unnecessary evictions. During the transition period after enabling the feature, some candidates may lack `ResourceBindingPermanentIDLabel` on their pods. If any candidates are missing the label, `selectVictimsWithEstimator` falls back to Phase 1 rather than sending a partial candidate set to the estimator.

### Flow

```
Scheduler detects assignReplicas failure
  → Scheduler pre-filters candidates (shared)
  → Scheduler orders candidates by reprieve priority (shared)
  → Scheduler sends ordered candidate list to estimator via SelectVictims RPC
  → Estimator runs node-level "remove all, then reprieve" on cloned snapshot
  → Estimator returns minimum victim set
  → Scheduler evicts victims via GracefulEviction (shared)
```

The scheduler handles **policy** (who is eligible, what reprieve order). The estimator handles **feasibility** (does the preemptor fit after removing victims, which victims can be reprieved).

### Estimator API

```protobuf
rpc SelectVictims(SelectVictimsRequest) returns (SelectVictimsResponse) {}

message SelectVictimsRequest {
    string cluster = 1;
    ReplicaRequirements preemptorReplicaRequirements = 2;
    int32 preemptorReplicas = 3;
    repeated VictimCandidate candidates = 4;  // ordered by reprieve priority
}

message VictimCandidate {
    string bindingPermanentID = 1;  // ResourceBindingPermanentIDLabel value
}

message SelectVictimsResponse {
    bool feasible = 1;
    repeated VictimCandidate selectedVictims = 2;  // candidates that must be evicted
}
```

The scheduler sends only **pre-filtered, ordered** candidates (typically 10-50 entries). Each candidate is identified by its `ResourceBindingPermanentIDLabel` value. The estimator does not need per-candidate priority because the reprieve order is determined by the scheduler.

### Estimator-Side Algorithm

```
function SelectVictims(request):
    snapshot = Cache.UpdateSnapshot()
    clonedNodes = snapshot.CloneNodes()

    // Find pods for each candidate via binding permanent ID index
    for each candidate in request.candidates:
        candidatePods[i] = podIndexer.ByIndex("binding-permanent-id", candidate.id)

    // Step 1: Remove ALL candidate pods from cloned snapshot
    for each candidate's pods:
        clonedNodes[pod.nodeName].RemovePod(pod)

    // Feasibility check: does preemptor fit with all candidates removed?
    modifiedSnapshot = NewFromNodeInfoMap(clonedNodes)
    maxReplicas = estimateReplicas(modifiedSnapshot, request.preemptor_requirements)
    if maxReplicas < request.preemptor_replicas:
        return {feasible: false}

    // Step 2: Reprieve in given order (candidates arrive sorted by scheduler)
    for each candidate in request.candidates:
        for each pod in candidatePods[i]:
            clonedNodes[pod.nodeName].AddPod(pod)  // try adding back

        checkSnapshot = NewFromNodeInfoMap(clonedNodes)
        maxReplicas = estimateReplicas(checkSnapshot, request.preemptor_requirements)
        if maxReplicas < request.preemptor_replicas:
            // Cannot reprieve — undo and mark as victim
            for each pod: clonedNodes[pod.nodeName].RemovePod(pod)
        // else: reprieved (pods stay added back)

    return {feasible: true, selected_victims: un-reprieved candidates}
```

The `estimateReplicas` function reuses the existing estimation framework (`RunEstimateReplicasPlugins`) including the `noderesource` plugin for per-node capacity with affinity/toleration filtering. Snapshot manipulation uses `CloneNodes()` and `NewFromNodeInfoMap()` helpers added to `pkg/util/lifted/scheduler/cache/snapshot.go`.

### What Changes in the Estimator

1. **Mutable snapshot clone**: `CloneNodes()` deep-clones all `NodeInfo` objects. `NewFromNodeInfoMap()` reconstructs a `Snapshot` from the modified map.

2. **Pod-to-binding index**: An index mapping `ResourceBindingPermanentIDLabel` values to pods, maintained via pod informer events. Enables O(1) lookup of all pods belonging to a candidate victim.

3. **`SelectVictims` RPC**: Clones the snapshot, removes candidate pods, checks preemptor feasibility, and runs the reprieve loop.

### Pod-to-Binding Mapping (Prerequisite)

The binding controller injects `ResourceBindingPermanentIDLabel` into the workload's top-level metadata. For Phase 2, this label must also appear on the **pods** so the estimator can map pods to bindings.

A new Resource Interpreter operation, `RevisePodTemplate`, injects the binding permanent-id label into `.spec.template.metadata.labels` before the workload is applied to member clusters. The binding controller calls `RevisePodTemplate` in `ensureWork()` when `PriorityBasedPreemptiveScheduling` is enabled.

**Supported workload types** (default native interpreters via `.spec.template.metadata.labels`):
- Deployment, StatefulSet, DaemonSet, ReplicaSet, Job

For custom resources, users implement the operation via the webhook interpreter (`InterpreterOperationRevisePodTemplate`).

### Component Scheduling Path

Multi-component preemptors (`len(Components) > 1`) bypass `assignFunc` in `AssignReplicas` entirely — they are propagated to all candidate clusters without replica division. Since preemption is triggered by `assignReplicas` failure, multi-component workloads cannot reach the trigger. This is a **Phase 1 limitation**, not a permanent design constraint.

The preemption infrastructure (claims, victim selection, eviction) is designed to be reusable. The Phase 2 estimator path operates at the pod level, which naturally handles multi-component workloads. Extending preemption to multi-component requires:

1. **A new trigger**: Extend `AssignReplicas` or add a pre-check to call `MaxAvailableComponentSets` (which already exists in `pkg/scheduler/core/estimation.go`) for multi-component workloads, returning an error when the cluster lacks capacity.
2. **Relax the component check**: Remove `len(Components) > 1` guard in `isPreemptionApplicable`.
3. **Extend claims**: Track per-component resources in `preemptionClaim.resourceNeed` for accurate claim deductions.

Multi-component workloads can already be **victims** in Phase 2 — the estimator knows their actual pod-level resource consumption. In Phase 1, they are filtered out because their `Spec.Clusters` entries lack `Replicas`.

## Why ClusterAffinities Is Excluded

`ClusterAffinities` allows multiple affinity terms in preference order. Preemption interacts poorly with this loop:

1. **Semantic ambiguity**: If term 1 (preferred) needs preemption but term 2 (less preferred) has resources, should the scheduler preempt on term 1 or schedule on term 2? Neither is obviously correct.
2. **Claim consistency**: Each term calls `Schedule()`, which overwrites the previous claim. The claim state may point to the wrong cluster.
3. **Nil-clusters bug**: `Schedule()` returns `(result, nil)` on preemption with `SuggestedClusters == nil`. Without a `PreemptionResult` check, the ClusterAffinities success path would patch the binding with nil clusters.

These issues require deliberate design (e.g., two-phase preemption, multi-claim support, or "try all terms before preempting"). The `isPreemptionApplicable` check returns `false` when `ClusterAffinities != nil`, cleanly excluding this case. This is a Phase 1 limitation deferred to a future proposal.

## Alternative Designs Considered

| Alternative | Verdict |
|---|---|
| Multi-cluster preemption from start | Deferred. Cross-cluster coordination requires iterative recalculation. |
| Estimator-only preemption (skip summary-based) | Deferred. Requires estimator on every cluster. Summary-based is sufficient for Phase 1. |
| Preempt entire bindings across all clusters | Rejected. Massive blast radius. Per-cluster preemption minimizes disruption. |
| NominatedCluster (persistent on binding) | Rejected. Informer cache lag creates race conditions. In-memory claims provide equivalent functionality without API changes. |
| Cooldown-only (no resource accounting) | Rejected. Does not prevent victims from consuming resources reserved for the preemptor. |

## Risks and Limitations

### Summary-based precision (Phase 1)

Summary-based preemption cannot detect node fragmentation, affinity constraints, or resource quotas. Phase 2 addresses this via estimator node-level simulation.

### Preemption delay

After preemption, the preemptor enters `unschedulableBindings` and waits for the 5-minute flush interval. Worst-case delay: **~5 minutes**. The claim (TTL 10 min) protects freed resources during this window. A companion PR ([#7369](https://github.com/karmada-io/karmada/pull/7369)) adds `ResourceSummary` change detection to move the preemptor to `activeQ` immediately when victim resources are freed, reducing delay to near-real-time.

### Cascading preemption

Preemption chains (P1→V1→V2→V3) are possible, bounded by the number of distinct priority levels. Each step is a locally correct single-cluster preemption. This is inherent to priority-based preemption and consistent with Kubernetes.

### Optimistic locking

Phase 1 uses `GenMergePatch` for victim patches without conflict retry. Concurrent modifications by other controllers could be overwritten. A retry loop on `409 Conflict` is tracked as a follow-up improvement before beta promotion.

### Cross-namespace preemption

Preemption is cross-namespace, consistent with Kubernetes (PriorityClass is cluster-scoped). Multi-tenancy protection should be enforced via admission webhooks restricting PriorityClass access per namespace.

### Informer cache lag

Between the time the scheduler patches a victim with `GracefulEvictCluster` and the time the informer cache reflects the change, `filterPreemptionCandidates` may still see the victim as assigned to the target cluster. The `ClusterInGracefulEvictionTasks` check mitigates this once the patch lands in the cache. In the worst case, the victim appears as a candidate for another preemption attempt, but the redundant eviction patch is harmless (idempotent). The system is self-correcting.

### Preemptor deleted mid-preemption

If the preemptor is deleted after victims are evicted but before it schedules, the claim is cleared (via the delete event handler), and freed resources become available to other bindings. The evicted victims were already re-queued and will reschedule normally. This is a transient resource waste bounded by the eviction grace period, and is inherent to any preemption system.

### Namespace quota interaction

Preemption is triggered by cluster-level resource exhaustion, not namespace quota. If preemption frees cluster resources but the preemptor's binding patch is rejected by the quota webhook, evicted victims are rescheduled and the claim expires. The system is self-healing.

### Scheduler high availability

The preemption claim store is in-memory and not replicated. This is safe because the Karmada scheduler uses leader election (`--leader-elect=true` by default) and runs single-active, consistent with the Kubernetes scheduler. Only the leader performs scheduling and holds claims. On leader failover, in-memory claims are lost; the new leader may re-preempt once for any in-flight preemptors, which is self-healing — identical to how Kubernetes handles its in-memory pod nominator across kube-scheduler restarts.

### Performance impact

When the feature gate is enabled but no preemption is occurring (common case), the overhead is minimal. `withClaimDeductions` wraps `calAvailableReplicas` and iterates over claims on each cluster; when the claim store is empty, this loop executes zero iterations per cluster. The cluster-to-bindings index is populated by the existing informer and adds no extra API calls. During preemption, `filterPreemptionCandidates` performs one index lookup (O(bindings on cluster)) plus a sort (O(n log n)) and a single reprieve pass (O(n)) for Phase 1. Phase 2 adds one gRPC round-trip to the estimator (10-second timeout). These costs are incurred only on the preemption path, which is triggered by `assignReplicas` failure — a path that already terminates the scheduling cycle with an error.

## Backward Compatibility

1. **Feature-gated**: `PriorityBasedPreemptiveScheduling` defaults to `false`. No behavior change when disabled.
2. **API additive**: `PreemptionPolicy` is a new optional field, defaults to unset (no preemption).
3. **Hard dependency**: Requires `PriorityBasedScheduling`. Preemption is a no-op if that gate is disabled.
4. **Cluster-to-bindings index**: Only initialized when feature gate is enabled.
5. **Composable**: Works alongside PolicyPreemption, GracefulEviction, Failover, SpreadConstraints.

## Rollout Plan

| Phase | Release | Scope | Feature Gate |
|---|---|---|---|
| 0 | v1.13 (done) | Priority-based scheduling (queue ordering) | `PriorityBasedScheduling` (alpha) |
| 1 | v1.19 | Summary-based single-cluster preemption | `PriorityBasedPreemptiveScheduling` (alpha) |
| 2 | v1.20 | Estimator-based preemption (node-level victim selection) | `PriorityBasedPreemptiveScheduling` (alpha) |
| 3 | v1.21+ | Harden, promote to beta | `PriorityBasedPreemptiveScheduling` (beta) |

## Test Plan

### Unit Tests
- Victim selection: minimum victims, priority ordering, reprieve correctness.
- `PreemptionPolicy` resolution: feature gate disabled → unset, explicit opt-in.
- Applicability checks: `MaxGroups == 1`, non-workload, component scheduling, ClusterAffinities exclusions.
- Cluster selection retry: when all clusters lack capacity and preemption is enabled, retry without capacity filtering to reach `assignReplicas`.
- Claim store: set, hasClaimOnCluster, ClearBinding, TTL expiry, claim replacement.
- `withClaimDeductions`: claim-adjusted AllocatableReplicas, self-exception, priority filtering.

### Integration Tests
- End-to-end: high-priority binding preempts low-priority binding on single cluster.
- No preemption when PreemptionPolicy is unset or feature gate disabled.
- Preempted binding reschedules to another cluster.
- Claim prevents victim from returning to claimed cluster.

### E2E Tests
- Fill a cluster with low-priority bindings, submit high-priority binding, verify preemption.
- Verify events on both preemptor and victim.
- Verify no preemption for Duplicated, StaticWeight, component scheduling, ClusterAffinities, multi-cluster.

## Observability

### Events

| Target | Type | Reason | Message |
|---|---|---|---|
| Preemptor | Normal | `PreemptionInitiated` | "Initiated preemption of N binding(s) on cluster C" |
| Victim | Warning | `Preempted` | "Preempted by binding ns/name (priority=N) from cluster C" |
| Preemptor | Normal | `ScheduleBindingSucceed` | Standard scheduling success |

### Metrics

| Metric | Type | Labels |
|---|---|---|
| `karmada_scheduler_preemption_attempts_total` | Counter | `result={success,failure}` |
| `karmada_scheduler_preemption_victims_total` | Counter | (none) |

### Status Conditions

1. **Preemption initiated**: `Scheduled=False, Reason=Preempting`.
2. **Scheduled after preemption**: `Scheduled=True, Reason=Success`.
3. **Failed**: `Scheduled=False, Reason=Unschedulable`.
