---
title: Scheduler Estimator Support for Dynamic Resource Allocation (DRA)
authors:
  - "@seanlaii"
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-04-13
---

# Scheduler Estimator Support for Dynamic Resource Allocation (DRA)

## Summary

Kubernetes Dynamic Resource Allocation (DRA) provides a framework for managing non-traditional compute resources such as GPUs, FPGAs, and smart NICs. The `resource.k8s.io/v1` API group reached GA in Kubernetes 1.34. The `karmada-scheduler-estimator` currently evaluates only traditional resources (CPU, memory, extended resources) and has no awareness of DRA-managed devices. This means Karmada may schedule workloads to clusters that cannot satisfy their DRA requirements, or fail to accurately estimate how many replicas a cluster can accommodate when the bottleneck is a DRA resource.

This proposal enhances the `karmada-scheduler-estimator` with a new `DynamicResourceEstimator` plugin that evaluates DRA device availability on member clusters. The control plane extracts lightweight `ResourceClaimTemplate` name references from the Pod spec and passes them to the estimator. The estimator then resolves the templates locally from the member cluster, evaluates device availability against `ResourceSlice`, `ResourceClaim`, and `DeviceClass` objects, and computes `MaxAvailableReplicas`. This follows the same architectural pattern as the `ResourceQuotaEstimator`, where the estimator reads cluster-local resources rather than receiving pre-resolved data from the control plane.

The design uses **only GA fields** (no `+featureGate`-guarded fields) and supports all three GA node-selection modes on `ResourceSliceSpec`: `NodeName`, `NodeSelector`, and `AllNodes`. The API types are designed for additive extension as feature-gated DRA fields graduate to GA in future Kubernetes releases.

## Motivation

Modern workloads increasingly depend on specialized hardware managed through DRA. In a multi-cluster Karmada environment, the scheduler needs DRA awareness to make correct placement decisions. Without it:

- The scheduler may assign replicas to a cluster with no matching DRA devices, causing pods to remain pending indefinitely.
- The scheduler cannot accurately estimate cluster capacity when DRA devices are the bottleneck.
- Multi-cluster GPU scheduling becomes unreliable for AI/ML workloads.

### Goals

- Enable the estimator to identify DRA device requirements from a Pod's `resourceClaims` field.
- Query member cluster DRA state (`ResourceClaimTemplate`, `ResourceSlice`, `ResourceClaim`, `DeviceClass`) to determine device availability.
- Support all three GA node-selection modes for `ResourceSlice`: `NodeName`, `NodeSelector`, and `AllNodes`.
- Accurately calculate the maximum number of replicas a member cluster can support, factoring in DRA device availability.
- Integrate with the existing estimator plugin framework via the `min()` aggregation.
- Gate the feature behind a `DRAEstimator` feature flag (alpha, default off).
- Design API types to be extensible for future DRA features without breaking changes.

### Non-Goals

- **DRA object propagation.** Propagation of DRA objects (ResourceClaim, DeviceClass, ResourceClaimTemplate) from the Karmada control plane to member clusters. This requires a separate propagation mechanism design. Users are expected to propagate `ResourceClaimTemplate` objects to member clusters (via `PropagationPolicy` or direct creation) before deploying workloads that reference them.
- **Feature-gated DRA fields.** Every DRA field behind a `+featureGate` annotation is excluded (see [Kubernetes Version and DRA Field Scope](#kubernetes-version-and-dra-field-scope) for the full list). When these gates graduate to GA, support can be added as additive changes.
- **Cross-request `DeviceConstraint` evaluation** (e.g., "two GPUs must be on the same NUMA node"). `DeviceClaim.Constraints` is a GA field, but evaluating it correctly requires replicating K8s's exhaustive backtracking allocator (~500 lines of combinatorial logic). Without constraints, the estimate is a valid upper bound. See [Accuracy Limitations](#accuracy-limitations) for impact analysis and user guidance.
- **`DeviceClaim.Config`** (driver-specific opaque configuration). Not scheduling-relevant; ignored during estimation.
- **DRA support in the `EstimateComponentsPlugin` path** (multi-component workloads). `ResourceClaimReference` is added only to `ReplicaRequirements`, not `ComponentReplicaRequirements`. This can be extended in a future phase.
- **Custom Resource Interpreter for DRA objects** (status collection/aggregation).

## Proposal

This proposal introduces a new estimator plugin, `DynamicResourceEstimator`, that evaluates DRA device availability on member clusters. The control plane extracts `ResourceClaimTemplate` name references from the Pod spec and includes them in the `ReplicaRequirements` sent to estimators. The estimator plugin resolves these templates locally from the member cluster's API, then reads `ResourceSlice`, `ResourceClaim`, and `DeviceClass` objects via informers to compute how many replicas can be satisfied by available (unallocated) devices.

This follows the same pattern as the `ResourceQuotaEstimator` plugin, which receives lightweight workload metadata (resource requests, namespace, priority class) from the control plane and reads `ResourceQuota` objects locally from the member cluster. The DRA estimator similarly receives lightweight references and reads DRA objects locally.

### User Stories

#### Story 1: GPU Workload Scheduling

A user has two member clusters: `cluster-gpu` with 8 NVIDIA A100 GPUs, and `cluster-cpu` with no GPUs. They deploy a workload requiring 1 GPU per replica with 4 replicas. The `ResourceClaimTemplate` and `DeviceClass` are pre-propagated to both clusters.

**Without DRA support:** The estimator ignores DRA requirements. Both clusters may report high available replicas based on CPU/memory alone. The scheduler might assign replicas to `cluster-cpu`, where pods will remain pending forever.

**With DRA support:** The `DynamicResourceEstimator` on `cluster-cpu` returns 0 (no matching ResourceSlice). The plugin on `cluster-gpu` returns 8. Combined with CPU/memory estimation via `min()`, the scheduler correctly assigns all 4 replicas to `cluster-gpu`.

#### Story 2: Multi-Cluster GPU Distribution

A user has `cluster-a` with 4 available A100 GPUs and `cluster-b` with 6. They deploy a workload requiring 2 GPUs per replica with 5 replicas.

**With DRA support:** The estimator reports `cluster-a` = 2 replicas (4/2) and `cluster-b` = 3 replicas (6/2). The scheduler distributes proportionally.

#### Story 3: CEL Selector Filtering

A user needs GPUs with at least 40GB memory. The member cluster has 4x A100 80GB and 4x T4 16GB. The workload specifies a CEL selector filtering by memory capacity.

**With DRA support:** The estimator evaluates the CEL selector against each device's attributes. Only the 4 A100 GPUs match. The estimator correctly reports 4 (not 8).

#### Story 4: Template-Gated Scheduling

A user has three member clusters but only propagates the `ResourceClaimTemplate` to `cluster-a` and `cluster-b`. When the workload is scheduled, `cluster-c` returns 0 (template not found), naturally preventing scheduling there.

### Notes/Constraints/Caveats

- **Estimator does not perform pod placement.** The estimator calculates cluster-level capacity. Actual pod-to-node placement and device binding is handled by the member cluster's Kubernetes scheduler.
- **Estimation is a point-in-time approximation.** DRA devices can be allocated or released between estimation and actual scheduling. This is the same race condition that exists for CPU/memory estimation.
- **CEL evaluation cost.** Each device must be evaluated against CEL selectors. The plugin uses a CEL compilation cache to mitigate repeated compilation.
- **The estimator only reads DRA objects; it never creates or modifies them.** E2E tests can create ResourceSlice and DeviceClass objects directly without real hardware or DRA drivers.
- **ResourceClaimTemplate must exist on the member cluster.** The estimator resolves templates from the member cluster's API. If the template has not been propagated (via `PropagationPolicy` or direct creation), the estimator returns 0 for that cluster — the workload will not be scheduled there. This is the correct behavior: if a cluster doesn't have the template, it cannot run the workload.
- **ResourceClaimName claims (pre-existing, shared) are supported.** When a Pod references a `ResourceClaimName`, the estimator verifies the claim exists on the member cluster (if missing, returns 0). Shared claims do not constrain `MaxAvailableReplicas` because all replicas share one claim. The estimator treats an existing but unallocated shared claim as viable — the member cluster's scheduler handles actual allocation, mirroring how Karmada treats other pending dependencies (e.g., unbound PVCs).
- **A pod can have multiple `resourceClaims` entries.** Each entry has a unique name and references either a `ResourceClaimTemplateName` or `ResourceClaimName`. The control plane extracts all references. The estimator evaluates all per-replica templates (taking the minimum across all device requests) and verifies all shared claims exist.
- **`ResourceClaimTemplateName` and `ResourceClaimName` are mutually exclusive** per `PodResourceClaim` semantics in K8s. If both are set (which should not pass API validation), the template name takes precedence.
- **`ExactDeviceRequest.AdminAccess` is GA but not scheduling-relevant.** This field (GA since K8s 1.32) controls the access mode for allocated devices. It does not affect device capacity or availability and is therefore not evaluated during estimation.
- **New vendor dependency.** CEL evaluation uses `k8s.io/dynamic-resource-allocation/cel`, which is not currently vendored in Karmada. This package and its transitive dependencies must be added. The underlying `cel-go` library is already available as an indirect dependency.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| CEL evaluation overhead for large device pools | CEL compilation cache. Nodes processed sequentially (greedy shared consumption requires ordering). |
| DRA API changes in future K8s versions | Estimator resolves templates locally, insulated by internal types. Only the resolution layer needs updates. |
| Inaccurate estimation when DeviceConstraints are present | Constraints excluded. May overcount in constrained scenarios. Documented limitation with user guidance (see [Accuracy Limitations](#accuracy-limitations)). |
| Estimator memory usage increase from DRA informers | Feature-gated (default off). Informers only started when `DRAEstimator` is enabled. |
| `ResourceClaimTemplate` missing on member cluster | Estimator returns 0 capacity. Workload is not scheduled to that cluster (correct behavior). |
| Shared claim exists but referencing unavailable devices | Optimistic assumption — cluster considered viable. Member cluster scheduler handles actual allocation. Low risk (unlikely in practice). |

## Design Details

### Kubernetes Version and DRA Field Scope

| Requirement | Version |
|-------------|---------|
| Karmada vendored K8s API | v0.35.x (Kubernetes 1.35) |
| DRA `resource.k8s.io/v1` GA since | Kubernetes 1.34 |
| Minimum member cluster version | Kubernetes 1.34 (for DRA v1 API types) |
| Minimum Karmada API server version | Kubernetes 1.34 (for `PodResourceClaim.ResourceClaimTemplateName` field) |

This proposal uses **only GA fields** — fields present in the `resource.k8s.io/v1` API without any `+featureGate` guard. Feature-gated fields are explicitly excluded and can be adopted as additive changes when they graduate to GA.

The Kubernetes DRA v1 API defines four node-selection modes on `ResourceSliceSpec`, of which **three are GA** and one is feature-gated:

| Mode | Field | GA? | This Proposal |
|------|-------|-----|---------------|
| Single node | `ResourceSliceSpec.NodeName` | Yes | Supported — devices mapped to one node (exclusive) |
| Node selector | `ResourceSliceSpec.NodeSelector` | Yes | Supported — devices mapped to matching nodes |
| All nodes | `ResourceSliceSpec.AllNodes` | Yes | Supported — devices in shared pool |
| Per-device | `ResourceSliceSpec.PerDeviceNodeSelection` | No (`+featureGate=DRAPartitionableDevices`) | Skipped with warning |

The Kubernetes `DeviceRequest` type has two mutually exclusive request modes:

| Mode | Field | GA? | This Proposal |
|------|-------|-----|---------------|
| Exact | `DeviceRequest.Exactly` → `ExactDeviceRequest` | Yes | Supported |
| Prioritized list | `DeviceRequest.FirstAvailable` | No (`+featureGate=DRAPrioritizedList`) | Skipped |

**GA fields not evaluated (not scheduling-relevant):**

| Field | Reason |
|-------|--------|
| `ExactDeviceRequest.AdminAccess` | Controls access mode only. Does not affect device capacity or availability. GA since K8s 1.32. |
| `DeviceClaim.Config` | Driver-specific opaque configuration. No scheduling impact. |

**Feature-gated fields excluded from this proposal** (can be adopted when they graduate to GA):

| Feature Gate | Excluded Fields |
|-------------|-----------------|
| `DRAPartitionableDevices` | `PerDeviceNodeSelection`, per-device `Device.{NodeName,NodeSelector,AllNodes}`, `SharedCounters`, `ConsumesCounters` |
| `DRAPrioritizedList` | `DeviceRequest.FirstAvailable` |
| `DRADeviceTaints` | `Device.Taints`, `ExactDeviceRequest.Tolerations` |
| `DRAConsumableCapacity` | `Device.AllowMultipleAllocations`, `DeviceCapacity.RequestPolicy`, `ExactDeviceRequest.Capacity` |
| `DRADeviceBindingConditions` | `Device.{BindsToNode,BindingConditions,BindingFailureConditions}` |
| `DRAExtendedResource` | `DeviceClassSpec.ExtendedResourceName` |
| `DRANodeAllocatableResources` | `Device.NodeAllocatableResourceMappings` |
| `DRAListTypeAttributes` | `DeviceAttribute.{IntValues,BoolValues,StringValues,VersionValues}`, CEL `includes()` helper |

When any of these gates become GA in a future Kubernetes release, support can be added as additive optional fields — see [Future Extensibility](#6-future-extensibility).

### Architecture Overview

```
┌───────────────────────────────────────────────────────────────────────────┐
│                          Karmada Control Plane                            │
│                                                                           │
│  Deployment (pod.spec.resourceClaims)                                     │
│                     │                                                     │
│                     ▼                                                     │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ Resource Interpreter: extract ResourceClaimTemplate names         │    │
│  │   1. Extract NodeClaim + ResourceRequest (existing)               │    │
│  │   2. Extract resourceClaimTemplateName references from PodSpec    │    │
│  └─────────────────────────────┬─────────────────────────────────────┘    │
│                                ▼                                          │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ ReplicaRequirements { NodeClaim, ResourceRequest,                 │    │
│  │                        ResourceClaimRefs }                        │    │
│  └─────────────────────────────┬─────────────────────────────────────┘    │
│                                ▼                                          │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ Estimator Client: maps refs to protobuf via gRPC                  │    │
│  └─────────────────────────────┬─────────────────────────────────────┘    │
└────────────────────────────────┼──────────────────────────────────────────┘
                                 │
            ┌────────────────────┼────────────────────┐
            ▼                    ▼                    ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│    Member Cluster     │ │    Member Cluster     │ │    Member Cluster     │
│                       │ │                       │ │                       │
│ ┌───────────────────┐ │ │ ┌───────────────────┐ │ │ ┌───────────────────┐ │
│ │ Estimator Server  │ │ │ │ Estimator Server  │ │ │ │ Estimator Server  │ │
│ └─────────┬─────────┘ │ │ └───────────────────┘ │ │ └───────────────────┘ │
│           ▼           │ │                       │ │                       │
│ ┌───────────────────┐ │ │                       │ │                       │
│ │ DRA Informers     │ │ │                       │ │                       │
│ │  ResourceClaim    │ │ │                       │ │                       │
│ │  Template         │ │ │                       │ │                       │
│ │  ResourceSlice    │ │ │                       │ │                       │
│ │  ResourceClaim    │ │ │                       │ │                       │
│ │  DeviceClass      │ │ │                       │ │                       │
│ └─────────┬─────────┘ │ │                       │ │                       │
│           ▼           │ │                       │ │                       │
│ ┌───────────────────────────────────────────┐   │ │                       │
│ │ DynamicResourceEstimator Plugin:          │   │ │                       │
│ │   1. Resolve templates from local cluster │   │ │                       │
│ │   2. Extract DeviceClaim from templates   │   │ │                       │
│ │   3. Evaluate devices against claims      │   │ │                       │
│ └───────────────────────────────────────────┘   │ │                       │
│ ┌───────────────────────────────────────────┐   │ │                       │
│ │ RunEstimateReplicasPlugins()              │   │ │                       │
│ │   NodeResourceEstimator   → 10            │   │ │                       │
│ │   ResourceQuotaEstimator  → 20            │   │ │                       │
│ │   DynamicResourceEstimator → 4  ← NEW     │   │ │                       │
│ │   Result = min(10, 20, 4)  = 4            │   │ │                       │
│ └───────────────────────────────────────────┘   │ │                       │
└──────────────────────┘ └────────────────────────┘ └───────────────────────┘
```

The `DynamicResourceEstimator` plugin integrates into the existing estimator framework alongside `NodeResourceEstimator` and `ResourceQuotaEstimator`. The framework takes the **minimum** across all plugins, so DRA devices become a bottleneck constraint when they are the scarcest resource.

**Key architectural decision:** The control plane sends only lightweight `ResourceClaimTemplate` name references to the estimator. The estimator resolves templates locally from the member cluster, following the same pattern as `ResourceQuotaEstimator` (which reads `ResourceQuota` objects locally rather than receiving pre-resolved quota data). This keeps all DRA resolution logic in one place (the estimator), provides a natural scheduling guard (template not found → 0 capacity), and keeps the control plane simple.

### 1. API Type Changes

A new `ResourceClaimRefs` field is added to `ReplicaRequirements` in `pkg/apis/work/v1alpha2/binding_types.go`. The type carries lightweight name references from `pod.spec.resourceClaims`, not the resolved device requirements.

```go
type ReplicaRequirements struct {
    // ... existing fields: NodeClaim, ResourceRequest, Namespace, PriorityClassName ...

    // ResourceClaimRefs contains the ResourceClaimTemplate name references
    // from the Pod's resourceClaims field. The estimator resolves these
    // templates on the member cluster to determine DRA device availability.
    // +optional
    ResourceClaimRefs []ResourceClaimReference `json:"resourceClaimRefs,omitempty"`
}

// ResourceClaimReference identifies a ResourceClaimTemplate or ResourceClaim
// referenced by a Pod. The estimator uses these references to look up DRA
// objects on the member cluster.
type ResourceClaimReference struct {
    // ResourceClaimTemplateName is the name of a ResourceClaimTemplate
    // in the same namespace as the workload. Each replica gets its own claim.
    // +optional
    ResourceClaimTemplateName string `json:"resourceClaimTemplateName,omitempty"`

    // ResourceClaimName is the name of a pre-existing ResourceClaim
    // in the same namespace as the workload. Shared across replicas.
    // +optional
    ResourceClaimName string `json:"resourceClaimName,omitempty"`
}
```

**`ResourceClaimTemplateName` vs `ResourceClaimName`:** These fields are mutually exclusive (matching `PodResourceClaim` semantics in K8s). If both are set, the template name takes precedence. `ResourceClaimTemplateName` creates a per-replica claim (each replica gets its own devices) — these constrain `MaxAvailableReplicas`. `ResourceClaimName` references a shared, pre-existing claim (all replicas share the same devices) — these do NOT constrain replica count, but the estimator verifies the claim exists on the member cluster.

**Why lightweight references instead of resolved types?** This follows the `ResourceQuotaEstimator` pattern: the control plane sends only the information the estimator needs to look up the relevant cluster-local resources. The estimator resolves the full DRA object graph locally, where it has access to the member cluster's `ResourceSlice`, `ResourceClaim`, `DeviceClass`, and `ResourceClaimTemplate` objects. Benefits:

- All DRA logic is contained in the estimator plugin (single responsibility).
- Template-not-found naturally returns 0 capacity (prevents scheduling to clusters without templates).
- Simpler gRPC payload (names, not full device claim structures).
- Control plane stays lightweight (no DRA informer needed).

**Protobuf types** in `pkg/estimator/pb/` mirror the API type above with a corresponding proto message definition (`ResourceClaimReference`) and Go struct with protobuf tags. A new field `repeated ResourceClaimReference resourceClaimRefs = 5` is added to the existing `ReplicaRequirements` message.

### 2. Control Plane Changes

The control plane extraction is minimal:

- A new `ExtractResourceClaimRefs()` function iterates `podTemplate.Spec.ResourceClaims` and collects `ResourceClaimTemplateName` and `ResourceClaimName` values. If both are set on the same entry (which should not pass K8s API validation), the template name takes precedence. No template resolution, no DRA informer, no new dependencies.
- The existing `GenerateReplicaRequirements()` function is extended to include the extracted refs. No signature changes.
- The estimator client maps `ResourceClaimRefs` to the protobuf type in the gRPC request. The client always populates the field (regardless of feature gate state) to support version-skewed deployments where the server may be at a different version.
- **Namespace population.** The detector's `applyReplicaInterpretation()` now unconditionally populates `ReplicaRequirements.Namespace` from the resource object's namespace (previously gated behind `ResourceQuotaEstimate`). The DRA estimator uses this field to resolve templates and claims on the member cluster.

**Note:** The PodSpec's `containers[].resources.claims[]` maps containers to claims. This is a runtime concern (which containers access allocated devices) and does NOT affect capacity estimation.

### 3. Estimator Server Changes

When the `DRAEstimator` feature gate is enabled, the estimator server starts four additional informers for `ResourceSlice`, `ResourceClaim`, `DeviceClass`, and `ResourceClaimTemplate`. These are accessed by the plugin through the existing `Handle.SharedInformerFactory()` interface.

### 4. DynamicResourceEstimator Plugin

A new `DynamicResourceEstimator` plugin is added at `pkg/estimator/server/framework/plugins/dynamicresource/` and registered in `NewInTreeRegistry()`. The plugin:

- Returns `noDeviceConstraint (math.MaxInt32)` with `Noopperation` (the estimator framework code indicating the plugin has no opinion and does not constrain the result) when disabled or when no `ResourceClaimRefs` are present — matching the convention of `NodeResourceEstimator` and `ResourceQuotaEstimator`.
- **Resolves references from the member cluster:**
  - `ResourceClaimTemplateName` → looks up the template, extracts device requests that constrain replica count.
  - `ResourceClaimName` → verifies the shared claim exists (does not constrain replicas since all replicas share it).
  - If any referenced object is missing → returns **0 capacity** (natural scheduling guard).
- Uses `k8s.io/dynamic-resource-allocation/cel` for CEL selector evaluation in GA-only mode. A compilation cache (100 entries, keyed by expression string) avoids recompilation across estimation cycles. Cache invalidation is inherent: a changed expression is a different cache key.
- The CEL device context includes `driver`, `attributes`, and `capacity`. The `AllowMultipleAllocations` field is omitted (defaults to `false`) because it is behind the `DRAConsumableCapacity` feature gate. When this feature graduates to GA, the field should be populated.

### 5. Estimation Algorithm

The algorithm uses **per-node simulation** to compute how many replicas a cluster can support, matching the K8s DRA scheduler's `GatherPools` + `allocateOne` approach. Each node is evaluated independently: all device requests must be satisfiable simultaneously on the same node (co-location guarantee). Shared devices are consumed greedily across nodes.

**Steps:**

1. **Resolve references.** For each `ResourceClaimReference`:
   - `ResourceClaimTemplateName`: look up the `ResourceClaimTemplate` from the member cluster by namespace and name. Extract `DeviceRequest` entries from `ExactDeviceRequest` (GA mode only; `FirstAvailable` is feature-gated and skipped with warning). These per-replica requests constrain `MaxAvailableReplicas`.
   - `ResourceClaimName`: verify the pre-existing `ResourceClaim` exists on the member cluster. Shared claims do not constrain replica count (all replicas share the same claim). The estimator uses an optimistic approach: if the shared claim exists (even if not yet allocated), the cluster is considered viable. The member cluster's scheduler handles actual allocation. This mirrors how Karmada treats other pending dependencies such as unbound PVCs.
   - If any referenced template or claim is not found, return 0 capacity immediately.
   - A single template can contain multiple `DeviceRequest` entries (up to `DeviceRequestsMaxSize=32` per K8s API validation). All are extracted and must be satisfiable simultaneously on the same node.
   - For `All` mode: a single replica claims all matching devices across all pools. Capacity is 1 if any matching devices exist, 0 otherwise.

2. **Build pool index.** Load all `ResourceSlice` objects. Group by `(driver, pool.Name)`. For each pool, keep only slices with the highest `pool.Generation`. Skip incomplete pools where `len(slices) != pool.ResourceSliceCount`.

3. **Build allocated device set.** Load all `ResourceClaim` objects (cluster-wide, since device allocation is globally unique). Collect allocated `(driver, pool, deviceName)` tuples.

4. **Classify available devices.** Separate unallocated devices into per-node exclusive and shared pools:
   - `NodeName` → exclusive to that node
   - `AllNodes` → shared pool visible to all nodes
   - `NodeSelector` → shared pool visible only to matching nodes (uses `nodeaffinity.NewNodeSelector` for proper K8s-compatible matching, pre-compiled once per pool)
   - `PerDeviceNodeSelection` → skipped (non-GA)

5. **Handle All mode.** If any request uses `AllocationMode=All`, the maximum capacity is 1 (one replica claims all matching devices). For each node, gather all visible devices and verify that ALL requests (both All-mode and ExactCount) can be satisfied on that node. All-mode requests consume matching devices first; ExactCount requests use remaining devices. If any node satisfies all requests, return 1; otherwise return 0. This enforces co-location even for All mode.

6. **Per-node evaluation with greedy shared consumption (ExactCount mode).** Pre-compile CEL selectors (class + request selectors AND-ed). Track remaining shared devices per pool per request: `remainingShared[pool][R]`. Then for each node (sorted by exclusive device count descending, so self-sufficient nodes consume fewer shared devices):

   ```
   visibleShared[R] = Σ remainingShared[pool][R] for pools visible to this node

   nodeReplicas = min over all requests R of:
       floor((exclusive_matches(node, R) + visibleShared[R]) / R.count)

   For each request R, consume shared devices from visible pools:
       totalNeeded = nodeReplicas * R.count
       fromExclusive = min(totalNeeded, exclusive_matches(node, R))
       deduct (totalNeeded - fromExclusive) from visible pools' remainingShared

   total += nodeReplicas
   ```

7. **Return** `total` (clamped to `int32` range).

**Why per-node simulation?** This matches the K8s DRA scheduler's co-location semantics: all device requests for a pod must be satisfiable on the same node. A per-request independent approach would overcount when different device types exist on different nodes:

```
Example: Pod needs 1 GPU + 1 FPGA

  Node A: 4 GPUs, 0 FPGAs
  Node B: 0 GPUs, 4 FPGAs

  Per-request (WRONG): min(4, 4) = 4
  Per-node (CORRECT):  Node A: min(4, 0) = 0
                        Node B: min(0, 4) = 0
                        Total = 0  ✓ (no node has both)
```

#### Accuracy Limitations

| Limitation | Impact | Guidance |
|------------|--------|----------|
| `DeviceClaim.Constraints` not evaluated (GA field, excluded for complexity) | May overcount when constraints restrict valid device combinations. For example, a constraint requiring "2 GPUs on same NUMA node" on a cluster with 8 GPUs across 4 NUMA nodes could report 4 replicas when only 2 are achievable. | Users with topology-sensitive constraints should expect the estimator to report an upper bound. Consider setting conservative `replicaSchedulingPolicy` weights or using `StaticWeightList` until constraint support is added. |
| Shared claims assumed viable if present | If a shared claim exists but references devices from a pool with no matching ResourceSlices, the cluster is still considered viable. | Low risk: claims without matching device pools are unusual in practice. The member cluster's scheduler rejects the pod if allocation fails. |

**Note on the greedy shared-device algorithm:** Nodes are processed in order of descending exclusive device count so that self-sufficient nodes consume fewer shared devices. The total across all nodes is exact: each replica consumes a fixed number of devices per request, and the algorithm exhausts shared pools across all nodes before returning. Node ordering affects which node hosts each replica, but not the sum.

### 6. Future Extensibility

The `ResourceClaimReference` type is designed for additive extension. Future fields can be added without breaking existing consumers:

| Extension | Change |
|-----------|--------|
| Inline template (if K8s ever adds it) | Add `InlineTemplate` field |

The estimator plugin's internal device-matching logic can also evolve independently:

| Feature Gate (K8s) | Estimator Change | API Impact |
|---------------------|------------------|------------|
| — (`DeviceClaim.Constraints`, GA field) | Add backtracking constraint evaluation inside per-node loop | None |
| `DRADeviceTaints` | Read `Tolerations` from `ExactDeviceRequest` | None |
| `DRAPrioritizedList` | Handle `FirstAvailable` request mode | None |
| `DRAPartitionableDevices` | Read per-device node selection | None |
| `DRAConsumableCapacity` | Read `AllowMultipleAllocations`, populate in CEL device context | None |

Since the estimator resolves templates locally, all these extensions are internal to the estimator — no API type or protobuf changes needed.

**Multi-component workloads:** The `DynamicResourceEstimator` implements only `EstimateReplicasPlugin`, not `EstimateComponentsPlugin`. This is intentional: `ResourceClaimRefs` is added only to `ReplicaRequirements`, not to `ComponentReplicaRequirements`. A future phase can add `ResourceClaimRefs` to `ComponentReplicaRequirements` and implement `EstimateComponentsPlugin` as an additive change.

### 7. Feature Gate

```go
// DRAEstimator controls whether the DRA estimation plugin is enabled.
// owner: @seanlaii
// alpha: Karmada v1.18
DRAEstimator featuregate.Feature = "DRAEstimator"
// Default: false, PreRelease: Alpha
```

The feature gate controls the **estimator server side** only: DRA informers (including `ResourceClaimTemplate`) are not started and the plugin returns `Noopperation` when disabled. The control-plane extraction (`ExtractResourceClaimRefs`) always populates `ResourceClaimRefs` from the PodSpec since it is a trivial read operation with no overhead. This allows the control-plane and estimator to be upgraded independently.

### 8. Concrete Example

**Step 1:** A DRA driver publishes devices via ResourceSlice on the member cluster:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceSlice
metadata:
  name: gpu-nvidia-worker-1
spec:
  driver: gpu.nvidia.com
  pool:
    name: worker-node-1
    generation: 1
    resourceSliceCount: 1
  nodeName: worker-node-1          # GA node-selection mode
  devices:
    - name: gpu-0
      attributes:
        gpu.nvidia.com/model:
          string: "A100"
      capacity:
        gpu.nvidia.com/memory:
          value: "80Gi"
    - name: gpu-1
      attributes:
        gpu.nvidia.com/model:
          string: "A100"
      capacity:
        gpu.nvidia.com/memory:
          value: "80Gi"
```

**Step 2:** A DeviceClass and ResourceClaimTemplate exist on the member cluster (pre-propagated or directly created):

```yaml
apiVersion: resource.k8s.io/v1
kind: DeviceClass
metadata:
  name: gpu.nvidia.com
spec:
  selectors:
    - cel:
        expression: 'device.driver == "gpu.nvidia.com"'
---
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: gpu-claim-template
  namespace: default
spec:
  spec:
    devices:
      requests:
        - name: training-gpu
          exactly:                          # GA request mode (not FirstAvailable)
            deviceClassName: gpu.nvidia.com
            selectors:
              - cel:
                  expression: >-
                    device.capacity["gpu.nvidia.com"]["memory"].compareTo(quantity("40Gi")) >= 0
            allocationMode: ExactCount
            count: 1
```

**Step 3:** The user deploys a workload on the Karmada control plane:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ml-training
spec:
  replicas: 4
  selector:
    matchLabels:
      app: ml-training
  template:
    metadata:
      labels:
        app: ml-training
    spec:
      containers:
        - name: trainer
          image: ml-framework:latest
          resources:
            claims:
              - name: gpu
      resourceClaims:
        - name: gpu
          resourceClaimTemplateName: gpu-claim-template
```

**Step 4:** An existing workload already holds gpu-0 on this cluster:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaim
metadata:
  name: existing-training-claim
  namespace: default
status:
  allocation:
    devices:
      results:
        - driver: gpu.nvidia.com
          pool: worker-node-1
          device: gpu-0
```

**Step 5:** The control plane extracts the reference and the estimator resolves:

- Control plane extracts: `ResourceClaimRefs: [{ResourceClaimTemplateName: "gpu-claim-template"}]`
- Estimator resolves `gpu-claim-template` from the member cluster.
- Extracts `DeviceRequest`: ExactCount, 1x `gpu.nvidia.com`, CEL filter `memory >= 40Gi`.
- Builds allocated set: gpu-0 is allocated (from claim above) → excluded.
- Available: gpu-1 (1 device). CEL: 80Gi >= 40Gi → matches.
- Per-node capacity: `1 match / 1 count = 1` replica for this cluster.
- Combined with CPU/memory estimation: `min(cpuResult, memResult, 1) = 1`.

### Test Plan

**Unit tests:**
- `ExtractResourceClaimRefs`: no claims → empty; template refs → extracted; `ResourceClaimName` → extracted (shared claim verified on member cluster, does not constrain replicas); nil fields → skipped; both template and claim name set → template takes precedence.
- `DynamicResourceEstimator`: disabled → `Noopperation`; no refs → `Noopperation`; template not found → 0 capacity; template found → correct device evaluation; NodeName/NodeSelector/AllNodes slices → correct capacity; allocated devices → excluded; CEL filtering → only matches counted; DeviceClass selectors intersected with request selectors; incomplete pool → skipped; `PerDeviceNodeSelection` → skipped; `AllocationMode=All` → correct; mixed pool types across requests → per-node evaluation correct; multi-pool same node → devices from different drivers combined correctly; large count with integer division remainder → correct; empty namespace → handled; shared devices consumed across multiple nodes with greedy ordering → correct totals.

**E2E tests** (no real hardware required — ResourceSlice/DeviceClass/ResourceClaimTemplate created via `kubectl apply`):

| Scenario | Expected |
|----------|----------|
| Template not on member cluster | Estimator returns 0 |
| Template on member cluster, cluster with available devices | Correct max replicas |
| Partially allocated devices | Allocated devices excluded |
| CEL selector filtering | Only matching devices counted |
| Multi-cluster distribution | Proportional distribution |
| Feature gate disabled | DRA ignored |
| AllNodes devices | Counted once globally |
| NodeSelector devices | Only matching nodes see the devices |
| Cross-node co-location guard | Devices on different nodes, pod needs both → 0 |

## Alternatives

### Alternative 1: Control-Plane Template Resolution

The control plane resolves `ResourceClaimTemplate` objects, extracts a `DeviceClaim` type with full device requirements, and sends it to the estimator via gRPC. The estimator receives pre-resolved requirements.

**Why not chosen:** Adds a `ResourceClaimTemplate` informer to the control plane. Splits DRA logic between control plane (extraction) and estimator (evaluation). Does not provide a natural scheduling guard (control plane reports requirements even if the template hasn't been propagated to the member cluster). Inconsistent with the `ResourceQuotaEstimator` pattern where the estimator reads cluster-local resources.

### Alternative 2: Pass Full PodTemplateSpec Instead of References

Pass the entire `corev1.PodTemplateSpec` in `ReplicaRequirements`. Each plugin extracts what it needs.

**Why not chosen:** Significantly larger gRPC payload; breaks the established Karmada pattern of focused, scheduling-relevant types; tight coupling to upstream PodSpec changes.

### Alternative 3: Use Kubernetes DRA Types Directly

Reference `resource.k8s.io/v1.DeviceClaim` directly in `ReplicaRequirements`.

**Why not chosen:** Tight coupling to upstream API; larger gRPC payload; breaks the `NodeClaim` pattern where Karmada defines focused custom types.
