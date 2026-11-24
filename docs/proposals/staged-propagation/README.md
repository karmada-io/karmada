---
title: Staged propagation for PropagationPolicy and ClusterPropagationPolicy
authors:
- "@zzklachlan"
reviewers:
- "@RainbowMango"
- "@jabellard"
- "@mszacillo"
- "@zhzhuang-zju"
approvers:
- TBD

creation-date: 2026-09-02

---

# Staged propagation for PropagationPolicy and ClusterPropagationPolicy

## Summary

Karmada already has per-cluster dispatch suspension
(`spec.suspension.dispatchingOnClusters`) and per-cluster health
(`ResourceBinding.status.aggregatedStatus[].health`), but nothing that
combines them into an ordered rollout. Users roll out "cluster A,
verify, then cluster B" by hand, with no defined failure behavior and
no observable progress.

This proposal adds an opt-in `rolloutStrategy` to `PropagationSpec`.
In `Staged` mode, Karmada releases one cluster at a time in name order;
each cluster must pass a gate (health, conditions, minimum success
time) within a per-cluster timeout before the next is released. Any
change to what a cluster would receive (template, OverridePolicy, or
ConfigMap/Secret data) is staged, tracked by a hash of each cluster's
rendered manifest. The logic is a `pkg/rollout/` library called from
the existing binding controllers: no new controller, no data-plane
change.

## Motivation

All-at-once propagation is the right default but unsafe for:

- **Restart-sensitive stateful services**, e.g. a Trino coordinator
  (single coordinator, ~30s unavailable per restart). Restarting every
  cluster at once takes the service offline.
- **Blast-radius-sensitive cluster infrastructure**: ingress
  controllers, admission webhooks, CoreDNS overrides (typically
  `ClusterPropagationPolicy`).
- **Tenant-driven validation**: smoke tests or custom readiness
  signals between clusters, beyond `Health == Healthy`.

The [dispatch-suspension proposal](../dispatch-suspension/README.md)
(Story 2) describes ordered rollout as a manual pattern and lists
automating it as a Non-Goal. This proposal takes up that Non-Goal.

### Goals

- Ordered, one-cluster-at-a-time rollout across the clusters selected
  by a single PP or CPP.
- Stage every change to a cluster's rendered manifest, not only
  changes that bump the template's `metadata.generation`.
- A per-cluster gate (`RequireHealthy`, `RequiredConditions`,
  `MinSuccessTime`), a per-cluster `Timeout`, and a declared failure
  policy (`Pause | Continue`).
- Reuse existing primitives (`Work.spec.suspendDispatching`,
  `AggregatedStatusItem`); add control logic and status fields only.
- Opt-in per policy, behind alpha feature gate `StagedPropagation`;
  same support for PP and CPP.

### Non-Goals

- Traffic-shifting canary / blue-green (Argo Rollouts, Flagger, mesh).
- Per-replica progressive rollout within a cluster (`rollingUpdate`).
- A general workflow engine (DAGs, per-step scripts).
- Automatic rollback. v1 stops the rollout; reverting the template is
  a GitOps concern.
- Coordination across policies or across bindings.
- Grouping, batching, or user-specified order (possible v2).
- Combining `Staged` with application failover in v1 (see Reschedule
  and failover). Cluster failover is supported.

## Proposal

Setting `rolloutStrategy.type: Staged` on a PP/CPP makes the binding
controller release clusters one at a time by setting
`suspendDispatching` on each cluster's Work, recording progress in the
`ResourceBinding` / `ClusterResourceBinding` status. Scheduling,
overrides, execution, and status aggregation are unchanged.

### User Stories

#### Story 1: Sequential cluster-by-cluster rollout with health gating

As a service owner running the same HA workload in several clusters, I
want a change to roll out one cluster at a time, holding each until it
has been healthy for a minimum period, so the other clusters keep
serving while one restarts.

```yaml
kind: PropagationPolicy
spec:
  # ... resourceSelectors, placement (selects member-east, member-west) ...
  rolloutStrategy:
    type: Staged
    staged:
      gate: {requireHealthy: true, minSuccessTime: 60s}
      timeout: 30m           # per cluster
      onFailure: {action: Pause}
```

`member-east` is updated first and must stay Healthy for 60s before
`member-west` is released.

#### Story 2: Tenant-driven validation between clusters

Karmada's per-cluster `Healthy` means the workload's pods are up; it
does not prove the new version works. As a service owner, I run my own
smoke-test controller in each cluster that writes
`SmokeTestsPassed=True` onto the workload's `.status.conditions[]`. I
want each cluster held until it is Healthy *and* that condition is
`True`.

```yaml
staged:
  gate:
    requireHealthy: true
    requiredConditions:
      - {type: SmokeTestsPassed, status: "True"}
  timeout: 1h
```

This requires the resource interpreter to preserve conditions (see
Interpreter contract for `RequiredConditions`).

#### Story 3: Automatic pause on failure

If a cluster fails its gate, I want the rollout to pause so the
remaining clusters stay on the previous version until I intervene. My
template is in git, so Karmada does not need to roll back.

### Notes/Constraints/Caveats

- **Scope.** Only resources selected by a `Staged` policy are staged,
  each binding independently. Replica counts are excluded, so scaling,
  rebalancing, and failover replica moves reach released clusters
  immediately; a suspended cluster gets them when released.
- **Dependent resources are not coordinated.** A ConfigMap in a
  `Staged` policy gets its own rollout, gated on the ConfigMap's own
  (always Healthy) status, not on the pods that mount it. To gate a
  config change on workload health, reference it from the pod template
  (e.g. a checksum annotation).
- **Enable `Staged` as its own step.** If a policy change to `Staged`
  and a template change land together, the detector handles them
  independently, and the template change may go out all at once first.
- **The gate is checked once per change.** A cluster that becomes
  Unhealthy after passing is a monitoring or failover concern.
- **Suspension is a union.** User suspension (`spec.suspension`) and
  rollout suspension combine; rollout progress never releases a
  user-suspended cluster. Overlap emits a warning, not a rejection.
- **Binding API version.** The policy field is in `policy/v1alpha1`.
  On bindings, the copied `spec.rolloutStrategy` and `status.rollout`
  exist only in `work/v1alpha2` (the storage version), like
  `spec.suspension`; writing a binding through `work/v1alpha1` drops
  them.

### Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Gate reads status left over from the previous version | Gate evaluates only fresh status (Status freshness). |
| Rollout steps churn the scheduler and other controllers | Rollout writes only `status.rollout`, which doesn't bump generation. |
| Controller restart or lost status mid-rollout | Desired hashes are recomputed; state is in status; losing it re-verifies every cluster, never releases one. |
| Unreachable cluster stalls the rollout | Its status is not fresh; `Timeout` bounds the wait; `Pause` protects later clusters. |
| `Healthy` is a weak signal | `RequiredConditions`. Karmada observes conditions; it does not run tests. |
| A Karmada upgrade changes how manifests render, changing every hash | Karmada metadata is excluded from the hash; a residual change causes a staged (not all-at-once) rollout. |

## Design Details

The PP/CPP declares the rollout (`spec.rolloutStrategy`); each binding
records progress in `status.rollout`. Which clusters are held is
derived on every reconcile, never stored. `ClusterResourceBinding`
embeds the same spec and status types, so everything below applies to
CRB unchanged.

### API changes

All new fields are optional, so no API version changes. Who writes
what:

| Resource | Field | Written by | Purpose |
| --- | --- | --- | --- |
| PP / CPP | `spec.rolloutStrategy` | user | declares the rollout |
| RB / CRB | `spec.rolloutStrategy` | detector (copied from the policy) | input to the binding controller |
| RB / CRB | `status.rollout` | binding controller | rollout progress; the only persisted rollout state |
| RB / CRB | `status.conditions[type=RolloutProgressing]` | binding controller | rollout summary for humans and tooling |
| Work | annotation `work.karmada.io/manifest-hash` in the manifest | binding controller | desired hash, carried to the member |
| Work | `status.manifestStatuses[].{manifestHash, memberGeneration, memberObservedGeneration}` | work-status controller | freshness data read from the member object |
| RB / CRB | `status.aggregatedStatus[].{same three}` | rb-status controller (`assembleWorkStatus`) | freshness data for `Compute` |

#### PropagationPolicy / ClusterPropagationPolicy

`PropagationSpec` is shared by PP and CPP (`policy.karmada.io/v1alpha1`).

```go
type PropagationSpec struct {
    // ... existing fields ...

    // RolloutStrategy declares how changes to the selected resources are
    // rolled out to their target clusters.
    // nil means AllAtOnce: every cluster is updated as soon as the change
    // is rendered (today's behavior).
    // +optional
    RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`
}

type RolloutStrategyType string

const (
    // RolloutStrategyAllAtOnce updates every target cluster at once.
    RolloutStrategyAllAtOnce RolloutStrategyType = "AllAtOnce"
    // RolloutStrategyStaged updates one target cluster at a time, gating
    // each before the next is released. Requires the StagedPropagation
    // feature gate.
    RolloutStrategyStaged RolloutStrategyType = "Staged"
)

type RolloutStrategy struct {
    // Type is the rollout strategy.
    // +kubebuilder:validation:Enum=AllAtOnce;Staged
    // +kubebuilder:default=AllAtOnce
    Type RolloutStrategyType `json:"type"`

    // Staged configures a Staged rollout. Required when Type is Staged;
    // must be unset when Type is AllAtOnce.
    // +optional
    Staged *StagedRollout `json:"staged,omitempty"`
}

// StagedRollout releases target clusters one at a time, in cluster name
// order. Each released cluster must pass Gate within Timeout before the
// next one is released.
type StagedRollout struct {
    // Gate is what a released cluster must satisfy before the next
    // cluster is released. nil means {} (all gate defaults).
    // +optional
    Gate *RolloutGate `json:"gate,omitempty"`

    // Timeout is how long a released cluster has to pass Gate, measured
    // from when it is released. On expiry the cluster fails and OnFailure
    // applies. Must be > 0. Defaults to 30m.
    // +optional
    Timeout *metav1.Duration `json:"timeout,omitempty"`

    // OnFailure is what happens when a cluster fails Gate.
    // nil means {action: Pause}.
    // +optional
    OnFailure *RolloutFailurePolicy `json:"onFailure,omitempty"`
}

// RolloutGate is evaluated per cluster, only once the cluster's status
// reflects the new manifest (see Status freshness). All set checks must
// hold continuously for MinSuccessTime.
type RolloutGate struct {
    // RequireHealthy requires the resource's health in that cluster
    // (AggregatedStatusItem.Health, from the resource interpreter) to be
    // Healthy. Defaults to true.
    // +optional
    RequireHealthy *bool `json:"requireHealthy,omitempty"`

    // RequiredConditions are conditions that must be present in the
    // member resource's status.conditions[] with the given status.
    // +optional
    RequiredConditions []ConditionRequirement `json:"requiredConditions,omitempty"`

    // MinSuccessTime is how long the checks must hold continuously
    // before the cluster passes. Any check turning false resets the
    // clock. Must be >= 0 and < Timeout. Defaults to 0.
    // +optional
    MinSuccessTime *metav1.Duration `json:"minSuccessTime,omitempty"`
}

// ConditionRequirement matches a condition by type in the member
// resource's status.conditions[]. A missing condition counts as Unknown.
// If the condition carries observedGeneration, it matches only when that
// is >= the member resource's metadata.generation.
type ConditionRequirement struct {
    // Type is the condition type, e.g. "Available" or "SmokeTestsPassed".
    // +kubebuilder:validation:MinLength=1
    Type string `json:"type"`

    // Status is the required condition status. Defaults to True.
    // +kubebuilder:validation:Enum=True;False;Unknown
    // +kubebuilder:default=True
    // +optional
    Status metav1.ConditionStatus `json:"status,omitempty"`
}

type RolloutFailureAction string

const (
    // RolloutFailureActionPause stops the rollout: clusters not yet
    // updated stay on their current version until a new change arrives
    // or the strategy is removed.
    RolloutFailureActionPause RolloutFailureAction = "Pause"
    // RolloutFailureActionContinue records the failure and releases the
    // next cluster.
    RolloutFailureActionContinue RolloutFailureAction = "Continue"
)

type RolloutFailurePolicy struct {
    // Action is the failure action.
    // +kubebuilder:validation:Enum=Pause;Continue
    // +kubebuilder:default=Pause
    Action RolloutFailureAction `json:"action"`
}
```

Only condition `type` and `status` are compared, so heartbeat updates
don't reset the `MinSuccessTime` clock. `Timeout` is per cluster, so
the worst case for a whole rollout is `N × Timeout`. Validation and
defaults are summarized in Feature gate, validation, and defaulting.

#### ResourceBinding / ClusterResourceBinding

`ClusterResourceBinding` embeds the same spec and status types
(`work.karmada.io/v1alpha2`).

**Spec.** The detector copies the policy's `rolloutStrategy`, as it
does `Failover` and `ConflictResolution`. Users don't set it on
bindings, and the binding controller never writes binding spec.

```go
type ResourceBindingSpec struct {
    // ... existing fields ...

    // RolloutStrategy is copied from the PropagationPolicy or
    // ClusterPropagationPolicy by the detector.
    // +optional
    RolloutStrategy *policyv1alpha1.RolloutStrategy `json:"rolloutStrategy,omitempty"`
}
```

**Status.** `status.rollout` is written only by the binding controller,
only while the strategy is `Staged`, and cleared when it isn't.

```go
type ResourceBindingStatus struct {
    // ... existing fields ...

    // Rollout is the progress of a Staged rollout. nil when the binding's
    // rolloutStrategy is not Staged.
    // +optional
    Rollout *RolloutStatus `json:"rollout,omitempty"`
}

type RolloutStatus struct {
    // Phase summarizes the rollout.
    // +kubebuilder:validation:Enum=Pending;Progressing;Succeeded;Failed
    Phase RolloutPhase `json:"phase"`

    // CurrentCluster is the cluster being gated; when Phase is Failed, the
    // cluster that failed. Empty when Pending or Succeeded. It is also the
    // position the next pass searches from (see Rollout order).
    // +optional
    CurrentCluster string `json:"currentCluster,omitempty"`

    // Clusters holds one entry per target cluster that has been through
    // the rollout. A target with no entry has never passed the gate.
    // +listType=map
    // +listMapKey=name
    // +optional
    Clusters []ClusterRolloutStatus `json:"clusters,omitempty"`

    // CurrentClusterStartedAt is when CurrentCluster was released; its
    // Timeout is measured from here.
    // +optional
    CurrentClusterStartedAt *metav1.Time `json:"currentClusterStartedAt,omitempty"`

    // GateSatisfiedSince is when CurrentCluster began passing every gate
    // check continuously; nil while any check fails. MinSuccessTime is
    // measured from here.
    // +optional
    GateSatisfiedSince *metav1.Time `json:"gateSatisfiedSince,omitempty"`

    // StartedAt and CompletedAt cover the whole rollout of this binding,
    // not a single cluster. StartedAt is set when Phase becomes
    // Progressing from Pending, Succeeded, or Failed; a change arriving
    // mid-rollout doesn't reset it (the NewRevision reason records it).
    // CompletedAt is when Phase last became Succeeded.
    // +optional
    StartedAt *metav1.Time `json:"startedAt,omitempty"`
    // +optional
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`
}

type RolloutPhase string

const (
    // RolloutPhasePending: the binding has no target clusters yet.
    RolloutPhasePending RolloutPhase = "Pending"
    // RolloutPhaseProgressing: some target is not up to date.
    RolloutPhaseProgressing RolloutPhase = "Progressing"
    // RolloutPhaseSucceeded: every target is up to date.
    RolloutPhaseSucceeded RolloutPhase = "Succeeded"
    // RolloutPhaseFailed: CurrentCluster failed the gate under Pause.
    RolloutPhaseFailed RolloutPhase = "Failed"
)

type ClusterRolloutStatus struct {
    // Name is the cluster name.
    Name string `json:"name"`

    // State summarizes the cluster for display; Compute never reads it.
    // +kubebuilder:validation:Enum=Verified;Gating;Suspended;Failed
    // +optional
    State ClusterRolloutState `json:"state,omitempty"`

    // DesiredHash is the manifest hash last rendered for this cluster.
    // Compute recomputes it every reconcile; the recorded value is used
    // only to detect a new change while Phase is Failed.
    // +optional
    DesiredHash string `json:"desiredHash,omitempty"`

    // VerifiedHash is the last manifest hash that passed the gate on this
    // cluster (or was forced past by onFailure: Continue). The cluster is
    // up to date when DesiredHash equals VerifiedHash.
    // +optional
    VerifiedHash string `json:"verifiedHash,omitempty"`

    // FailedHash is the manifest hash that failed the gate on this
    // cluster under onFailure: Pause. Cleared when the cluster passes.
    // +optional
    FailedHash string `json:"failedHash,omitempty"`
}

type ClusterRolloutState string

const (
    // Verified: up to date; released.
    ClusterRolloutVerified ClusterRolloutState = "Verified"
    // Gating: CurrentCluster; released and being gated.
    ClusterRolloutGating ClusterRolloutState = "Gating"
    // Suspended: not up to date; held on the version it already runs.
    ClusterRolloutSuspended ClusterRolloutState = "Suspended"
    // Failed: failed the gate under Pause; released (it already applied
    // the failing version).
    ClusterRolloutFailed ClusterRolloutState = "Failed"
)
```

**Condition and events.** The binding controller maintains one
condition, `RolloutProgressing`, in the binding's existing
`status.conditions` (alongside `Scheduled` and `FullyApplied`), so
standard tooling such as `kubectl wait --for=condition=...` works. It
is removed when the binding leaves `Staged` mode. The binding
controller also emits events on the binding and its resource template:

| `RolloutProgressing` status | Reason | When |
| --- | --- | --- |
| `True` | `ClusterGating` | a cluster is released and being gated |
| `True` | `NewRevision` | a new change arrived mid-rollout or after `Succeeded` / `Failed` |
| `False` | `RolloutSucceeded` | every target is up to date |
| `False` | `GateFailed` | `CurrentCluster` failed under `Pause` (message names the cluster and cause: `Timeout` or `ApplicationFailure` eviction) |

| Event | Type | When |
| --- | --- | --- |
| `RolloutClusterReleased` | Normal | a cluster becomes `CurrentCluster` |
| `RolloutClusterPassed` | Normal | a cluster passes the gate |
| `RolloutClusterFailed` | Warning | a cluster fails the gate (message names the `onFailure` action taken) |
| `RolloutSucceeded` | Normal | the phase becomes `Succeeded` |

#### Work

The binding controller writes the desired hash as the annotation
`work.karmada.io/manifest-hash` in the Work's manifest (only for
`Staged` bindings). It is added to `util.ManagedResourceAnnotations`,
so it is stripped when a resource stops being managed. The
work-status controller reads it back, with the generations, from the
member object:

```go
// workv1alpha1.ManifestStatus
type ManifestStatus struct {
    // ... existing fields (Identifier, Status, Health) ...

    // ManifestHash is the work.karmada.io/manifest-hash annotation on the
    // member object: the hash of the manifest the member is running.
    // +optional
    ManifestHash string `json:"manifestHash,omitempty"`

    // MemberGeneration is the member object's metadata.generation;
    // 0 for kinds that don't track generation.
    // +optional
    MemberGeneration int64 `json:"memberGeneration,omitempty"`

    // MemberObservedGeneration is the member object's
    // status.observedGeneration; nil if the object doesn't report it.
    // +optional
    MemberObservedGeneration *int64 `json:"memberObservedGeneration,omitempty"`
}
```

#### AggregatedStatusItem

`workv1alpha2.AggregatedStatusItem` gains the same three fields, with
the same meaning. `assembleWorkStatus` copies them from the Work (see
Status freshness).

#### Example

A policy for Story 1 with every field set explicitly:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: trino-coordinator
spec:
  resourceSelectors:
    - {apiVersion: apps/v1, kind: Deployment, name: trino-coordinator}
  placement:
    clusterAffinity:
      clusterNames: [member-a, member-b, member-c]
  rolloutStrategy:
    type: Staged
    staged:
      gate:
        requireHealthy: true
        requiredConditions:
          - {type: Available, status: "True"}
        minSuccessTime: 60s
      timeout: 30m
      onFailure: {action: Pause}
```

The binding while `member-b` is being gated, after `member-a` passed
the new version:

```yaml
status:
  conditions:
    - {type: Scheduled, status: "True", ...}
    - {type: FullyApplied, status: "True", ...}
    - {type: RolloutProgressing, status: "True", reason: ClusterGating, message: "member-b released; gate satisfied since 10:03:10"}
  aggregatedStatus:
    - {clusterName: member-a, health: Healthy, manifestHash: "v1:9c2e…", memberGeneration: 6, memberObservedGeneration: 6}
    - {clusterName: member-b, health: Healthy, manifestHash: "v1:9c2e…", memberGeneration: 6, memberObservedGeneration: 6}
    - {clusterName: member-c, health: Healthy, manifestHash: "v1:1b7d…", memberGeneration: 5, memberObservedGeneration: 5}
  rollout:
    phase: Progressing
    currentCluster: member-b
    currentClusterStartedAt: "2026-11-24T10:02:00Z"
    gateSatisfiedSince: "2026-11-24T10:03:10Z"
    startedAt: "2026-11-24T09:58:00Z"
    clusters:
      - {name: member-a, state: Verified,  desiredHash: "v1:9c2e…", verifiedHash: "v1:9c2e…"}
      - {name: member-b, state: Gating,    desiredHash: "v1:9c2e…", verifiedHash: "v1:1b7d…"}
      - {name: member-c, state: Suspended, desiredHash: "v1:9c2e…", verifiedHash: "v1:1b7d…"}
```

`member-b`'s reported hash matches its desired hash and its
generations are equal, so its status is fresh; it passes at 10:04:10.
`member-c` still reports the old hash because its Work is suspended.
Had `member-b` timed out instead:

```yaml
status:
  conditions:
    - {type: RolloutProgressing, status: "False", reason: GateFailed, message: "member-b did not pass the gate within 30m"}
  rollout:
    phase: Failed
    currentCluster: member-b
    clusters:
      - {name: member-a, state: Verified,  desiredHash: "v1:9c2e…", verifiedHash: "v1:9c2e…"}
      - {name: member-b, state: Failed,    desiredHash: "v1:9c2e…", verifiedHash: "v1:1b7d…", failedHash: "v1:9c2e…"}
      - {name: member-c, state: Suspended, desiredHash: "v1:9c2e…", verifiedHash: "v1:1b7d…"}
```

`member-c` stays on the old version until a new change (gated on
`member-b` first) or removal of `rolloutStrategy`.

### Manifest hash

The template's `metadata.generation` misses OverridePolicy edits
(Works are re-rendered without a template change) and kinds with no
generation (ConfigMap, Secret). The rollout is therefore keyed on a
hash of each cluster's rendered manifest.

`rollout.ManifestHash` is a single function, applied identically to a
freshly rendered manifest (after `ReviseReplica`, Job completions, and
`ApplyOverridePolicies`) and, during adoption, to a manifest stored in
a Work. It:

1. applies the same pruning `CreateOrUpdateWork` does
   (`prune.RemoveIrrelevantFields`: `status`, `resourceVersion`, `uid`,
   ...). Without this, every status update Karmada writes back to the
   template would change the hash;
2. zeroes the fields Karmada sets per cluster from the scheduling
   result: the replica field via `ReviseReplica(copy, 0)`, and
   `spec.completions` for Jobs (set by `applyJobCompletions`);
3. drops Karmada-managed metadata (`util.ManagedResourceLabels`,
   `util.ManagedResourceAnnotations`, the hash annotation itself);
4. returns a version-prefixed SHA-256 of the JSON, e.g. `v1:9c2e…`.
   Changing the algorithm requires a new prefix.

Each cluster has three values:

| Value | Meaning | Where it lives |
| --- | --- | --- |
| desired | what the cluster should run | recomputed every reconcile; written as annotation `work.karmada.io/manifest-hash` in the Work's manifest |
| reported | what the cluster runs | read from that annotation on the member object (see Status freshness) |
| verified | last hash that passed the gate | `status.rollout.clusters[].verifiedHash` |

Desired needs no storage: it is a deterministic function of the
template, OverridePolicies, and binding spec. Reported is the
annotation value read back from the member, not a hash recomputed
there, so desired and reported always come from the same function.
The annotation is added
only for `Staged` bindings and is listed in
`util.ManagedResourceAnnotations`, so it is stripped when a resource
stops being managed.

### Status freshness

`Health` and conditions carry no revision, so right after a change
they still describe the old manifest. When reflecting status, the
work-status controller also records `ManifestHash`, `MemberGeneration`,
and `MemberObservedGeneration` (see API changes) from the same
member-object snapshot.

These are read from object metadata, not through the interpreter
(only four native reflectors report generations). `assembleWorkStatus`
(`pkg/util/helper/workstatus.go`), which today copies only `Status`
and `Health` from the Work into `AggregatedStatusItem`, also copies
the three new fields; otherwise `Compute` would always see an empty
hash. A cluster is fresh when:

- `ManifestHash == desired` (the new manifest reached the member), and
- `MemberObservedGeneration >= MemberGeneration` (the member's
  controller processed it). Skipped when `MemberObservedGeneration` is
  unset or `MemberGeneration` is 0 (kinds that don't track generation).

An empty hash is never fresh. The gate and its `MinSuccessTime` clock
start only once a cluster is fresh. A `RequiredConditions` entry
whose matching condition carries `observedGeneration` (as
`metav1.Condition` does) matches only if it is `>= MemberGeneration`.

A matching hash proves the new manifest was written, not that the
object's controller has reacted. If the object is processed by a
controller (`MemberGeneration > 0`) but reports no sign of having
processed the new spec (no `status.observedGeneration`, and no
matching required condition carrying `observedGeneration`), the gate
waits at least 30s, or half of `timeout` if that is shorter
(`max(minSuccessTime, min(30s, timeout/2))`), so status left over from
the previous version can't pass immediately and the floor never makes
the gate unpassable. Otherwise `minSuccessTime` applies as written.
Kinds that don't track generation (ConfigMap, Secret) have no
controller and no status to go stale.

### Interpreter contract for `RequiredConditions`

`RequiredConditions` reads `.status.conditions[]` from
`AggregatedStatusItem.Status`. Native reflectors keep conditions for
`Job` and for kinds using `reflectWholeStatus` (most CRDs), but drop
them for `Deployment`, `DaemonSet`, `StatefulSet`, and `ReplicaSet`: their
`Wrapped*Status` types embed the full Kubernetes status, but the
`reflect*Status` functions copy only replica fields. Fix: also copy
`Conditions`, stripping Deployment's `LastUpdateTime` (it changes on
every progress update) to avoid `Work.Status` churn. Aggregators don't
read them, so federated status is unchanged. Custom interpreters must
preserve `Conditions`; otherwise the cluster fails with
`Reason=ConditionsMissing` after `Timeout`.

### Rollout reconciliation

For `Staged` bindings, `ensureWork` is split into render and write,
with the rollout decision between them:

```text
syncBinding
 ├─ 1. render: per target cluster → manifest, desired[cluster] = ManifestHash(manifest)
 ├─ 2. Compute(strategy, prevStatus, aggregatedStatus, desired, now)
 │       -> (suspendedClusters, newStatus, requeueAfter)
 │     write status.rollout
 └─ 3. CreateOrUpdateWork(manifest, suspendDispatching) per cluster
```

- **Targets** are `mergeTargetClusters(spec.Clusters, spec.RequiredBy)`,
  the set `ensureWork` already dispatches to, sorted by name.
- **Suspended set**: every target that is not up to date, except
  `CurrentCluster`; empty when `Succeeded`. It is passed in memory to
  `shouldSuspendDispatching`, which unions it with `spec.suspension`.
  Each cluster's `state` mirrors it.
- **Passing the gate** sets `VerifiedHash = desired` and clears
  `FailedHash`.
- **No leak.** `Compute` sees a new hash before the Work carrying it is
  written, so a change to a released cluster is suspended in the same
  Work update that delivers it.
- **Status only.** Rollout steps never bump the binding's generation,
  so the scheduler, dependencies distributor, and graceful eviction
  controller don't react. `status.rollout` is persisted before any Work
  it releases, so a crash in between can't release a cluster without
  its predecessor's `VerifiedHash` on record.
- **Trigger.** Both binding controllers filter events with
  `GenerationChangedPredicate`, so status changes never reach them
  today. The RB/CRB watch becomes `Or(GenerationChanged,
  aggregatedStatusChanged)`, the latter only for `Staged` bindings and
  ignoring `status.rollout`. Timers use `requeueAfter`.

```mermaid
stateDiagram-v2
    direction TB
    state adopt <<choice>>

    [*] --> adopt : Staged, no status.rollout<br/>(adopt released Works as verified)
    adopt --> Pending : no targets yet
    adopt --> Succeeded : every target up to date
    adopt --> Progressing : some target not up to date

    Pending --> Progressing : scheduler adds targets

    Progressing --> Progressing : CurrentCluster passes gate, or fails with Continue<br/>verifiedHash = desired, then next target from current position
    Progressing --> Succeeded : every target up to date
    Progressing --> Failed : Timeout or ApplicationFailure eviction, onFailure = Pause<br/>(failedHash = desired)

    Succeeded --> Progressing : new change<br/>(start at first target)
    Failed --> Progressing : new change<br/>(start at failed cluster)
```

**Entering Staged mode (adoption).** Staging applies to changes made
after it is enabled. When `status.rollout` is absent, every target
whose Work exists and is not suspended is adopted with
`VerifiedHash = ManifestHash(the Work's current manifest)`. So enabling
`Staged` on a deployed workload goes straight to `Succeeded`, and the
hash annotation is added in the same Work write (top-level metadata;
no pod restart). Targets with no Work (a new workload or cluster) or a
user-suspended Work are not adopted and are gated normally. Adoption
reads the Work because member objects carry no hash annotation yet;
whatever is in a released Work was dispatched already.

**Leaving Staged mode.** When the strategy is unset or `AllAtOnce`, or
the feature gate is disabled, `Compute` isn't called, nothing is
suspended, and `status.rollout` is cleared so a later re-entry adopts
afresh.

### Rollout order

`CurrentCluster` is the first target that is not up to date, searching
by name from the current position (the previous `currentCluster`) and
wrapping around. With no rollout in progress, the search starts at the
first target.

- **Change after `Succeeded`**: starts at the first target.
- **Change mid-rollout**: `CurrentCluster` keeps its slot (its gate
  restarts), then the rollout continues and wraps around to clusters
  that passed the old hash. Restarting from the first target would
  starve the tail when changes arrive faster than a full rollout.
- **Change after `Failed`**: the position is the failed cluster, so the
  fix is gated there first instead of the cluster being held on a
  known-bad version.

If changes arrive faster than a single cluster's gate, nothing can be
verified under any order; batch changes or shorten `MinSuccessTime`.

### Reschedule and failover

`Compute` re-reads the target list every reconcile.

- **Cluster added** (placement change or failover replacement): not up
  to date; released when the rollout reaches it.
- **Cluster removed** (placement change or taint-based cluster
  failover): its entry is dropped. If it was `CurrentCluster`, the
  rollout moves on; the next cluster is still gated. Removing the
  failed cluster does not leave `Failed`.
- **All clusters removed**: back to `Pending`.

An evicted cluster is no longer a target, so it never receives a newer
manifest; its Work is kept or deleted per `PurgeMode` as today.

**Application failover is rejected with `Staged` in v1.** It evicts on
the same health signal the gate uses, so a bad version on
`CurrentCluster` would look like a removal, and the rollout would carry
it to the next cluster and to each replacement. The webhook rejects
the combination. As a safety net, an `ApplicationFailure` eviction of
`CurrentCluster` (`spec.gracefulEvictionTasks`) counts as a gate
failure. Cluster failover can't be excluded this way, since it is
triggered by cluster taints rather than a policy opt-in. A workload
that prefers no failover can tolerate the `cluster.karmada.io/not-ready`
and `cluster.karmada.io/unreachable` `NoExecute` taints in
`placement.clusterTolerations` (without `tolerationSeconds`); a failed
cluster then stays a target, and `Timeout` and `Pause` handle it as a
stalled cluster.

**Known limitation.** A failover replacement has nothing running, so
holding it means no workload. Mid-rollout, a replacement runs nothing
until the rollout reaches it (or, under `Failed`, until the user
recovers). Outside a rollout it is released immediately.

### Failure handling

- **Pause** (default): `Phase=Failed`, `FailedHash = desired`,
  `VerifiedHash` unchanged; clusters that are not up to date stay
  suspended. The rollout stays `Failed`, without re-gating, until a new
  change arrives: a target with an entry renders a hash different from
  its recorded `DesiredHash`. Evictions and replacements don't resume
  it. To recover, push a fix (gated on the failed cluster first) or
  remove `rolloutStrategy`. To retry the same version, make a no-op
  template change (e.g. an annotation).
- **Continue**: `VerifiedHash = desired` despite the failure; emit a
  `Warning` event and move on.

### Feature gate, validation, and defaulting

- Feature gate `StagedPropagation` (alpha). When disabled, the webhook
  rejects `type: Staged`.
- `type: Staged` requires `staged`; `type: AllAtOnce` must not set
  `staged`, so gate, timeout, and failure settings are never silently
  ignored.
- Durations: `timeout > 0`, `minSuccessTime >= 0`, and
  `minSuccessTime < timeout` (using defaults for unset values), so a
  healthy cluster can always pass the gate before it times out.
- The webhook rejects `Staged` together with `failover.application`,
  alongside the existing rule that `failover.application` requires
  `propagateDeps`.
- Defaults: `Type=AllAtOnce`, `Timeout=30m`, `OnFailure.Action=Pause`,
  `Gate.RequireHealthy=true`, `Gate.MinSuccessTime=0`; omitted `gate`
  means `{}`.

### Test Plan

**Unit** (`pkg/rollout`, webhook, detector, work-status controller):

- Validation and defaults: `Staged` without `staged` and `AllAtOnce`
  with `staged` are rejected; zero/negative `timeout`, negative
  `minSuccessTime`, and `minSuccessTime >= timeout` (including against
  the default `timeout`) are rejected; `Staged` +
  `failover.application` is rejected on create and update.
- `ManifestHash`: deterministic; changes on template, OverridePolicy,
  and ConfigMap data edits; unchanged by replicas, divided Job
  `completions`, Karmada metadata, and template `status` /
  `resourceVersion`; equal for a rendered manifest and its stored Work
  copy.
- Transitions and order: sort order from `Succeeded`; mid-rollout
  change keeps `CurrentCluster` and wraps; fix after `Failed` starts at
  the failed cluster; an OverridePolicy on one cluster re-gates only
  that cluster.
- Freshness: mismatched or empty hash, or `observedGeneration <
  generation`, neither passes nor starts the clock; skip rules for
  unset values. A condition with an older `observedGeneration` doesn't
  match. A kind with `MemberGeneration > 0` and no generation signal
  waits at least 30s even with `minSuccessTime: 0`; a Deployment and a
  ConfigMap don't. The work-status controller records the three fields,
  and `assembleWorkStatus` carries them from the Work to the binding.
- Failure: `Failed` persists without re-gating; evicting the failed
  cluster or adding a replacement doesn't resume it; a new hash does.
  `Continue` sets `VerifiedHash`.
- Adoption: a deployed workload enters `Succeeded`; targets with no
  Work or a suspended Work are gated.
- Failover: taint eviction of `CurrentCluster` moves on;
  `ApplicationFailure` eviction counts as a gate failure.
- Reconcile: rollout writes only status (generation unchanged across a
  rollout); the predicate fires on `aggregatedStatus` only; restarts
  and crashes between writes never release an unverified cluster.
- Reflectors keep `Conditions` for the four replica-based kinds.
- Each transition sets the documented `RolloutProgressing` reason and emits
  the documented event.

**Integration & E2E:**

- N-cluster rollout toggles `suspendDispatching` in order; `Pause`
  holds later clusters.
- Condition-driven promotion (`Available`, `SmokeTestsPassed`).
- After `Succeeded`, an OverridePolicy edit and a ConfigMap change each
  roll out in order.
- CPP with namespaced and cluster-scoped targets; Argo CD / Flux sync
  of the PP doesn't fight the rollout.

## Alternatives

- **Suspension on PP / CPP spec.** Rejected: GitOps drift, detector
  fan-out on every step, collision with user suspension.
- **Storing held clusters in RB / CRB spec** (an earlier
  `spec.suspension.rollout`). Rejected: it duplicates `status.rollout`,
  and every write bumps generation and wakes the scheduler.
- **Dedicated rollout controller.** Rejected for v1: it would need a
  stored hand-off to the binding controller. `pkg/rollout/` is a pure
  library, so extracting one later is possible.

## Future work

- **Application failover with `Staged`**: how health-driven evictions
  count toward the gate, and what a replacement runs mid-rollout.
- **Early release of replacements** whose hash already passed on
  another cluster, to close the capacity gap.
- **Explicit retry** of the same hash without a no-op template change.
