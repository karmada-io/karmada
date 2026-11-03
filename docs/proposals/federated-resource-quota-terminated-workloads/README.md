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

When `FederatedQuotaEnforcement` is enabled, `FederatedResourceQuota` (FRQ) computes usage by summing the resource requests recorded in the spec of every `ResourceBinding` in the namespace. It does not know whether the workload behind a binding is still running. For batch workloads (Job, SparkApplication, Volcano Job, PyTorchJob, MPIJob, RayJob, Argo Workflow, ...), the resource template usually stays around after the job finishes, so its binding keeps holding quota until someone deletes the template. In batch-heavy namespaces, finished jobs fill the quota and every new job is rejected while the member clusters sit idle.

This proposal makes FRQ behave like the Kubernetes native `ResourceQuota`, which stops counting pods once they reach a terminal phase (`Succeeded`/`Failed`). It has three parts:

1. **A new resource interpreter operation, `InterpretTermination`.** It tells Karmada whether an object observed on a member cluster has reached a terminal state. It is available through all interpreter layers: built-in, declarative (`ResourceInterpreterCustomization`) and webhook.
2. **Record the terminal state on `ResourceBinding`.** The terminal state is evaluated per member cluster, next to the existing `health` assessment, and flows `Work.status.manifestStatuses[].terminated` → `ResourceBinding.status.aggregatedStatus[].terminated`.
3. **FRQ skips terminated workloads.** `helper.CalculateResourceUsage` (shared by the FRQ enforcement controller and the ResourceBinding validating webhook) does not count clusters whose workload has terminated. When a binding's terminal state changes, the FRQ enforcement controller recomputes usage.

## Motivation

`FederatedResourceQuota` is meant to limit the resources a namespace consumes across member clusters. For long-running workloads such as Deployments, the requests recorded on a binding are a good measure of that. For batch workloads they are not. A Job, SparkApplication, Volcano Job, RayJob or PyTorchJob runs for a limited time and then reaches a terminal state, but its resource template is usually kept afterwards so users can inspect status, logs and history.

Because FRQ counts every binding regardless of the workload's state, a finished job keeps holding its quota until its resource template is deleted. In namespaces that run many short-lived jobs, finished jobs eventually fill the quota and every new job is rejected with `QuotaExceeded`, even though the member clusters have free capacity. Users are left with two poor options: delete finished jobs and lose their history, or size the FRQ for all jobs ever run instead of the jobs currently running, which defeats the purpose of the quota.

Kubernetes solved this for a single cluster long ago: `ResourceQuota` stops counting pods once they reach a terminal phase (`Succeeded`/`Failed`). Users reasonably expect FRQ to behave the same way. See [#7931](https://github.com/karmada-io/karmada/issues/7931) for a production report and a reproduction with a native `batch/v1` Job.

### Why not reuse existing hooks

- **Return zero replicas/components from `GetReplicas`/`GetComponents` once the job finishes.** We tried this first. It is fragile because those hooks describe *how an application is composed*, and the result should not depend on runtime state (see [this comment](https://github.com/karmada-io/karmada/issues/7931#issuecomment-6122561940) by @RainbowMango). It also does not work today, because a status-only change on the resource template never triggers re-interpretation: the detector filters it out with `SpecificationChanged`.
- **Overload `InterpretHealth`.** Health and termination are orthogonal. A failed job is terminated but unhealthy. A completed job is terminated and healthy. A running job can be either healthy or unhealthy. Health is also used by failover (`application failover` treats unhealthy as a trigger), so changing its meaning has side effects.

A dedicated operation keeps existing semantics unchanged and makes "terminated" an explicit concept that other components (such as scheduling and failover) can use later.

### Goals

- Add an `InterpretTermination` interpreter operation that reports whether a member-cluster object is in a terminal state. Support it through all interpreter layers.
- Record the terminal state per member cluster on `ResourceBinding` (and `ClusterResourceBinding`, since they share `AggregatedStatusItem`).
- Exclude terminated workloads from FRQ `overallUsed`, both in the enforcement controller and in the admission webhook. A finished job's quota should be released within seconds, without touching the resource template.
- Provide conservative built-in termination rules for batch kinds Karmada already supports: native `batch/v1` `Job` and `v1` `Pod`, plus reference rules for the third-party customizations Karmada already ships for batch CRDs, finalized per kind in their implementation PRs (see [Built-in termination rules](#built-in-termination-rules)).
- Keep `GetReplicas`/`GetComponents` state-independent, and leave the detector's event filtering as it is.

### Non-Goals

- Changing scheduling, rescheduling or failover behavior for terminated bindings. Today, a cluster failover can reschedule a finished Job and run it again on the new cluster. Skipping failover and eviction for bindings whose workload has terminated on every scheduled cluster would fix this. The per-cluster terminal state recorded here makes that possible, but the change, and any binding-level condition it needs, will be a follow-up proposal.
- Releasing quota for suspended workloads (e.g. `Job.spec.suspend: true`). A suspended workload has no running pods, but it can be resumed, so it is not terminated and keeps counting. Resuming is a spec change on the resource template that does not change any quota-relevant field of the binding, so excluding suspended workloads would let them resume without a quota check. Supporting this needs resuming to be gated on quota, which is left to a future proposal.
- Adding FRQ `scopes`/`scopeSelector` (e.g. a `Terminating`/`NotTerminating` scope like native `ResourceQuota`). This could be a follow-up enhancement.
- Garbage-collecting finished resource templates.

## Proposal

### User Stories

#### Story 1: Spark batch platform

As a platform owner running hundreds of short-lived `SparkJob`s per day per namespace, I want the CPU/memory requested by a job to go back to the namespace FRQ as soon as the job reaches `Done` or `Failed`. That way the next job can be admitted, and I can still keep the finished `SparkJob` objects for history and debugging.

I add a `terminationInterpretation` to the `ResourceInterpreterCustomization` I already have for `SparkJob`:

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

As a user propagating `batch/v1` `Job`s with `FederatedQuotaEnforcement` enabled, I expect a completed or failed Job to stop counting against the FRQ, the same way its pods stop counting against the member cluster's `ResourceQuota`. This works without any customization, because Karmada ships a built-in `InterpretTermination` for `Job`.

#### Story 3: Divided Job across clusters

As a user whose Job is divided across `member1` (3 replicas) and `member2` (2 replicas), I expect the replicas on `member1` to stop counting once the Job reaches a terminal state there, even if `member2` is still running. Quota is released per cluster.

### Notes/Constraints/Caveats

- **Terminal state is evaluated on the member-cluster object, not on the resource template.** This mirrors `InterpretHealth`. It does not depend on `AggregateStatus` being implemented for the kind, it gives per-cluster granularity (Story 3), and it follows a status path that already exists (`Work` → `ResourceBinding`). The template and the detector are not involved, so the detector's `SpecificationChanged` filter stays as it is.
- **Terminal should mean final, with resources released.** Native `ResourceQuota` can safely ignore terminal pods because a pod phase never leaves `Succeeded`/`Failed` and a terminal pod no longer runs containers. Interpreter authors should only return `true` for states the workload will not leave on its own and in which it no longer holds resources on the member cluster. For example, a Kubeflow `PyTorchJob` with the default `cleanPodPolicy` (`None`) can reach `Succeeded` while workers that have not finished keep running, so it is not treated as terminated. See [Risks and Mitigations](#risks-and-mitigations) for what happens if a workload leaves a terminal state anyway.
- **Built-in rules return `false` when unsure.** A built-in rule applies to every user of a kind, and a wrong `true` undercounts usage without anyone noticing, while a wrong `false` only keeps counting. Built-in rules therefore return `true` only when the observed object proves the workload is final and its pods are gone. Users whose workloads need a different trade-off override the built-in rule with their own `terminationInterpretation`, which takes precedence.
- **Quota is released per cluster, not per pod.** Native `ResourceQuota` stops counting each pod as soon as it terminates. FRQ accounts at the binding level, so the replicas on a cluster are released only when the workload as a whole reaches a terminal state on that cluster. For example, a Job with 5 replicas on one cluster keeps holding all 5 until the Job is `Complete` or `Failed`, even if 4 of its pods have already succeeded.
- **Kinds without a rule are never treated as terminated.** Karmada ships built-in rules only for kinds it already supports (see [Built-in termination rules](#built-in-termination-rules)). Workloads of any other kind, including custom CRDs, keep counting toward quota unless the user provides a `terminationInterpretation`. Existing behavior is unchanged for those kinds.
- **A terminal status can be reported shortly before all pods are gone.** Several operators write the final status first and delete the remaining pods afterwards, and pod deletion honors the termination grace period. Where the observed object exposes pod counts, the built-in rule checks them (for example Volcano `Job`). Otherwise quota can be released up to about one grace period early. This is accepted as a short, bounded undercount.
- **Quota is released only after Karmada observes termination.** In practice this takes a few seconds: member status reflection, then Work status, then binding status aggregation, then FRQ recomputation.

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **A workload leaves a terminal state.** For the built-in kinds, this only happens through an explicit user action. For example, re-applying a changed spec to a finished `SparkApplication`, or running `argo retry` on a failed Argo `Workflow`, runs the workload again. In both cases the usage comes back through a status update, which the ResourceBinding webhook does not check against the quota. | Rules only treat as terminated a state the operator never leaves on its own (see [Notes](#notesconstraintscaveats)). If a user action still restarts the workload, its `terminated` flips back to `false` and the FRQ controller recomputes usage (see [Recomputing usage on termination changes](#recomputing-usage-on-termination-changes)), so the workload counts again. A `TerminatedWorkloadReactivated` event is recorded on the binding. **Limitation:** the restarted workload itself is not checked against the quota and keeps running, so the FRQ can be over its limit until usage drops. However, any new binding is rejected until then. |
| **The member object disappears after termination** (e.g. manual deletion on the member cluster). | No extra handling is needed. When the member object is deleted, `WorkStatusController.syncWorkStatus` gets `NotFound` and returns without touching `Work.status`, so the last reflected `terminated: true` is kept. If Karmada later recreates the object from the Work, the workload really runs again, so counting it again is correct. For native Jobs, `ttlSecondsAfterFinished` is not a concern either, because Karmada prunes it before propagation (`prune.RemoveJobTTLSeconds`). |
| **Terminal does not imply that resources are released.** Some workloads reach a final state while their pods keep running or can still be restarted. Examples: a `RayJob` in `Complete` with `shutdownAfterJobFinishes: false` (the default) keeps its RayCluster running; a Kubeflow job or `MPIJob` in `Succeeded` or `Failed` with the default `cleanPodPolicy` (`None`) can leave worker pods running. Releasing quota in these cases would undercount usage. | The hook contract requires `true` only for states that are final **and** no longer hold resources on the member cluster (see [Notes](#notesconstraintscaveats)). The built-in rules check the relevant fields (for example `cleanPodPolicy`) and return `false` when they cannot be sure (see [Built-in termination rules](#built-in-termination-rules)). Users whose workloads behave differently can override a built-in rule with their own `terminationInterpretation`. |
| **Interpreter errors.** | If `InterpretTermination` fails, `terminated` is left unset, which is treated as not terminated (the conservative choice: keep counting), and an `InterpretTerminationFailed` event is recorded on the Work, matching how `InterpretHealth` failures are handled. The object is requeued with backoff so interpretation is retried (see [Status collection changes](#status-collection-changes)). |
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
                                                    │
                                                    ▼  scheduled terminated set changed
                                   FRQ enforcement controller: collectQuotaStatus()
                                   sum of CalculateResourceUsage(rb), skipping terminated clusters  (CHANGED)
                                                    │
                                                    ▼
                                   FRQ.status.overallUsed no longer includes the finished job → webhook admits new RBs
```

### API changes

The new concept is called *termination* and is modeled as an optional bool, following the `InterpretHealth` / `healthInterpretation` / `health` pattern:

- "Terminated" covers both success and failure. A failed job must release quota as well, so a name like "completed" would be misleading. It also matches Kubernetes' own wording for pods in `Succeeded`/`Failed` ("terminal phase").
- A bool rather than a `phase` enum keeps the contract simple: terminal means final. States a workload can come back from, such as suspension, do not fit that contract and should be modeled separately if they are ever needed.
- The status fields are `*bool`, so "not evaluated" (no hook for the kind, or the interpreter failed) can be told apart from "evaluated as not terminated". Both are counted toward quota, but only the latter tells users that the kind supports termination.

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

The usual precedence applies: declarative (`ResourceInterpreterCustomization`) → webhook → third-party → native. The work needed in each layer:

| Layer | Change |
|---|---|
| Native (`pkg/resourceinterpreter/default/native`) | New `termination.go` with `getAllDefaultTerminationInterpreter()`: `batch/v1 Job` (condition `Complete` or `Failed` is `True`), `v1 Pod` (`status.phase` is `Succeeded` or `Failed`). |
| Third-party (`pkg/resourceinterpreter/default/thirdparty/resourcecustomizations`) | Add `terminationInterpretation` plus test cases to the batch customizations listed in [Built-in termination rules](#built-in-termination-rules). |
| Declarative (`customized/declarative`, `luavm`) | Load and run `InterpretTermination(observedObj)`. Validate in the RIC validating webhook. |
| Webhook (`customized/webhook`) | Support the new operation in `RuleWithOperations`. Require `response.terminated` for this operation in response verification. |
| `karmadactl interpret` (`pkg/util/interpreter/rule.go`) | Add `terminationInterpretationRule` so users can test their scripts locally. |

#### Built-in termination rules

Built-in rules follow the "return `false` when unsure" principle (see [Notes](#notesconstraintscaveats)).

The native rules (`Job`, `Pod`) are part of this proposal. The third-party rules are reference rules: each was checked against the source of the operator version listed and shows that the kind can be supported under the contract above. The final rule for each kind is reviewed in its implementation PR together with its test data, and may be refined there, for example for newer operator versions. Whatever is refined, a rule must keep returning `false` when it cannot be sure.

| Kind | Terminated when | Notes |
|---|---|---|
| `batch/v1 Job` (native) | condition `Complete` or `Failed` is `True` | Both are final. A suspended Job is not terminated. |
| `v1 Pod` (native) | `status.phase` is `Succeeded` or `Failed` | Same as native `ResourceQuota`. |
| `batch.volcano.sh/v1alpha1 Job` (Volcano v1.16.0) | `status.state.phase` is `Completed`, `Failed` or `Terminated`, and `status.pending`, `status.running`, `status.terminating` and `status.unknown` are all 0 | `Aborted` can be resumed, so it is not terminated. `Completed` can be set once `minSuccess` is reached while remaining pods are still being killed, hence the pod counts. Pods in phase `Unknown` may still be running on a node that lost contact. |
| `sparkoperator.k8s.io/v1beta2 SparkApplication` (spark-operator v2.5.2) | `status.applicationState.state` is `COMPLETED` or `FAILED`, and no executor in `status.executorState` is still pending, running or unknown | Retries are decided in `SUCCEEDING`, `FAILING` and `SUBMISSION_FAILED`, before these states are reached, so `restartPolicy` does not need to be checked. |
| `kubeflow.org/v1 PyTorchJob`, `TFJob` (training-operator v1.9.4) | condition `Succeeded` or `Failed` is `True`, `spec.runPolicy.cleanPodPolicy` is `All` or `Running`, and `spec.runPolicy.managedBy` is unset or the training operator | The default `cleanPodPolicy` is `None` and is not persisted, so a missing field means `None`. With `None`, replicas that have not finished keep running, so the job keeps counting. The rule does not use `status.replicaStatuses[].active`: on success the operator moves `active` into `succeeded` and sets it to 0 itself, so it does not prove the pods are gone. |
| `kubeflow.org/v2beta1 MPIJob` (mpi-operator v0.8.2) | condition `Succeeded` or `Failed` is `True`, `spec.runPolicy.cleanPodPolicy` is `All` or `Running`, `spec.runPolicy.managedBy` is unset or `kubeflow.org/mpi-operator`, and `status.completionTime` is set | The API comment says the default `cleanPodPolicy` is `Running`, but the operator defaults it to `None` in memory only (`setDefaultsRunPolicy`), so a missing field means `None`. Workers run sshd and never exit on their own. `Failed` caused by a worker eviction is set without `completionTime` and can still become `Succeeded`. |
| `argoproj.io/v1alpha1 Workflow` (argo-workflows v4.1.6) | `status.phase` is `Succeeded`, `Failed` or `Error`, `status.finishedAt` is set, the label `workflows.argoproj.io/completed` is `true`, and no node of type `Pod` in `status.nodes` is `Pending` or `Running` | `Error` can be set after a controller error while sibling pods keep running, hence the node check. When node statuses are compressed or offloaded and `status.nodes` is empty, a `Succeeded` or `Failed` workflow is still treated as terminated; only an `Error` workflow is not. `argo retry` moves a finished workflow back to running on the same object, see [Risks and Mitigations](#risks-and-mitigations). |
| `apps.kruise.io/v1alpha1 BroadcastJob` (OpenKruise v1.9.2) | `status.phase` is `completed` with condition `Complete` `True`, or `failed` with condition `Failed` `True`; `status.completionTime` is set; `status.active` is 0; for `completed`, `spec.completionPolicy.type` is not `Never`; for `failed`, the pod template's `restartPolicy` is `Never` | With `restartPolicy: OnFailure`, a pod over its restart limit is counted as failed but not deleted, so it can keep running after the job is `failed`. The spec is mutable: changing a `completed` job's `completionPolicy` to `Never` makes the controller create pods again. |
| `ray.io/v1 RayJob` (KubeRay v1.7.1) | no built-in rule | KubeRay deletes the RayCluster after the job finishes without recording it in status, so the observed object cannot prove the pods are gone. With the default `shutdownAfterJobFinishes: false`, the cluster keeps running. Users can override, see the example below. |

Other kinds Karmada ships customizations for can get rules the same way later. Until then they keep counting unless the user provides a rule. Long-running kinds (Deployment, StatefulSet, FlinkDeployment, RayCluster, ...) get no termination rule and are never treated as terminated.

The full rules and their test data (`testdata/interprettermination-test.yaml`) ship with the implementation. Three examples show the different patterns.

**Final state plus pod counts (Volcano `Job`).** The status reports how many pods are still alive, so the rule can wait for them:

```lua
function InterpretTermination(observedObj)
  if observedObj.status == nil or observedObj.status.state == nil then
    return false
  end
  local status = observedObj.status
  local phase = status.state.phase
  -- Aborted is not terminal: an aborted job can be resumed.
  if phase ~= 'Completed' and phase ~= 'Failed' and phase ~= 'Terminated' then
    return false
  end
  -- Completed can be set (e.g. once minSuccess is reached) while the remaining
  -- pods are still being killed. Pods in phase Unknown may still be running.
  if (status.pending or 0) > 0 or (status.running or 0) > 0 or (status.terminating or 0) > 0 or (status.unknown or 0) > 0 then
    return false
  end
  return true
end
```

**Final state only when cleanup is configured (`PyTorchJob`).** Whether pods are gone depends on a spec field with an unpersisted default:

```lua
function InterpretTermination(observedObj)
  if observedObj.status == nil or observedObj.status.conditions == nil then
    return false
  end
  local finished = false
  for _, condition in ipairs(observedObj.status.conditions) do
    if (condition.type == 'Succeeded' or condition.type == 'Failed') and condition.status == 'True' then
      finished = true
    end
  end
  if not finished then
    return false
  end
  -- The training operator defaults cleanPodPolicy to None in memory only, so a
  -- missing field means None. With None, replicas that have not finished keep
  -- running after the job finishes.
  if observedObj.spec == nil or observedObj.spec.runPolicy == nil then
    return false
  end
  local runPolicy = observedObj.spec.runPolicy
  -- Jobs managed by another controller (e.g. MultiKueue) are not cleaned up by
  -- the training operator.
  if runPolicy.managedBy ~= nil and runPolicy.managedBy ~= '' and runPolicy.managedBy ~= 'kubeflow.org/training-operator' then
    return false
  end
  return runPolicy.cleanPodPolicy == 'All' or runPolicy.cleanPodPolicy == 'Running'
end
```

**User override when the status cannot prove release (`RayJob`).** There is no built-in rule. A user who runs RayJobs with `shutdownAfterJobFinishes: true` and accepts releasing quota while the RayCluster shuts down can provide:

```yaml
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: rayjob-termination
spec:
  target:
    apiVersion: ray.io/v1
    kind: RayJob
  customizations:
    terminationInterpretation:
      luaScript: |
        function InterpretTermination(observedObj)
          if observedObj.spec == nil or observedObj.status == nil then
            return false
          end
          local spec = observedObj.spec
          local status = observedObj.status.jobDeploymentStatus
          if status ~= 'Complete' and status ~= 'Failed' then
            return false
          end
          -- Treat the job as terminated once KubeRay has been asked to delete the
          -- RayCluster right away. The status does not record when the deletion has
          -- finished, so quota is released while the cluster's pods shut down.
          if spec.shutdownAfterJobFinishes ~= true or spec.deletionStrategy ~= nil then
            return false
          end
          return spec.ttlSecondsAfterFinished == nil or spec.ttlSecondsAfterFinished == 0
        end
```

### Status collection changes

`pkg/controllers/status/work_status_controller.go`: in `reflectStatus`, after `interpretHealth`:

```go
func (c *WorkStatusController) interpretTermination(clusterObj *unstructured.Unstructured, work *workv1alpha1.Work) (*bool, error) {
	if !c.ResourceInterpreter.HookEnabled(clusterObj.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretTermination) {
		return nil, nil
	}
	terminated, err := c.ResourceInterpreter.InterpretTermination(clusterObj)
	if err != nil {
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonInterpretTerminationFailed, ...)
		return nil, err
	}
	return &terminated, nil
}
```

and set `ManifestStatus.Terminated`. If interpretation fails, `reflectStatus` still writes the status with `terminated` unset, then returns the error so the worker requeues the object with rate-limited backoff and interpretation is retried. A finished object may never change again, and `WorkStatusController.onUpdate` drops informer resyncs whose old and new objects are equal, so without the requeue a transient failure (for example an interpreter webhook outage) would keep the workload counted until the controller restarts.

`pkg/util/helper/workstatus.go`:

- `assembleWorkStatus` copies `ManifestStatus.Terminated` into `AggregatedStatusItem.Terminated`. It stays nil for items that are not applied.
- `AggregateResourceBindingWorkStatus` records a `TerminatedWorkloadReactivated` warning event on the binding when a scheduled cluster's `terminated` flips from `true` to `false`. The event is emitted here because the per-cluster flip is visible directly, so a terminated cluster that is simply removed from `spec.clusters` by rescheduling is not reported as a reactivation. `ClusterResourceBinding` is not counted by FRQ, so the cluster-scoped path does not do this.

Because these are status-only writes, they do not go through the ResourceBinding validating webhook (it only matches the `resourcebindings` resource, not `resourcebindings/status`), and they do not affect the detector.

### FederatedResourceQuota changes

#### Usage calculation

`helper.CalculateResourceUsage` is the single place both the controller and the webhook use to compute a binding's usage. It changes to skip terminated clusters:

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

- The calculation is driven by `spec.clusters`. An old `aggregatedStatus` item has no effect while its cluster is not scheduled. If the binding is rescheduled back to that cluster before the binding status aggregation removes the old item, the webhook briefly treats the cluster as terminated. The recomputation corrects this once the new Work reports status. This window is short and requires a reschedule back to the same cluster. A newly scheduled cluster has no item yet, so it is counted. This is the conservative choice.
- On the webhook path, `newRB.status` equals `oldRB.status` for main-resource updates (the status subresource is enabled), so the delta calculation stays consistent. Example: a spec update that does not change replicas on a terminated binding produces a zero delta, as it does today. This includes re-applying a changed spec to a finished `SparkApplication`, which reruns it on the member cluster (see [Risks and Mitigations](#risks-and-mitigations)).
- On create, a binding has no aggregated status, so it is counted in full. Nothing changes here.

#### Recomputing usage on termination changes

`FRQ.status.overallUsed` is written in two places today:

- The ResourceBinding webhook adds `CalculateResourceUsage(newRB) - CalculateResourceUsage(oldRB)` to every matching FRQ at admission time (`updateFRQStatusesWithRetrySignal` in `pkg/webhook/resourcebinding/validating.go`).
- The FRQ enforcement controller overwrites it with a full recomputation over the bindings in its informer cache (`collectQuotaStatus`). It does this on FRQ changes, binding deletion and the periodic resync.

Termination changes a binding's usage through a status update, which the webhook does not see, so a termination change triggers a full recomputation. A recomputation sets an absolute value, so a release cannot be counted twice.

**Trigger.** `resourceBindingPredicate` in `pkg/controllers/federatedresourcequota/federated_resource_quota_enforcement_controller.go` currently ignores binding updates. It changes to enqueue the binding's FRQs when the set of terminated scheduled clusters changes:

```go
UpdateFunc: func(e event.UpdateEvent) bool {
	oldRB := e.ObjectOld.(*workv1alpha2.ResourceBinding)
	newRB := e.ObjectNew.(*workv1alpha2.ResourceBinding)
	return !scheduledTerminatedClusters(oldRB).Equal(scheduledTerminatedClusters(newRB))
},
```

`collectQuotaStatus` already uses `CalculateResourceUsage`, so recomputations exclude terminated clusters without further changes.

The periodic resync (`--federated-resource-quota-sync-period`) remains as a safety net.

#### Webhook

`pkg/webhook/resourcebinding/validating.go` changes in one place: on updates, it always computes the usage delta instead of first filtering with `isQuotaRelevantFieldChanged`. `isQuotaRelevantFieldChanged` and the helpers only it uses (`isResourceRequestChanged`, `isScheduledReplicasChanged`, `isScheduledClusterChanged`, `isComponentsChanged`, `areResourceListsEqual`) are deleted.

Today, `isQuotaRelevantFieldChanged` skips quota validation when the resource requests, the total scheduled replicas and the components are unchanged. It compares the set of scheduled clusters only for bindings with components. Once usage depends on which scheduled clusters have terminated, that filter is no longer enough. For example, a reschedule that moves a replica-based binding from a terminated cluster to a new one, `[member1: 3, terminated]` → `[member2: 3]`, keeps the same total and would skip validation, so the 3 replicas now running on `member2` would not be counted until the next full recomputation.

Instead, `validateFederatedResourceQuota` computes `delta = CalculateResourceUsage(newRB) - CalculateResourceUsage(oldRB)` for every update, and returns early only when the delta is zero, as it already does after the filter today. In the example above the delta is `+3` replicas, so the update is checked against the quota. This stays cheap: `CalculateResourceUsage` is pure computation on the two objects in the request, and FRQs are only listed and updated when the delta is non-zero. Bindings with no terminated cluster behave exactly as before: the filter only skipped updates whose delta was zero anyway.

Because the FRQ enforcement controller recomputes `status.overallUsed` as soon as termination is observed, the next binding admission sees the freed quota.

### Feature gate

No new feature gate is proposed:

- `FederatedQuotaEnforcement` is Alpha and off by default, and @RainbowMango [agreed](https://github.com/karmada-io/karmada/issues/7931#issuecomment-6122561940) that FRQ should ignore finished workloads by default. Making this the behavior of `FederatedQuotaEnforcement` is consistent with native `ResourceQuota`.
- `InterpretTermination` and the new status fields only record information, so they are not gated. With `FederatedQuotaEnforcement` off, nothing consumes them.
- A separate gate for the quota filtering alone would only add a combination to test, with no user-facing benefit while the parent feature is still Alpha.

### Compatibility

- **API.** All new fields are optional and additive. Old controllers ignore them, and new controllers treat a missing field as "not terminated".
- **Webhook interpreters.** Existing `ResourceInterpreterWebhookConfiguration`s that match `"*"` operations will start receiving `InterpretTermination` requests. Webhooks that do not recognize the operation should return an error or omit `terminated`. Either way the object is treated as not terminated and an event is recorded, so behavior is unchanged. Webhook authors can opt in by adding the operation to their rules and returning `terminated`.
- **Upgrade.** After upgrade, existing finished workloads are picked up the next time their member status is reflected. For a workload that has already finished, its member object usually no longer changes, so in practice this happens when karmada-controller-manager restarts: the member informers re-list and `WorkStatusController.onAdd` enqueues every object. Informer resyncs do not help, because `WorkStatusController.onUpdate` drops events whose old and new objects are equal. Upgrading karmada-controller-manager restarts it anyway, so finished workloads are released shortly after the upgrade.
- **Pull-mode clusters.** Termination is evaluated where status is reflected: by karmada-controller-manager for `Push` clusters, and by karmada-agent for `Pull` clusters. karmada-agent must be upgraded and restarted too. Until then, it leaves `terminated` unset, so workloads on `Pull` clusters keep counting toward quota.

### Implementation Plan

- **Core**: the API changes, the interpreter operation in all layers, status collection, the FRQ recomputation trigger and the webhook change, plus the native `Job` and `Pod` rules. The third-party loading path is covered by unit tests.
- **Remaining built-in rules**: one PR per kind, tracked in an umbrella issue with one item each, starting from the reference rules in [Built-in termination rules](#built-in-termination-rules).

### Test Plan

- **Unit tests**
  - Native `InterpretTermination` for `Job` (complete, failed, running, suspended) and `Pod` (all phases).
  - Work status controller: a failed interpretation reflects status with `terminated` unset and requeues the object; the retry sets `terminated`.
  - Third-party customization test data for each kind with a built-in rule, in `testdata/interprettermination-test.yaml` next to the existing per-operation test files.
  - Declarative Lua VM and webhook interpreter for the new operation, including response verification.
  - `assembleWorkStatus`: terminated, not terminated, unapplied clusters, and no hook (nil `terminated`).
  - `CalculateResourceUsage`: replica path and components path, partial termination, stale items for unscheduled clusters, no hook.
  - FRQ controller predicate: enqueue only when the set of terminated scheduled clusters changes.
  - A binding's termination is stored while a recomputation is running: the release is counted exactly once.
  - `TerminatedWorkloadReactivated` event on `true` → `false` only, not when a terminated cluster is removed from `spec.clusters`.
  - RB webhook: delta with terminated clusters on spec update and create; a reschedule from a terminated cluster to a new cluster with the same total replicas is validated against the quota.
- **E2E tests** (`test/e2e/suites/base/federatedresourcequota_test.go`)
  - The reproduction from #7931: FRQ `cpu: 2`, run a 1-CPU Job to completion, verify `overallUsed.cpu` drops to `0`, then admit two more Jobs.
  - Divided Job across two clusters, one finishing first: verify partial release.
  - Submit new Jobs while others finish: verify `overallUsed` matches the full recomputation.
  - Declarative customization for a CRD with `terminationInterpretation`.
  - A Deployment is never excluded.
- **Production validation.** We will validate with Spark batch workloads in our QA and production control planes.
