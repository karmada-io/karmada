---
title: Exclude terminated workloads from FederatedResourceQuota usage
authors:
- "@CaesarTY"
reviewers:
- "@RainbowMango"
- "@mszacillo"
- TBD
approvers:
- "@RainbowMango"
- TBD

creation-date: 2026-10-13

---

# Exclude terminated workloads from FederatedResourceQuota usage

## Summary

When `FederatedQuotaEnforcement` is enabled, `FederatedResourceQuota` (FRQ) computes usage by
summing the resource requests recorded in the spec of every `ResourceBinding` in the namespace.
It does not know whether the workload behind a binding is still running. For batch workloads
(Job, SparkApplication, Volcano Job, RayJob, PyTorchJob, MPIJob, Argo Workflow, ...), the resource
template usually stays around after the job finishes, so its binding keeps holding quota until
someone deletes the template. In batch-heavy namespaces, finished jobs fill the quota and every
new job is rejected while the member clusters sit idle.

This proposal makes FRQ behave like the Kubernetes native `ResourceQuota`, which stops counting
pods once they reach a terminal phase (`Succeeded`/`Failed`). It has three parts:

1. **A new resource interpreter operation, `InterpretTermination`.** It tells Karmada whether
   an object observed on a member cluster has reached a terminal state. It is available through
   all interpreter layers: built-in (native), third-party, declarative
   (`ResourceInterpreterCustomization`) and webhook.
2. **Record the terminal state on `ResourceBinding`.** The terminal state is evaluated per member
   cluster, next to the existing `health` assessment, and flows
   `Work.status.manifestStatuses[].terminated` → `ResourceBinding.status.aggregatedStatus[].terminated`.
   A summary condition `Terminated` is added to the binding.
3. **FRQ skips terminated workloads.** `helper.CalculateResourceUsage` (shared by the FRQ
   enforcement controller and the ResourceBinding validating webhook) does not count clusters
   whose workload has terminated. When a binding's terminal state changes, the component that
   writes the binding status applies the usage delta to the affected FRQs, the same way the
   ResourceBinding webhook applies deltas for spec changes.

## Motivation

`FederatedResourceQuota` is meant to limit the resources a namespace consumes across member
clusters. For long-running workloads such as Deployments, the requests recorded on a binding are
a good measure of that. For batch workloads they are not. A Job, SparkApplication, Volcano Job,
RayJob or PyTorchJob runs for a limited time and then reaches a terminal state, but its resource
template is usually kept afterwards so users can inspect status, logs and history.

Because FRQ counts every binding regardless of the workload's state, a finished job keeps
holding its quota until its resource template is deleted. In namespaces that run many
short-lived jobs, finished jobs eventually fill the quota and every new job is rejected with
`QuotaExceeded`, even though the member clusters have free capacity. Users are left with two
poor options: delete finished jobs and lose their history, or size the FRQ for all jobs ever
run instead of the jobs currently running, which defeats the purpose of the quota.

Kubernetes solved this for a single cluster long ago: `ResourceQuota` stops counting pods once
they reach a terminal phase (`Succeeded`/`Failed`). Users reasonably expect FRQ to behave the
same way. See [#7931](https://github.com/karmada-io/karmada/issues/7931) for a production report
and a reproduction with a native `batch/v1` Job.

### Why not reuse existing hooks

- **Return zero replicas/components from `GetReplicas`/`GetComponents` once the job finishes.**
  We tried this first. It is fragile because those hooks describe *how an application is
  composed*, and the result should not depend on runtime state (see
  [this comment](https://github.com/karmada-io/karmada/issues/7931#issuecomment-6122561940) by
  @RainbowMango). It also does not work today, because a status-only change on the resource
  template never triggers re-interpretation: the detector filters it out with
  `SpecificationChanged`.
- **Overload `InterpretHealth`.** Health and termination are orthogonal. A failed job is
  terminated but unhealthy. A completed job is terminated and healthy. A running job can be
  either healthy or unhealthy. Health is also used by failover (`application failover` treats
  unhealthy as a trigger), so changing its meaning has side effects.

A dedicated operation keeps existing semantics unchanged and makes "terminated" an explicit
concept that other components (scheduler, estimator, failover) can use later.

### Goals

- Add an `InterpretTermination` interpreter operation that reports whether a member-cluster
  object is in a terminal state. Support it through all interpreter layers.
- Record the terminal state per member cluster on `ResourceBinding` (and `ClusterResourceBinding`,
  since they share `AggregatedStatusItem`), plus a binding-level summary condition.
- Exclude terminated workloads from FRQ `overallUsed`, both in the enforcement controller and
  in the admission webhook. A finished job's quota should be released within seconds, without
  touching the resource template.
- Provide built-in termination rules for common batch kinds: native `batch/v1` `Job` and `v1`
  `Pod`, plus the third-party customizations Karmada already ships for batch CRDs.
- Keep `GetReplicas`/`GetComponents` state-independent, and leave the detector's event
  filtering as it is.

### Non-Goals

- Changing scheduling, rescheduling or failover behavior for terminated bindings. Today, a
  cluster failover can reschedule a finished Job and run it again on the new cluster. Skipping
  failover and eviction for bindings with `Terminated=True` would fix this, and the condition
  defined here is meant to support it, but that change will be a follow-up proposal.
- Accounting for suspended workloads (e.g. `Job.spec.suspend: true`). The same mechanism could
  be extended later.
- Adding FRQ `scopes`/`scopeSelector` (e.g. a `Terminating`/`NotTerminating` scope like native
  `ResourceQuota`). This could be a follow-up enhancement.
- Garbage-collecting finished resource templates.

## Proposal

### User Stories

#### Story 1: Spark batch platform

As a platform owner running hundreds of short-lived `SparkJob`s per day per namespace, I want the
CPU/memory requested by a job to go back to the namespace FRQ as soon as the job reaches `Done`
or `Failed`. That way the next job can be admitted, and I can still keep the finished
`SparkJob` objects for history and debugging.

I add a `terminationInterpretation` to the `ResourceInterpreterCustomization` I already have
for `SparkJob`:

```yaml
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: sparkjob
spec:
  target:
    apiVersion: example.com/v1
    kind: SparkJob
  customizations:
    componentResource:
      luaScript: ... # unchanged, state-independent
    terminationInterpretation:
      luaScript: >
        function InterpretTermination(observedObj)
          if observedObj.status == nil or observedObj.status.state == nil then
            return false
          end
          local state = observedObj.status.state
          return state == 'Done' or state == 'Failed'
        end
```

#### Story 2: Native Kubernetes Jobs

As a user propagating `batch/v1` `Job`s with `FederatedQuotaEnforcement` enabled, I expect a
completed or failed Job to stop counting against the FRQ, the same way its pods stop counting
against the member cluster's `ResourceQuota`. This works without any customization, because
Karmada ships a built-in `InterpretTermination` for `Job`.

#### Story 3: Divided Job across clusters

As a user whose Job is divided across `member1` (3 replicas) and `member2` (2 replicas), I
expect the replicas on `member1` to stop counting once the Job reaches a terminal state there,
even if `member2` is still running. Quota is released per cluster.

### Notes/Constraints/Caveats

- **Terminal state is evaluated on the member-cluster object, not on the resource template.**
  This mirrors `InterpretHealth`. It does not depend on `AggregateStatus` being implemented for
  the kind, it gives per-cluster granularity (Story 3), and it follows a status path that already
  exists (`Work` → `ResourceBinding`). The template and the detector are not involved, so the
  detector's `SpecificationChanged` filter stays as it is.
- **Terminal should mean final, with resources released.** Native `ResourceQuota` can safely ignore
  terminal pods because a pod phase never leaves `Succeeded`/`Failed` and a terminal pod no longer
  runs containers. Interpreter authors should only return `true` for states the workload will not
  leave on its own and in which it no longer holds resources on the member cluster. For example, a `SparkApplication` in `FAILED`
  with an `OnFailure` restart policy will move to `PENDING_RERUN`, so it is not terminal. See
  [Risks and Mitigations](#risks-and-mitigations) for what happens if a workload leaves a
  terminal state anyway.
- **Quota is released per cluster, not per pod.** Native `ResourceQuota` stops counting each pod
  as soon as it terminates. FRQ accounts at the binding level, so the replicas on a cluster are
  released only when the workload as a whole reaches a terminal state on that cluster. For
  example, a Job with 5 replicas on one cluster keeps holding all 5 until the Job is `Complete`
  or `Failed`, even if 4 of its pods have already succeeded.
- **Kinds without the hook are never treated as terminated.** The default is to count usage, so
  existing behavior is unchanged for those kinds.
- **Quota is released only after Karmada observes termination.** In practice this takes a few
  seconds: member status reflection, then Work status, then binding status aggregation, which
  also applies the usage delta to the FRQ.

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **A workload leaves a terminal state** (restart, rerun, or a misbehaving interpreter). Usage goes back up through a *status* update, which the ResourceBinding webhook does not intercept, so FRQ can end up over its limit. | (1) The hook contract states that terminal means final (see [Notes](#notesconstraintscaveats)), and the built-in rules follow it. (2) The binding status aggregation applies the positive usage delta to the FRQ on the transition (see [Applying termination transitions to FRQ usage](#applying-termination-transitions-to-frq-usage)), so `overallUsed` reflects reality and **new** bindings are rejected until usage drops. This is the same behavior as lowering `spec.overall` below current usage, and the same as native `ResourceQuota` when its limit is reduced. (3) The binding status aggregation records a `TerminatedWorkloadReactivated` event on the binding and the FRQ when a cluster's `terminated` flips from `true` to `false`, so the overrun can be observed. Blocking the reactivation is deliberately not done: it would require either rejecting binding status updates, which would break status reporting, or having Karmada suspend the workload, which is intrusive and not supported by every kind. |
| **The member object disappears after termination** (e.g. manual deletion on the member cluster). | No extra handling is needed. When the member object is deleted, `WorkStatusController.syncWorkStatus` gets `NotFound` and returns without touching `Work.status`, so the last reflected `terminated: true` is kept. If Karmada later recreates the object from the Work, the workload really runs again, so counting it again is correct. For native Jobs, `ttlSecondsAfterFinished` is not a concern either, because Karmada prunes it before propagation (`prune.RemoveJobTTLSeconds`). |
| **Terminal does not imply that resources are released.** Some workloads reach a final state while their pods keep running or can still be restarted. Examples: a `RayJob` in `Complete` with `shutdownAfterJobFinishes: false` (the default) keeps its RayCluster running; a Kubeflow job in `Failed` with `cleanPodPolicy: None` can leave worker pods running; a `SparkApplication` with `restartPolicy: Always` is rerun after `COMPLETED`, and `SUBMISSION_FAILED` is retried when `onSubmissionFailureRetries` is set. Releasing quota in these cases would undercount usage. | The hook contract requires `true` only for states that are final **and** no longer hold resources on the member cluster (see [Notes](#notesconstraintscaveats)). The built-in rules check the relevant fields (for example `shutdownAfterJobFinishes`, `cleanPodPolicy`, restart policy and remaining retries) and return `false` when they cannot be sure. Users whose workloads behave differently can override a built-in rule with their own `terminationInterpretation`. |
| **Interpreter errors.** | If `InterpretTermination` fails, `terminated` is left unset, which is treated as not terminated (the conservative choice: keep counting), and an `InterpretTerminationFailed` event is recorded on the Work, matching how `InterpretHealth` failures are handled. |
| **Spoofing through the main resource.** A client could try to send a fake `status.aggregatedStatus[].terminated` together with a spec update so that the webhook sees lower usage. | `ResourceBinding` has the status subresource enabled, so the apiserver resets `status` to the stored value before admission on main-resource updates. Only components with `resourcebindings/status` RBAC (karmada-controller-manager) can set it. |

## Design Details

### Overview

```
member cluster object (e.g. Job, SparkApplication)
        │  status change observed by member informer
        ▼
WorkStatusController.reflectStatus()
        ├─ ReflectStatus()          (existing)
        ├─ InterpretHealth()        (existing)
        └─ InterpretTermination()   (NEW)  ──► Work.status.manifestStatuses[].terminated
                                                    │
                                                    ▼  Work status changed
                                   RB/CRB status controller: assembleWorkStatus()
                                                    │
                                                    ▼
                         RB.status.aggregatedStatus[].terminated  (NEW)
                         RB.status.conditions[type=Terminated]    (NEW)
                                                    │
                                                    ▼  scheduled terminated set changed
                                   delta = CalculateResourceUsage(new) - CalculateResourceUsage(old)
                                   (skips terminated clusters)  (CHANGED)
                                                    │
                                                    ▼
                                   FRQ.status.overallUsed += delta → webhook admits new RBs
```

### API changes

The new concept is called *termination* and is modeled as an optional bool, following the
`InterpretHealth` / `healthInterpretation` / `health` pattern:

- "Terminated" covers both success and failure. A failed job must release quota as well, so a
  name like "completed" would be misleading. It also matches Kubernetes' own wording for pods in
  `Succeeded`/`Failed` ("terminal phase").
- A bool rather than a `phase` enum keeps the contract simple: terminal means final. States a
  workload can come back from, such as suspension, do not fit that contract and should be
  modeled separately if they are ever needed.
- The status fields are `*bool`, so "not evaluated" (no hook for the kind, or the interpreter
  failed) can be told apart from "evaluated as not terminated". Both are counted toward quota,
  but only the latter means the kind supports termination (see the `Terminated` condition below).

#### 1. New interpreter operation

`pkg/apis/config/v1alpha1/resourceinterpreterwebhook_types.go`:

```go
const (
	// ...existing operations...

	// InterpreterOperationInterpretTermination indicates that karmada wants to figure out
	// whether a specific object has reached a terminal state, i.e. it has finished running
	// (successfully or not) and will not resume on its own.
	// Resources of a terminated object are not counted toward FederatedResourceQuota usage.
	InterpreterOperationInterpretTermination InterpreterOperation = "InterpretTermination"
)
```

`pkg/apis/config/v1alpha1/interpretercontext_types.go`, `ResourceInterpreterResponse`:

```go
	// Terminated represents whether the referencing object has reached a terminal state.
	// Required if InterpreterOperation is InterpreterOperationInterpretTermination.
	// +optional
	Terminated *bool `json:"terminated,omitempty"`
```

#### 2. Declarative customization

`pkg/apis/config/v1alpha1/resourceinterpretercustomization_types.go`:

```go
type CustomizationRules struct {
	// ...existing fields...

	// TerminationInterpretation describes the rules by which Karmada can tell whether
	// the resource has reached a terminal state. Resources of a terminated object are
	// not counted toward FederatedResourceQuota usage.
	// Karmada provides built-in rules for several standard Kubernetes types, see:
	// https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/#interprettermination
	// If TerminationInterpretation is set, the built-in rules will be ignored.
	// +optional
	TerminationInterpretation *TerminationInterpretation `json:"terminationInterpretation,omitempty"`
}

// TerminationInterpretation holds the rules for interpreting whether a specific resource
// has reached a terminal state.
type TerminationInterpretation struct {
	// LuaScript holds the Lua script that is used to tell whether a specific resource
	// has reached a terminal state.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function InterpretTermination(observedObj)
	//           if observedObj.status == nil then
	//               return false
	//           end
	//           return observedObj.status.phase == 'Succeeded' or observedObj.status.phase == 'Failed'
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - observedObj: the object represents the configuration that is observed
	//       from a specific member cluster.
	//
	// The returned boolean value indicates whether the object has terminated. A terminal
	// state should be final: the object is not expected to leave it on its own, and it no
	// longer holds resources on the member cluster.
	//
	// +required
	LuaScript string `json:"luaScript"`
}
```

#### 3. Work and ResourceBinding status

`pkg/apis/work/v1alpha1/work_types.go`, `ManifestStatus`:

```go
	// Terminated represents whether the current resource has reached a terminal state,
	// as assessed by the InterpretTermination interpreter operation.
	// Nil means the state was not evaluated, either because the resource kind has no
	// InterpretTermination hook or because the interpretation failed.
	// +optional
	Terminated *bool `json:"terminated,omitempty"`
```

`pkg/apis/work/v1alpha2/binding_types.go`, `AggregatedStatusItem`:

```go
	// Terminated represents whether the resource running in this member cluster has
	// reached a terminal state. Resources of terminated clusters are not counted toward
	// FederatedResourceQuota usage. Nil means the state was not evaluated and is treated
	// as not terminated.
	// +optional
	Terminated *bool `json:"terminated,omitempty"`
```

A new binding condition type:

```go
const (
	// ...Scheduled, FullyApplied...

	// Terminated represents the condition that the resource referenced by the
	// ResourceBinding or ClusterResourceBinding has reached a terminal state on all
	// scheduled clusters.
	Terminated string = "Terminated"
)

// These are reasons for a binding's transition to a Terminated condition.
const (
	// TerminatedReasonAllClusters means the resource has terminated on every scheduled cluster.
	TerminatedReasonAllClusters = "AllClustersTerminated"
	// TerminatedReasonPartialClusters means the resource has terminated on some but not all
	// scheduled clusters.
	TerminatedReasonPartialClusters = "PartialClustersTerminated"
	// TerminatedReasonRunning means the resource has not terminated on any scheduled cluster.
	TerminatedReasonRunning = "NotTerminated"
)
```

The condition is only set when at least one scheduled cluster has a non-nil
`aggregatedStatus[].terminated`, i.e. when the kind has an `InterpretTermination` hook. Bindings
of other kinds do not get a meaningless `Terminated=False`, and the binding status aggregation
does not need access to the resource interpreter to decide this. The per-cluster
`aggregatedStatus[].terminated` is the source of truth for quota accounting. The condition is a
summary for users and for other components. It is exposed as a new `TERMINATED` print column on
both `ResourceBinding` and `ClusterResourceBinding`, next to the existing `SCHEDULED` and
`FULLYAPPLIED` columns:

```go
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Terminated")].status`,name="TERMINATED",type=string
```

`Terminated=True` has a fixed meaning: the workload has reached a final state on every
scheduled cluster. Future consumers can rely on that. For example, failover could skip such
bindings instead of rerunning a finished job on another cluster (see [Non-Goals](#non-goals)).
The scheduler estimator does not need this signal, because a finished job's pods are already
gone from member cluster capacity.

Example:

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: pi-job
  namespace: tenant-a
spec:
  clusters:
  - name: member1
    replicas: 3
  - name: member2
    replicas: 2
  replicaRequirements:
    resourceRequest:
      cpu: "1"
status:
  aggregatedStatus:
  - clusterName: member1
    applied: true
    health: Healthy
    terminated: true          # NEW
    status: {succeeded: 3, ...}
  - clusterName: member2
    applied: true
    health: Healthy
    terminated: false         # NEW
    status: {active: 2, ...}
  conditions:
  - type: Terminated          # NEW
    status: "False"
    reason: PartialClustersTerminated
    message: "Resource terminated on 1/2 clusters: member1"
```

FRQ usage for this binding: `2 × 1 CPU = 2 CPU` (member1's 3 replicas are excluded).

### Resource interpreter changes

`pkg/resourceinterpreter/interpreter.go`:

```go
type ResourceInterpreter interface {
	// ...existing methods...

	// InterpretTermination returns whether the object has reached a terminal state.
	InterpretTermination(object *unstructured.Unstructured) (terminated bool, err error)
}
```

The usual precedence applies: declarative (`ResourceInterpreterCustomization`) → webhook →
third-party → native. The work needed in each layer:

| Layer | Change |
|---|---|
| Native (`pkg/resourceinterpreter/default/native`) | New `termination.go` with `getAllDefaultTerminationInterpreter()`: `batch/v1 Job` (condition `Complete` or `Failed` is `True`), `v1 Pod` (`status.phase` is `Succeeded` or `Failed`). |
| Third-party (`pkg/resourceinterpreter/default/thirdparty/resourcecustomizations`) | Add `terminationInterpretation` plus test cases to the batch customizations listed below. |
| Declarative (`customized/declarative`, `luavm`) | Load and run `InterpretTermination(observedObj)`. Validate in the RIC validating webhook. |
| Webhook (`customized/webhook`) | Support the new operation in `RuleWithOperations`. Require `response.terminated` for this operation in response verification. |
| `karmadactl interpret` (`pkg/util/interpreter/rule.go`) | Add `terminationInterpretationRule` so users can test their scripts locally. |

Built-in rules proposed for the shipped third-party customizations:

| Kind | Terminal when |
|---|---|
| `batch.volcano.sh/v1alpha1 Job` | `status.state.phase` ∈ {`Completed`, `Failed`, `Terminated`} (not `Aborted`, which can be resumed) |
| `sparkoperator.k8s.io/v1beta2 SparkApplication` | `status.applicationState.state` ∈ {`COMPLETED`, `SUBMISSION_FAILED`}, or `FAILED` when `spec.restartPolicy.type` is `Never` / retries are exhausted |
| `kubeflow.org/v1 PyTorchJob`, `TFJob`; `kubeflow.org/v2beta1 MPIJob` | condition `Succeeded` or `Failed` is `True` |
| `ray.io/v1 RayJob` | `status.jobDeploymentStatus` ∈ {`Complete`, `Failed`} |
| `argoproj.io/v1alpha1 Workflow` | `status.phase` ∈ {`Succeeded`, `Failed`, `Error`} |
| `apps.kruise.io/v1alpha1 BroadcastJob` | `status.phase` ∈ {`completed`, `failed`} |

Long-running kinds (Deployment, StatefulSet, FlinkDeployment, RayCluster, ...) get no termination
rule and are never treated as terminated.

### Status collection changes

`pkg/controllers/status/work_status_controller.go`: in `reflectStatus`, after `interpretHealth`:

```go
func (c *WorkStatusController) interpretTermination(clusterObj *unstructured.Unstructured, work *workv1alpha1.Work) *bool {
	if !c.ResourceInterpreter.HookEnabled(clusterObj.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretTermination) {
		return nil
	}
	terminated, err := c.ResourceInterpreter.InterpretTermination(clusterObj)
	if err != nil {
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonInterpretTerminationFailed, ...)
		return nil
	}
	return &terminated
}
```

and set `ManifestStatus.Terminated`.

`pkg/util/helper/workstatus.go`:

- `assembleWorkStatus` copies `ManifestStatus.Terminated` into `AggregatedStatusItem.Terminated`.
  It stays nil for items that are not applied.
- `AggregateResourceBindingWorkStatus` / `AggregateClusterResourceBindingWorkStatus` set the
  `Terminated` condition (computed against `spec.clusters`) next to `FullyApplied`, when at least
  one scheduled cluster has a non-nil `terminated`.
- `AggregateResourceBindingWorkStatus` also applies the FRQ usage delta and records the
  `TerminatedWorkloadReactivated` event when the scheduled terminated cluster set changes (see
  [Applying termination transitions to FRQ usage](#applying-termination-transitions-to-frq-usage)).
  `ClusterResourceBinding` is not counted by FRQ, so the cluster-scoped path does not do this.

Because these are status-only writes, they do not go through the ResourceBinding validating
webhook (it only matches the `resourcebindings` resource, not `resourcebindings/status`), and
they do not affect the detector.

### FederatedResourceQuota changes

#### Usage calculation

`helper.CalculateResourceUsage` is the single place both the controller and the webhook use to
compute a binding's usage. It changes to skip terminated clusters:

```go
// terminatedClusters returns the set of scheduled clusters on which the workload has
// reached a terminal state.
func terminatedClusters(rb *workv1alpha2.ResourceBinding) sets.Set[string] {
	result := sets.New[string]()
	for _, item := range rb.Status.AggregatedStatus {
		if ptr.Deref(item.Terminated, false) {
			result.Insert(item.ClusterName)
		}
	}
	return result
}

func CalculateResourceUsage(rb *workv1alpha2.ResourceBinding) corev1.ResourceList {
	...
	terminated := terminatedClusters(rb)

	// Components path: count only clusters that have not terminated.
	clusterCount := int64(0)
	for _, c := range rb.Spec.Clusters {
		if !terminated.Has(c.Name) {
			clusterCount++
		}
	}

	// ReplicaRequirements path: sum replicas only from clusters that have not terminated.
	for _, c := range rb.Spec.Clusters {
		if !terminated.Has(c.Name) {
			totalReplicas += c.Replicas
		}
	}
	...
}
```

Notes:

- The calculation is driven by `spec.clusters`. A stale `aggregatedStatus` item for a cluster
  that is no longer scheduled has no effect. A newly scheduled cluster has no item yet, so it is
  counted. This is the conservative choice.
- On the webhook path, `newRB.status` equals `oldRB.status` for main-resource updates (the status
  subresource is enabled), so the delta calculation stays consistent. Example: a spec update
  that does not change replicas on a terminated binding produces a zero delta, as it does today.
- On create, a binding has no aggregated status, so it is counted in full. Nothing changes here.

#### Applying termination transitions to FRQ usage

`FRQ.status.overallUsed` already has two writers:

- The ResourceBinding webhook adds `CalculateResourceUsage(newRB) - CalculateResourceUsage(oldRB)`
  to every matching FRQ at admission time (`updateFRQStatusesWithRetrySignal` in
  `pkg/webhook/resourcebinding/validating.go`).
- The FRQ enforcement controller overwrites it with a full recomputation over the bindings in its
  informer cache (`collectQuotaStatus`). It does this on FRQ changes, binding deletion and the
  periodic resync.

Termination changes a binding's usage through a status update, which the webhook does not see.
Triggering a full recomputation for each transition would be the simplest fix, but it races with
the webhook. If the controller recomputes after the webhook admitted a new binding and before
that binding reaches the controller's cache, the new binding's usage is dropped from
`overallUsed` and more bindings than the quota allows can be admitted. In batch namespaces, jobs
finish while new ones are being submitted, so this would happen often.

Instead, termination transitions are applied as **deltas**, the same way the webhook applies
spec changes:

1. `AggregateResourceBindingWorkStatus` computes the new `aggregatedStatus`. Inside the
   conflict-retry update function, it compares the scheduled terminated cluster set of the
   binding being replaced with that of the new status:

   ```go
   // scheduledTerminatedClusters returns the clusters in spec.clusters whose
   // aggregatedStatus item has terminated set to true.
   func scheduledTerminatedClusters(rb *workv1alpha2.ResourceBinding) sets.Set[string]
   ```

2. If the set changed and `FederatedQuotaEnforcement` is enabled, it computes
   `delta = CalculateResourceUsage(new) - CalculateResourceUsage(old)`, where `old` and `new`
   share the same spec and differ only in status.
3. It applies the delta to every FRQ in the namespace through a helper shared with the webhook,
   e.g. `helper.ApplyFederatedResourceQuotaDelta(ctx, client, namespace, delta)`. The helper
   filters the delta by each FRQ's `spec.overall` keys, adds it to `overallUsed` and updates the
   FRQ status with optimistic concurrency, retrying on conflict. Unlike the webhook, it never
   denies: the status change has already happened on the member cluster.

The order of the two writes is chosen so that a failure between them overcounts rather than
undercounts:

- **Release** (a cluster becomes terminated, negative delta): the binding status is written
  first, then the FRQ. If the FRQ write fails, usage stays too high until the next periodic
  resync.
- **Reactivation** (a cluster leaves the terminated state, positive delta): the FRQ is written
  first, then the binding status. If the binding status write fails and is retried, the delta can
  be applied twice. Usage is then too high until the next periodic resync. In this case the
  aggregation also records a `TerminatedWorkloadReactivated` warning event on the binding and
  the FRQ (see [Risks and Mitigations](#risks-and-mitigations)).

The event is emitted from the aggregation and not from a controller predicate. There the
per-cluster `true` → `false` flip is visible directly, so a terminated cluster that is simply
removed from `spec.clusters` by rescheduling is not reported as a reactivation.

`resourceBindingPredicate` in
`pkg/controllers/federatedresourcequota/federated_resource_quota_enforcement_controller.go` is
unchanged and keeps ignoring binding updates. `collectQuotaStatus` already uses
`CalculateResourceUsage`, so full recomputations exclude terminated clusters without further
changes. The periodic resync (`--federated-resource-quota-sync-period`) remains as a safety net
for missed or failed delta updates.

#### Webhook

`pkg/webhook/resourcebinding/validating.go` needs no logic changes beyond the shared
`CalculateResourceUsage`. Because the binding status aggregation lowers FRQ `status.overallUsed`
as soon as termination is observed, the next binding admission sees the freed quota. Its
FRQ status update logic is factored into the shared delta helper described above.

### Feature gate

No new feature gate is proposed:

- `FederatedQuotaEnforcement` is Alpha and off by default, and @RainbowMango
  [agreed](https://github.com/karmada-io/karmada/issues/7931#issuecomment-6122561940) that FRQ should ignore finished
  workloads by default. Making this the behavior of `FederatedQuotaEnforcement` is consistent
  with native `ResourceQuota`.
- `InterpretTermination` and the new status fields only record information, so they are not
  gated. With `FederatedQuotaEnforcement` off, nothing consumes them.
- A separate gate for the quota filtering alone would only add a combination to test, with no
  user-facing benefit while the parent feature is still Alpha.

### Compatibility

- **API.** All new fields are optional and additive. Old controllers ignore them, and new
  controllers treat a missing field as "not terminated".
- **Webhook interpreters.** Existing `ResourceInterpreterWebhookConfiguration`s that match
  `"*"` operations will start receiving `InterpretTermination` requests. Webhooks that do not
  recognize the operation should return an error or omit `terminated`. Either way the object is
  treated as not terminated and an event is recorded, so behavior is unchanged.
  Webhook authors can opt in by adding the operation to their rules and returning `terminated`.
- **Upgrade.** After upgrade, existing finished workloads are picked up the next time their
  member status is reflected. For a workload that has already finished, its member object
  usually no longer changes, so in practice this happens when karmada-controller-manager
  restarts: the member informers re-list and `WorkStatusController.onAdd` enqueues every object.
  Informer resyncs do not help, because `WorkStatusController.onUpdate` drops events whose old and
  new objects are equal. Upgrading karmada-controller-manager restarts it anyway, so finished
  workloads are released shortly after the upgrade.

### Test Plan

- **Unit tests**
  - Native `InterpretTermination` for `Job` (complete, failed, running, suspended) and `Pod` (all
    phases).
  - Third-party customization test data for each kind in the table above, using the existing
    `customizations_tests.yaml` framework.
  - Declarative Lua VM and webhook interpreter for the new operation, including response
    verification.
  - `assembleWorkStatus` / condition computation: all, partial, none, unapplied clusters, and
    no hook (nil `terminated`, no condition).
  - `CalculateResourceUsage`: replica path and components path, partial termination, stale
    items for unscheduled clusters, no hook.
  - FRQ delta on status aggregation: release and reactivation deltas, no delta when only
    `spec.clusters` drops a terminated cluster, write ordering on failure, no delta when
    `FederatedQuotaEnforcement` is disabled, `TerminatedWorkloadReactivated` event on
    `true` → `false` only.
  - RB webhook: delta with terminated clusters on spec update, create.
- **E2E tests** (`test/e2e/suites/base/federatedresourcequota_test.go`)
  - The reproduction from #7931: FRQ `cpu: 2`, run a 1-CPU Job to completion, verify
    `overallUsed.cpu` drops to `0`, then admit two more Jobs.
  - Divided Job across two clusters, one finishing first: verify partial release.
  - Submit new Jobs while others finish: verify `overallUsed` matches the full recomputation
    and never exceeds `spec.overall`.
  - Declarative customization for a CRD with `terminationInterpretation`.
  - A Deployment is never excluded.
- **Production validation.** We will validate with Spark batch workloads in our QA and
  production control planes.
