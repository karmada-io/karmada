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
In `Staged` mode, Karmada updates one cluster at a time in alphabetical
order; each cluster must pass a gate (health, conditions, minimum
success time) within a per-cluster timeout before the next one is
released. The logic is a `pkg/rollout/` library called from the
existing binding controllers: no new controller, no data-plane change.

## Motivation

All-at-once propagation is the right default but unsafe for:

- **Restart-sensitive stateful services** — e.g. a Trino coordinator
  (single coordinator, ~30s unavailable per restart). Restarting every
  cluster at once takes the service offline.
- **Blast-radius-sensitive cluster infrastructure** — ingress
  controllers, admission webhooks, CoreDNS overrides (typically
  `ClusterPropagationPolicy`).
- **Tenant-driven validation** — smoke tests or custom readiness
  signals between clusters, beyond `Health == Healthy`.

The [dispatch-suspension proposal](../dispatch-suspension/README.md)
(Story 2) describes ordered rollout as a manual pattern and lists
automating it as a Non-Goal. This proposal takes up that Non-Goal.

### Goals

- Ordered, one-cluster-at-a-time rollout across the clusters selected
  by a single PP or CPP, in deterministic (sorted) order.
- A per-cluster gate (`RequireHealthy`, `RequiredConditions`,
  `MinSuccessTime`) and a per-cluster `Timeout`.
- A declared failure policy (`Pause | Continue`).
- Reuse existing primitives (`Work.spec.suspendDispatching`,
  `AggregatedStatusItem`); add control logic and a few status fields.
- Opt-in per policy, behind alpha feature gate `StagedPropagation`;
  no-op for existing policies.
- Same support for `PropagationPolicy` and `ClusterPropagationPolicy`.

### Non-Goals

- Traffic-shifting canary / blue-green (Argo Rollouts, Flagger, mesh).
- Per-replica progressive rollout within a cluster (`rollingUpdate`).
- A general workflow engine (DAGs, per-step scripts).
- Automatic workload rollback. v1 stops the rollout; reverting the
  template is a GitOps concern.
- Coordination across policies or across bindings.
- Grouping, batching, or user-specified order (possible v2).

## Proposal

Setting `rolloutStrategy.type: Staged` on a PP/CPP makes the binding
controller release clusters one at a time by writing per-cluster
suspension onto the `ResourceBinding` / `ClusterResourceBinding`.
Scheduling, overrides, Work generation, execution, and status
aggregation are unchanged.

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

If a cluster fails its gate (times out before becoming Healthy, or a
required condition never becomes `True`), I want the rollout to pause
so the remaining clusters stay on the previous version until I
intervene. My template is in git, so Karmada does not need to roll
back.

### Notes/Constraints/Caveats

- **Suspension is a union.** A cluster is suspended if either the user
  (`Dispatching`, `DispatchingOnClusters`) or the rollout
  (`Rollout.SuspendedClusters`) suspends it. Rollout progress never
  releases a user-suspended cluster.
- **Per-binding rollout.** A PP selecting N templates produces N
  bindings, each rolling out independently.
- **v1alpha2 bindings only.** `spec.suspension.rollout` and
  `status.rollout` exist only in `work/v1alpha2` (the storage
  version), like `spec.suspension` itself. The served `v1alpha1`
  version gets no conversion for them, so writing a binding through
  `v1alpha1` drops rollout state just as it already drops
  `spec.suspension`. Karmada's own controllers use `v1alpha2` only.

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Detector and binding controller both write `RB.spec.suspension` | Rollout state lives in a separate field (`Suspension.Rollout`) that the detector never clears (see API changes). |
| Gate reads status left over from the previous template version | The gate only evaluates fresh status (see Status freshness). |
| Unreachable cluster stalls the rollout | Its status is `Unknown` (not passed); `Timeout` bounds the wait; `Pause` protects later clusters. |
| `Healthy` is a weak signal | `RequiredConditions` (e.g. `Available`, or a custom `SmokeTestsPassed`). Karmada observes conditions; it does not run tests. |
| Binding controller restarts mid-rollout | `Compute` is a pure function; state is recovered from `status.rollout` and `spec.suspension.rollout`. |

## Design Details

The PP/CPP declares the rollout (`spec.rolloutStrategy`). Each binding
carries the per-workload state: which clusters are held
(`spec.suspension.rollout`) and progress (`status.rollout`). This
mirrors Deployment (policy) and ReplicaSet (per-instance state).

### API changes

**Policy.** Add `RolloutStrategy` to `PropagationSpec` (shared by PP
and CPP):

```go
// pkg/apis/policy/v1alpha1/propagation_types.go
type PropagationSpec struct {
    // ... existing fields ...
    RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`
}

type RolloutStrategyType string

const (
    RolloutStrategyAllAtOnce RolloutStrategyType = "AllAtOnce" // existing behavior
    RolloutStrategyStaged    RolloutStrategyType = "Staged"
)

type RolloutStrategy struct {
    // +kubebuilder:validation:Enum=AllAtOnce;Staged
    // +kubebuilder:default=AllAtOnce
    Type   RolloutStrategyType `json:"type"`
    Staged *StagedRollout      `json:"staged,omitempty"` // required when Type=Staged
}

// StagedRollout releases one cluster at a time in sorted order.
type StagedRollout struct {
    Gate      *RolloutGate          `json:"gate,omitempty"`      // default {}
    Timeout   *metav1.Duration      `json:"timeout,omitempty"`   // per cluster; default 30m
    OnFailure *RolloutFailurePolicy `json:"onFailure,omitempty"` // default {action: Pause}
}

// RolloutGate must be met by each cluster before the next is released.
type RolloutGate struct {
    RequireHealthy     *bool                  `json:"requireHealthy,omitempty"` // default true
    RequiredConditions []ConditionRequirement `json:"requiredConditions,omitempty"`
    MinSuccessTime     *metav1.Duration       `json:"minSuccessTime,omitempty"` // default 0
}

// ConditionRequirement matches a condition in the workload's
// status.conditions[]. A missing condition counts as Unknown.
type ConditionRequirement struct {
    Type string `json:"type"` // e.g. "Available", "SmokeTestsPassed"
    // +kubebuilder:validation:Enum=True;False;Unknown
    // +kubebuilder:default=True
    Status metav1.ConditionStatus `json:"status,omitempty"`
}

type RolloutFailureAction string

const (
    RolloutFailureActionPause    RolloutFailureAction = "Pause"
    RolloutFailureActionContinue RolloutFailureAction = "Continue"
)

type RolloutFailurePolicy struct {
    // +kubebuilder:validation:Enum=Pause;Continue
    // +kubebuilder:default=Pause
    Action RolloutFailureAction `json:"action"`
}
```

A cluster passes the gate when its status is fresh (see Status
freshness), `RequireHealthy` (if true) holds, and every
`RequiredConditions` entry matches, all continuously for
`MinSuccessTime`. Only condition `type` and `status` are compared, so
heartbeat updates don't reset the clock; any change away from the
required status does. `Timeout` is per cluster: its clock starts when
the cluster becomes current and resets for the next one, so the
worst case is `N × Timeout`.

**Binding spec.** Add `RolloutStrategy` (copied from the policy) and
`Suspension.Rollout` (controller-owned):

```go
// pkg/apis/work/v1alpha2/binding_types.go
type ResourceBindingSpec struct {
    // ... existing fields (Placement, Failover, ConflictResolution, etc.) ...
    RolloutStrategy *policyv1alpha1.RolloutStrategy `json:"rolloutStrategy,omitempty"` // NEW
}

type Suspension struct {
    policyv1alpha1.Suspension `json:",inline"`
    Scheduling *bool              `json:"scheduling,omitempty"`
    Rollout    *RolloutSuspension `json:"rollout,omitempty"` // NEW
}

type RolloutSuspension struct {
    // CurrentCluster is the cluster being gated. Empty when Pending or Succeeded.
    CurrentCluster string `json:"currentCluster,omitempty"`
    // SuspendedClusters is every target that is neither current nor completed.
    SuspendedClusters []string `json:"suspendedClusters,omitempty"`
}
```

- The detector copies `RolloutStrategy` from the PP/CPP the same way
  it copies `Failover` and `ConflictResolution`
  (`BuildResourceBinding`, `BuildClusterResourceBinding`, and the
  binding update path). Binding controllers read it only from the
  binding, so policy edits reach them through the existing RB/CRB
  watch.
- `Rollout` is a sibling of `Scheduling`, outside the embedded
  `policyv1alpha1.Suspension` that the detector rewrites. Today
  `util.MergePolicySuspension` returns `nil` when the policy has no
  suspension and `Scheduling` is nil, which would drop `Rollout`; its
  nil check becomes `Scheduling == nil && Rollout == nil`.
- `shouldSuspendDispatching` (`pkg/controllers/binding/common.go`)
  takes the union of user-declared and rollout suspension.

**Binding status.** Add `Rollout`:

```go
type ResourceBindingStatus struct {
    // ... existing fields ...
    Rollout *RolloutStatus `json:"rollout,omitempty"` // NEW
}

type RolloutStatus struct {
    // ObservedGeneration is the resource template's metadata.generation
    // (not the RB/CRB's) this rollout is driving toward.
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // +kubebuilder:validation:Enum=Pending;Progressing;Succeeded;Failed;Superseded
    Phase RolloutPhase `json:"phase"`

    // CurrentCluster is the cluster being gated; when Failed, the cluster that failed.
    CurrentCluster string `json:"currentCluster,omitempty"`
    // CompletedClusters passed the gate for ObservedGeneration, in order.
    CompletedClusters []string `json:"completedClusters,omitempty"`

    // GateSatisfiedSince is when CurrentCluster began continuously passing
    // the gate; nil on flap. Persisted so MinSuccessTime survives restarts.
    GateSatisfiedSince *metav1.Time `json:"gateSatisfiedSince,omitempty"`
    // CurrentClusterStartedAt starts the current cluster's Timeout clock.
    CurrentClusterStartedAt *metav1.Time `json:"currentClusterStartedAt,omitempty"`
    StartedAt               *metav1.Time `json:"startedAt,omitempty"`
    CompletedAt             *metav1.Time `json:"completedAt,omitempty"`

    // Well-known types: "Progressing", "Healthy", "ConditionsMet".
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    Message    string             `json:"message,omitempty"`
}

type RolloutPhase string

const (
    RolloutPhasePending     RolloutPhase = "Pending"
    RolloutPhaseProgressing RolloutPhase = "Progressing"
    RolloutPhaseSucceeded   RolloutPhase = "Succeeded"
    RolloutPhaseFailed      RolloutPhase = "Failed"
    // Superseded: the template generation changed mid-rollout. The next
    // reconcile clears CompletedClusters and restarts from the first
    // cluster. Distinct from Failed so dashboards can tell them apart.
    RolloutPhaseSuperseded RolloutPhase = "Superseded"
)
```

`ClusterResourceBinding` embeds `ResourceBindingSpec` /
`ResourceBindingStatus`, so all of the above applies to CRB unchanged.

### Interpreter contract for `RequiredConditions`

`RequiredConditions` reads the member workload's
`.status.conditions[]` from `AggregatedStatusItem.Status`, so
`InterpretStatus` must preserve `Conditions`. Native reflectors do for
`Job`, `Ingress`, and CRDs using `reflectWholeStatus`, but drop them
for `Deployment`, `DaemonSet`, `StatefulSet`, and `ReplicaSet`.

Fix: add `Conditions` (with `LastUpdateTime` stripped, to avoid
`Work.Status` churn) to `Wrapped{Deployment,DaemonSet,StatefulSet,ReplicaSet}Status`
and their `reflect*Status` functions. Aggregators read only
replica/generation fields, so the federated resource's
`.status.conditions[]` is unchanged. Custom interpreters
(`ResourceInterpreterCustomization` or webhook) must preserve
`Conditions` themselves; otherwise the cluster fails with
`Reason=ConditionsMissing` after `Timeout`.

The same change also adds `ObservedGeneration` to the StatefulSet
reflector, which today drops it (Deployment, DaemonSet, and ReplicaSet
already copy it). Staged propagation doesn't need this (see Status
freshness), but it fixes an existing bug: StatefulSet aggregation
compares `member.ObservedGeneration >= member.Generation`, so the
federated StatefulSet's `status.observedGeneration` never advances
today.

### Status freshness

`AggregatedStatusItem.Applied` / `Health` / conditions carry no
template revision, so right after a template change they still
describe the previous version. The gate only evaluates a cluster once
its status is fresh.

Only the four replica-based reflectors report generation fields; every
other kind (Jobs, CRDs) goes through `reflectWholeStatus`, which
doesn't. So freshness does not rely on the interpreter. The
work-status controller reads three values directly from the member
object when it reflects status, and records them next to `Status`:

```go
// Added to both workv1alpha1.ManifestStatus and
// workv1alpha2.AggregatedStatusItem (copied by assembleWorkStatus).
ResourceTemplateGeneration int64  `json:"resourceTemplateGeneration,omitempty"` // resourcetemplate.karmada.io/generation annotation
MemberGeneration           int64  `json:"memberGeneration,omitempty"`           // metadata.generation
MemberObservedGeneration   *int64 `json:"memberObservedGeneration,omitempty"`   // status.observedGeneration; nil if unset
```

A cluster is fresh when:

- `ResourceTemplateGeneration >= RolloutStatus.ObservedGeneration`
  (the new template reached this cluster), and
- `MemberObservedGeneration >= MemberGeneration` (the member controller
  processed it), skipped when `MemberObservedGeneration` is nil.

Until then the cluster has not passed and the `MinSuccessTime` clock
does not start. Many operators never set `status.observedGeneration`;
for those kinds only the first check applies, so there is a short
window after apply where status may still describe the old version.
Users should set `MinSuccessTime` (or a `RequiredConditions` entry the
operator only sets for the new version) to cover it.

### Rollout reconciliation

When the binding's `spec.rolloutStrategy.type` is `Staged`, the binding
controller calls

```text
Compute(strategy, prevStatus, aggregatedStatus, targetClusters, templateGeneration, now)
  -> (suspendedClusters, newStatus, requeueAfter)
```

writes `spec.suspension.rollout` and `status.rollout`, and requeues.
`ensureWork` then applies the suspension through
`shouldSuspendDispatching` as today.

- **`targetClusters`** is `mergeTargetClusters(spec.Clusters,
  spec.RequiredBy)`, the same set `ensureWork` dispatches to, sorted
  by name. `CurrentCluster` is the first one not in
  `CompletedClusters`.
- **`templateGeneration`** is the resource template's
  `metadata.generation`, which the binding controller already fetches
  and stamps on each Work as `resourcetemplate.karmada.io/generation`.
  The RB/CRB's own generation is never compared, so the controller's
  own `spec.suspension.rollout` writes don't restart the rollout.
- **Reconcile trigger.** A rollout-aware predicate also wakes the
  binding controller on aggregated-status changes. The execution
  controller needs no change.

![State machine](<staged rollout.png>)

`SuspendedClusters` per phase:

- **Pending** / **Superseded** — every target.
- **Progressing** — every target except `CurrentCluster` and
  `CompletedClusters`.
- **Succeeded** — none.
- **Failed** — every target after `CurrentCluster`;
  `CompletedClusters` stay released.

**Leaving Staged mode.** If the binding's `rolloutStrategy` is nil or `AllAtOnce`, or if the `StagedPropagation` feature gate is disabled while the binding still has a `Staged` strategy, the binding controller clears `spec.suspension.rollout` and `status.rollout` before `ensureWork`, releasing every cluster. Otherwise nothing would clear the leftover suspension.
the leftover suspension.

### Reschedule and failover

The scheduler can change `spec.clusters` at any time, including for
failover; `Compute` simply recomputes the sorted target list each
reconcile.

- **Cluster added.** If it sorts after `CurrentCluster`, it joins the
  suspended tail. If it sorts before and is not completed, it becomes
  `CurrentCluster` and is gated fresh, so no cluster is released
  without passing the gate for the current generation.
- **Cluster removed.** It is dropped from `CompletedClusters`. If it
  was `CurrentCluster`, the rollout moves to the next target without
  failing.
- **All clusters removed.** The rollout returns to `Pending` with
  `CompletedClusters` cleared, like a new binding.

### Failure handling

- **Pause** (default) — set `Phase=Failed`, keep later clusters
  suspended, emit `RolloutClusterFailed`, stop requeuing. To recover,
  either change the template (restarts from the first cluster) or
  remove `rolloutStrategy` (releases everything; see Leaving Staged
  mode).
- **Continue** — mark the cluster completed despite the failure, emit
  a `Warning` event, and move on.

### Feature gate and defaulting

- Feature gate `StagedPropagation` (alpha). When disabled, the webhook
  rejects `rolloutStrategy.type: Staged`; `AllAtOnce` is still
  accepted.
- With `rolloutStrategy` unset or `AllAtOnce`, behavior is unchanged
  and `pkg/rollout/` is not called, apart from clearing leftover
  rollout state.
- Defaults: `Type=AllAtOnce`, `Timeout=30m`, `OnFailure.Action=Pause`,
  `Gate.RequireHealthy=true`, `Gate.MinSuccessTime=0`. An omitted
  `gate` defaults to `{}`, so `gate: nil` and `gate: {}` behave the
  same.

### Corner cases

- **Deletion during rollout** — `spec.suspendDispatching` does not
  block Work deletion; the binding controller clears
  `spec.suspension.rollout` on the deletion reconcile.
- **Overlap with user suspension** — user suspension wins (union). A
  warning is emitted; the policy is not rejected.

### Test Plan

**Unit** (`pkg/rollout.Compute`, webhook, detector, helpers):

- Validation: `type: Staged` requires `staged`; non-empty condition
  `type`; duration bounds.
- Transitions: `Pending → Progressing → Succeeded` in sort order;
  `Progressing → Failed` on `Timeout`; `Superseded → Progressing` on
  template change.
- `RequiredConditions` evaluation, with a missing condition treated as
  `Unknown`.
- Stale status: after a template change, old `Healthy` / condition data
  with a lower `ResourceTemplateGeneration`, or with
  `MemberObservedGeneration < MemberGeneration`, neither passes the
  gate nor starts `MinSuccessTime`; a nil `MemberObservedGeneration`
  skips only the second check.
- Work-status controller records the three generation fields for a
  native kind and for a CRD using `reflectWholeStatus`.
- Reschedule: add / remove / reorder never releases an unverified
  cluster.
- `shouldSuspendDispatching` union.
- Leaving Staged mode from `Failed` or `Progressing` clears rollout
  state and releases every cluster.
- Detector copies `rolloutStrategy` onto RB/CRB on create and on policy
  update.
- `MergePolicySuspension` with a nil policy suspension preserves
  `Rollout` and `Scheduling`.
- Native reflectors round-trip `Conditions` for the four replica-based
  kinds; aggregators do not copy them to the federated resource.
- StatefulSet reflector preserves `ObservedGeneration`, and the
  federated StatefulSet's `status.observedGeneration` advances.

**Integration & E2E:**

- N-cluster Deployment rollout: `Work.spec.suspendDispatching` toggles
  in sort order.
- `OnFailure: Pause` with a stuck-Unhealthy workload: later clusters
  stay suspended.
- Condition-driven promotion (`Available` `False → True`, and Story
  2's `SmokeTestsPassed`).
- CPP with mixed namespaced and cluster-scoped targets.
- Argo CD / Flux sync of the PP does not fight the rollout (we never
  write to the PP).
- Mid-rollout reschedule: a cluster inserted ahead of `CurrentCluster`
  is gated fresh.

## Alternatives

- **Suspension on PP / CPP spec instead of RB / CRB.** Rejected: causes
  GitOps drift (Argo CD / Flux fight the rollout), fans out through the
  detector on every transition, and collides with user suspension in
  the same field.
- **Dedicated `karmada-rollout` controller.** Rejected for v1: its only
  output (`RB.spec.suspension.rollout`) is consumed by the binding
  controller anyway, so it adds an inter-controller hand-off. The API
  doesn't depend on which controller writes it, so extracting one later
  is mechanical.
