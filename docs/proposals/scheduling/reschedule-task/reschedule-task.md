---
title: Introduce a mechanism to actively triggle rescheduling
authors:
  - "@chaosi-zju"
reviewers:
  - "@RainbowMango"
  - "@chaunceyjiang"
  - "TBD"
approvers:
  - "@RainbowMango"
  - "TBD"

creation-date: 2024-01-30
---

# Introduce a mechanism to actively trigger rescheduling

## Background

According to the current implementation, after the replicas of workload is scheduled, it will remain inertia and the 
replicas distribution will not change. 

However, in some scenarios, users hope to have means to actively trigger rescheduling.

### Motivation

Assuming the user has propagated the workloads to member clusters, replicas migrated due to member cluster failure.

However, the user expects an approach to trigger rescheduling after member cluster restored, so that replicas can
migrate back.

### Goals

Introduce a mechanism to actively trigger rescheduling of workload resource.

### Applicable scenario

This feature might help in a scenario where: the `replicas` in resource template or `placement` in policy has not changed, 
but the user wants to actively trigger rescheduling of replicas.

## Proposal

### Overview

This proposal aims to introduce a mechanism of active triggering rescheduling, which benefits a lot in application 
failover scenarios. This can be realized by introducing a new API, and a new field would be marked when this new API 
called, so that scheduler can perceive the need for rescheduling.

### User story

In application failover scenarios, replicas migrated from primary cluster to backup cluster when primary cluster failue.

As a user, I want to trigger replicas migrating back when cluster restored, so that:

1. restore the disaster recovery mode to ensure the reliability and stability of the cluster.
2. save the cost of the backup cluster.

### Notes/Constraints/Caveats

This ability is limited to triggering rescheduling. The scheduling result will be recalculated according to the
Placement in the current ResourceBinding, and the scheduling result is not guaranteed to be exactly the same as before
the cluster failure.

> Notes: pay attention to the recalculation is basing on Placement in the current `ResourceBinding`, not "Policy". So if
> your activation preference of Policy is `Lazy`, the rescheduling is still basing on previous `ResourceBinding` even if
> the current Policy has been changed.

## Design Details

### API change

* Introduce a new API named `RescheduleTask` into a new apiGroup `task.karmada.io`:

```go
//revive:disable:exported

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RescheduleTask represents the desire state and status of a task which can enforces a rescheduling.
type RescheduleTask struct {
    metav1.TypeMeta
    metav1.ObjectMeta
    
    // Spec represents the specification of the desired behavior of RescheduleTask.
    // +required
    Spec RescheduleTaskSpec `json:"spec"`
}

type RescheduleTaskSpec struct {
    // TargetRefPolicy used to select batch of resources managed by certain policies.
    // +optional
    TargetRefPolicy []PolicySelector `json:"targetRefPolicy,omitempty"`
    
    // TargetRefResource used to select resources.
    // +optional
    TargetRefResource []ResourceSelector `json:"targetRefResource,omitempty"`
}

type PolicySelector struct {
    // Namespace of the target policy.
    // Default is empty, which means inherit from the parent object scope.
    // +optional
    Namespace string `json:"namespace,omitempty"`
    
    // Name of the target resource.
    // Default is empty, which means selecting all resources.
    // +optional
    Name string `json:"name,omitempty"`
}
```

* Add a new field named `ForceRescheduling` to ResourceBinding/ClusterResourceBinding

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ...
    // ForceRescheduling indicates that the scheduler should enforces a rescheduling
    // Defaults to false.
    // +optional
    ForceRescheduling bool `json:"forceRescheduling,omitempty"`
    ...
}
```

### Example

Assuming there is a Deployment named `nginx`, the user wants to trigger its rescheduling,
he just needs to apply following yaml:


```yaml
apiVersion: task.karmada.io/v1alpha1
kind: RescheduleTask
metadata:
  name: demo-task
spec:
  targetRefResource:
  - apiVersion: apps/v1
    kind: Deployment
    name: nginx
    namespace: default
```

Then, he will get a `rescheduletask.task.karmada.io/demo-task created` result, which means the task started, attention,
not finished. Simultaneously, he will see the new field `spec.placement.forceRescheduling` in binding of the selected
resource been set to `true`.

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: nginx-deployment
  namespace: default
spec:
  forceRescheduling: true
  ...
```

Then, rescheduling is in progress. If it succeeds, the `status.conditions` field of binding will be updated,
the `spec.placement.forceRescheduling` field of binding will be unset; If it failed, scheduler will retry.

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: nginx-deployment
  namespace: default
spec:
  forceRescheduling: false
  ...
status:
  conditions:
    - ...
    - lastTransitionTime: "2024-03-08T08:53:03Z"
      message: Binding has been scheduled successfully.
      reason: Success
      status: "True"
      type: Scheduled
    - lastTransitionTime: "2024-03-08T08:53:03Z"
      message: All works have been successfully applied
      reason: FullyAppliedSuccess
      status: "True"
      type: FullyApplied
```

Finally, all works have been successfully applied.

### Implementation logic

1) add an aggregated-api into karmada-aggregated-apiserver, detail described as above.

2) add an aggregated-api handler into karmada-aggregated-apiserver, which only impletement `Create` method. It will 
fetch all referred resource declared in `targetRefResource` or indirectly declared by `targetRefPolicy`, and then set
`spec.forceRescheduling` field to `true` in corresponding ResourceBinding.

> This api is no resource, not need to restore, no state, no idempotency, no implemention of `Update` or `Dalete` method.
> This is also why we not choose CRD type API.

3) in scheduling process, add a trigger condition: even if `Placement` and `Replicas` of binding unchanged, schedule will
be triggerred if `spec.forceRescheduling` if set `true`. After schedule finished, scheduler will unset this field when refreshing
binding back.
