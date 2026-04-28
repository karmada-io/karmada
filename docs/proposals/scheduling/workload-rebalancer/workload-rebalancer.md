---
title: Introduce a rebalance mechanism to actively trigger rescheduling of resource.
authors:
  - "@chaosi-zju"
reviewers:
  - "@RainbowMango"
  - "@chaunceyjiang"
  - "@wu0407"
approvers:
  - "@RainbowMango"

creation-date: 2024-01-30
---

# Introduce a mechanism to actively trigger rescheduling

## Background

According to current karmada scheduler, after replicas of workloads is scheduled, it will keep the scheduling result inert 
and the replicas distribution will not change. Even if reschedule is triggered by modifying replicas or placement, 
it will maintain the exist replicas distribution as closely as possible, only making minimal adjustments when necessary, 
which minimizes disruptions and preserves the balance across clusters.

However, in some scenarios, users hope to have approach to actively trigger a fresh rescheduling, which disregards the 
previous assignment entirely and seeks to establish an entirely new replica distribution across clusters.

### Motivation

Assuming the user has propagated the workloads to member clusters, in some scenarios the current replicas distribution
is not the most expected, such as:

* replicas migrated due to cluster failover, while now cluster recovered.
* replicas migrated due to application-level failover, while now each cluster has sufficient resources to run the replicas.
* as for `Aggregated` schedule strategy, replicas were initially distributed across multiple clusters due to resource
  constraints, but now one cluster is enough to accommodate all replicas.

Therefore, the user desires for an approach to trigger rescheduling so that the replicas distribution can do a rebalance.

### Goals

Introduce a rebalance mechanism to actively trigger rescheduling of resource.

## Proposal

* **Introduce a configurable field into resource binding, and when it changes, the scheduler will perform a `Fresh` mode
  rescheduling.**

> In contrast to existing assignment mode of rescheduling, such as those triggered by modification of replicas or
> placement, will maintain the exist replicas distribution as closely as possible, the assignment mode of this rescheduling
> disregards the previous assignment entirely and seeks to establish an entirely new replica distribution across clusters.
>
> We call the former assignment as `Steady` mode and the latter as `Fresh` mode.

* **Introduce a new API, by which the users can actively adjust workload balance.**

> Since directly manipulating bindings is not the recommended friendly way, it would be better to design a new API
> specifically for adjusting workload balance. Currently, it is mainly considered for rescheduling scenario.
> In the future, it may continue to expand more workload rebalance scenarios, such as migration, rollback and so on,
> with different assignment modes and rolling modes specified.

### User story

#### Story 1

In cluster failover scenario, replicas are distributed in member1 + member2 two clusters, however they would all migrate to
member2 cluster if member1 cluster fails.

As a cluster administrator, I hope the replicas redistribute to two clusters when member1 cluster recovered, so that 
the resources of the member1 cluster will be re-utilized, also for the sake of high availability.

#### Story 2

In application-level failover, low-priority applications may be preempted, resulting in shrinking from multi clusters 
to single cluster due to cluster resources are in short supply
(refer to [Application-level Failover](https://karmada.io/docs/next/userguide/failover/application-failover#why-application-level-failover-is-required)).

As a user, I hope the replicas of low-priority applications can be redistributed to multi clusters when
cluster resources are sufficient to ensure the high availability of application.

#### Story 3

In `Aggregated` schedule type, replicas may still distribute across multiple clusters due to resource constraints.

As a user, I hope the replicas to be redistributed in an aggregated strategy when any cluster has
sufficient resource to accommodate all replicas, so that the application better meets actual business requirements.


#### Story 4

In disaster-recovery scenario, replicas migrated from primary cluster to backup cluster when primary cluster failure.

As a cluster administrator, I hope that replicas can migrate back when cluster restored, so that:

1. restore to the disaster-recovery mode to ensure the reliability and stability of the cluster federation.
2. save the cost of the backup cluster.

### Notes/Constraints/Caveats

This ability is limited to triggering workload rebalance, the schedule result will be recalculated according to the
`Placement` in the current ResourceBinding. That means:

* Take [story 1](#story-1) as an example, reschedule happened when cluster recovered, but the new schedule result is not 
guaranteed to be exactly the same as before the cluster failure, it is only guaranteed that the new schedule result meets
current `Placement`.

* Rebalance is basing on `Placement` in the current ResourceBinding, not PropagationPolicy. So if your activation preference
of PropagationPolicy is `Lazy`, the rescheduling is still basing on previous `ResourceBinding` even if the current Policy has been changed.

## Design Details

### API change

* As for *Introduce a configurable field into resource binding*, detail description is as follows:

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ...
  // RescheduleTriggeredAt is a timestamp representing when the referenced resource is triggered rescheduling.
  // When this field is updated, it means a rescheduling is manually triggered by user, and the expected behavior
  // of this action is to do a complete recalculation without referring to last scheduling results.
  // It works with the status.lastScheduledTime field, and only when this timestamp is later than timestamp in
  // status.lastScheduledTime will the rescheduling actually execute, otherwise, ignored.
  //
  // It is represented in RFC3339 form (like '2006-01-02T15:04:05Z') and is in UTC.
  // +optional
  RescheduleTriggeredAt *metav1.Time `json:"rescheduleTriggeredAt,omitempty"`
    ...
}

// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
    ...
  // LastScheduledTime representing the latest timestamp when scheduler successfully finished a scheduling.
  // It is represented in RFC3339 form (like '2006-01-02T15:04:05Z') and is in UTC.
  // +optional
  LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`
    ...
}
```

* As for *Introduce a new API, by which the users can actively adjust workload balance.*, we define a new API 
  named `WorkloadRebalancer` into a new apiGroup `apps.karmada.io/v1alpha1`:

```go
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=workloadrebalancers,scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadRebalancer represents the desired behavior and status of a job which can enforces a resource rebalance.
type WorkloadRebalancer struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`
  
  // Spec represents the specification of the desired behavior of WorkloadRebalancer.
  // +required
  Spec WorkloadRebalancerSpec `json:"spec"`
  
  // Status represents the status of WorkloadRebalancer.
  // +optional
  Status WorkloadRebalancerStatus `json:"status,omitempty"`
}

// WorkloadRebalancerSpec represents the specification of the desired behavior of Reschedule.
type WorkloadRebalancerSpec struct {
  // Workloads used to specify the list of expected resource.
  // Nil or empty list is not allowed.
  // +kubebuilder:validation:MinItems=1
  // +required
  Workloads []ObjectReference `json:"workloads"`

  // TTLSecondsAfterFinished limits the lifetime of a WorkloadRebalancer that has finished execution (means each
  // target workload is finished with result of Successful or Failed).
  // If this field is set, ttlSecondsAfterFinished after the WorkloadRebalancer finishes, it is eligible to be automatically deleted.
  // If this field is unset, the WorkloadRebalancer won't be automatically deleted.
  // If this field is set to zero, the WorkloadRebalancer becomes eligible to be deleted immediately after it finishes.
  // +optional
  TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// ObjectReference the expected resource.
type ObjectReference struct {
  // APIVersion represents the API version of the target resource.
  // +required
  APIVersion string `json:"apiVersion"`
  
  // Kind represents the Kind of the target resource.
  // +required
  Kind string `json:"kind"`
  
  // Name of the target resource.
  // +required
  Name string `json:"name"`
  
  // Namespace of the target resource.
  // Default is empty, which means it is a non-namespacescoped resource.
  // +optional
  Namespace string `json:"namespace,omitempty"`
}

// WorkloadRebalancerStatus contains information about the current status of a WorkloadRebalancer
// updated periodically by schedule trigger controller.
type WorkloadRebalancerStatus struct {
  // ObservedWorkloads contains information about the execution states and messages of target resources.
  // +optional
  ObservedWorkloads []ObservedWorkload `json:"observedWorkloads,omitempty"`

  // ObservedGeneration is the generation(.metadata.generation) observed by the controller.
  // If ObservedGeneration is less than the generation in metadata means the controller hasn't confirmed
  // the rebalance result or hasn't done the rebalance yet.
  // +optional
  ObservedGeneration int64 `json:"observedGeneration,omitempty"`

  // FinishTime represents the finish time of rebalancer.
  // +optional
  FinishTime *metav1.Time `json:"finishTime,omitempty"`
}

// ObservedWorkload the observed resource.
type ObservedWorkload struct {
  // Workload the observed resource.
  // +required
  Workload ObjectReference `json:"workload"`
  
  // Result the observed rebalance result of resource.
  // +optional
  Result RebalanceResult `json:"result,omitempty"`
  
  // Reason represents a machine-readable description of why this resource rebalanced failed.
  // +optional
  Reason RebalanceFailedReason `json:"reason,omitempty"`
}

// RebalanceResult the specific extent to which the resource has been rebalanced
type RebalanceResult string

const (
  // RebalanceFailed the resource has been rebalance failed.
  RebalanceFailed RebalanceResult = "Failed"
  // RebalanceSuccessful the resource has been successfully rebalanced.
  RebalanceSuccessful RebalanceResult = "Successful"
)

// RebalanceFailedReason represents a machine-readable description of why this resource rebalanced failed.
type RebalanceFailedReason string

const (
  // RebalanceObjectNotFound the resource referenced binding not found.
  RebalanceObjectNotFound RebalanceFailedReason = "ReferencedBindingNotFound"
)
```

### Interpretation of Realization by an Example

#### Step 1. apply WorkloadRebalancer resource yaml.

Assuming there is two Deployment named `demo-deploy-1` and `demo-deploy-2`, and a ClusterRole named `demo-role`, 
the user wants to trigger their rescheduling, he just needs to apply following yaml:

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: WorkloadRebalancer
metadata:
  name: demo
spec:
  workloads:
    - apiVersion: apps/v1
      kind: Deployment
      name: demo-deploy-1
      namespace: default
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRole
      name: demo-role
    - apiVersion: apps/v1
      kind: Deployment
      name: demo-deploy-2
      namespace: default
```

> Notes: as for `workloads` field:
> 1. `name` sub-field is required;
> 2. `namespace` sub-field is required when it is a namespace scoped resource, while empty when it is a cluster wide
     resource;

This API specified a batch of resources which needs a rescheduling, and the user will get a `workloadrebalancer.apps.karmada.io/demo created`
result, which means the API created success.

#### Step 2: Controller listens new API resource and do the rescheduling work.

Then the controller will work to trigger the rescheduling for each resource, by writing the `CreationTimestamp` of WorkloadRebalancer
to each resource binding's new field `spec.placement.rescheduleTriggeredAt`. Take `deployment/demo-deploy-1` as example,
you will see its resource binding be modified to:

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: demo-deploy-1-deployment
  namespace: default
spec:
  rescheduleTriggeredAt: "2024-04-17T15:04:05Z"   # this field would be updated to CreationTimestamp of WorkloadRebalancer
  ...
status:
  lastScheduledTime: "2024-04-17T15:00:05Z"
```

Since field `rescheduleTriggeredAt` updated, and it is later than field `lastScheduledTime`, rescheduling is triggered.
If it succeeds, the `lastScheduledTime` field will be updated again, which represents scheduler finished a rescheduling 
(if failed, the scheduler will retry), detail as follows:

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: demo-deploy-1-deployment
  namespace: default
spec:
  rescheduleTriggeredAt: "2024-04-17T15:04:05Z"
  ...
status:
  lastScheduledTime: "2024-04-17T15:04:05Z"
  conditions:
    - ...
    - lastTransitionTime: "2024-04-17T15:00:05Z"
      message: Binding has been scheduled successfully.
      reason: Success
      status: "True"
      type: Scheduled
    - lastTransitionTime: "2024-04-17T15:04:05Z"
      message: All works have been successfully applied
      reason: FullyAppliedSuccess
      status: "True"
      type: FullyApplied
```

Finally, all works have been successfully applied, the user will observe changes in the actual distribution of resource
template; the user can also see several recorded event in resource template, just like:

```shell
$ kubectl --context karmada-apiserver describe deployment demo-deploy-1
...
Events:
  Type    Reason                  Age                From                                Message
  ----    ------                  ----               ----                                -------
  ...
  Normal  ScheduleBindingSucceed  31s                default-scheduler                   Binding has been scheduled successfully. Result: {member2:2, member1:1}
  Normal  GetDependenciesSucceed  31s                dependencies-distributor            Get dependencies([]) succeed.
  Normal  SyncSucceed             31s                execution-controller                Successfully applied resource(default/demo-deploy-1) to cluster member1
  Normal  AggregateStatusSucceed  31s (x4 over 31s)  resource-binding-status-controller  Update resourceBinding(default/demo-deploy-1-deployment) with AggregatedStatus successfully.
  Normal  SyncSucceed             31s                execution-controller                Successfully applied resource(default/demo-deploy-1) to cluster member2
```

#### Step 3: check the status of WorkloadRebalancer.

The user can observe the rebalance result at `status.observedWorkloads` of `workloadrebalancer/demo`, just like:

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: WorkloadRebalancer
metadata:
  creationTimestamp: "2024-04-17T15:04:05Z"
  name: demo
spec:
  workloads:
    - apiVersion: apps/v1
      kind: Deployment
      name: demo-deploy-1
      namespace: default
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRole
      name: demo-role
    - apiVersion: apps/v1
      kind: Deployment
      name: demo-deploy-2
      namespace: default
status:
  observedWorkloads:
    - result: Successful
      workload:
        apiVersion: apps/v1
        kind: Deployment
        name: demo-deploy-1
        namespace: default
    - reason: ReferencedBindingNotFound
      result: Failed
      workload:
        apiVersion: apps/v1
        kind: Deployment
        name: demo-deploy-2
        namespace: default
    - result: Successful
      workload:
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRole
        name: demo-role
```

> Notes:
> 1. the `observedWorkloads` is sorted in increasing dict order of the combined string of `apiVersion/kind/namespace/name` .
> 2. if workload referenced binding not found, it will be marked as `failed` without retry.
> 3. if workload rebalanced failed due to occasional network error, the controller will retry, and its `result` and `reason`
> field will left empty until it succeeds.

### How to update this resource

When `spec` filed of WorkloadRebalancer updated, we shall refresh the workload list in `status.observedWorkloads`:

* a new workload added to spec list, just add it into status list too and do the rebalance.
* a workload deleted from previous spec list, keep it in status list if already success, and remove it if not.
* a workload is modified, just regard it as deleted an old one and inserted a new one.
* if the modification only involves a list order adjustment, no additional action, since `observedWorkloads` is arranged in increasing dict order.

### How to auto clean resource

referring to [Automatic Cleanup for Finished Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/).

Introduces field `ttlSecondsAfterFinished` which limits the lifetime of a WorkloadRebalancer that has finished execution 
(finished execution means each target workload is finished with result of `Successful` or `Failed`).

* If this field is set, `ttlSecondsAfterFinished` after the WorkloadRebalancer finishes, it is eligible to be automatically deleted.
* If this field is unset, the WorkloadRebalancer won't be automatically deleted.
* If this field is set to zero, the WorkloadRebalancer becomes eligible to be deleted immediately after it finishes.

Considering several corner cases:

* case 1: if a new target workload was added into `WorkloadRebalancer` before `ttlSecondsAfterFinished` expired, 
  which means the finish time of the `WorkloadRebalancer` is refreshed, so the `delete` action is deferred since expire time is refreshed too.
* case 2: if `ttlSecondsAfterFinished` is modified before `ttlSecondsAfterFinished` expired, 
  `delete` action should be performed according to latest `ttlSecondsAfterFinished`.
* case 3: when we have got and checked latest `WorkloadRebalancer` object and try to delete it, 
  if a modification to `WorkloadRebalancer` occurred just right between the two time point, the previous `delete` action should be Interrupted.

Several key implementation:
* A `WorkloadRebalancer` is judged as finished should meet two requirements:
  *  all expected workloads are finished with result of `Successful` or `Failed`.
  *  introduce a new field named `ObservedGeneration` to `Status` of WorkloadRebalancer, and it should be equal to `.metadata.Generation`,
     to prevent that the WorkloadRebalancer is updated but controller hasn't in time refreshed its `Status`.
* When `WorkloadRebalancer` is `Created` or `Updated`, add it to the workqueue and calculate its expiring time, and 
  call `workqueue.AddAfter()` function to re-enqueue it once more if it hasn't expired.
* Before deleting the `WorkloadRebalancer`, do a final sanity check. Use the latest `WorkloadRebalancer` directly 
  fetched from api server to see if the TTL truly expires, rather than object from lister cache.
* When deleting the `WorkloadRebalancer`, it is needed to confirm that the `resourceVersion` of the deleted object is as expected,
  to prevent from above corner case 3.

### How to prevent application from being out-of-service

As for disaster-recovery scenario mentioned in above [story 4](#story-4), after primary cluster recovered and reschedule 
has been triggered, if new replicas in primary cluster become ready later than old replicas removed from backup cluster,
there may be no ready replica in cluster federation and the application will be out-of-service. So, how to prevent 
application from being out-of-service?

This will be discussed and implemented separately in another proposal.
