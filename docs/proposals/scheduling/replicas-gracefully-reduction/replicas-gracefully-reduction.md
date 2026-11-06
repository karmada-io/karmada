---
title: Replica of workload gracefully reduction
authors:
  - "@chaosi-zju"
reviewers:
  - "@RainbowMango"
  - "TBD"
approvers:
  - "@RainbowMango"
  - "TBD"

creation-date: 2024-03-26
---

# Replica of workload gracefully reduction

## Background

When the target cluster of the workload changes, for example, the workload migrates from the member1 cluster to the member2 cluster.

The karmada performs the deletion of the workload on the old cluster and the creation of the new cluster at the same time.

Therefore, it is possible that the new workload is not ready but the old workload has been deleted, resulting in application out of service.

It is expected that there is a more elegant measure of workload deletion.

## Proposal

### Overview

I propose to introduce a configuration in the PropagationPolicy, so that users can choose the deletion time of `Work`. 

When the `Work` of some member clusters needs to be deleted, you can configure to delay the deletion operation until 
all the desired replicas of other clusters are ready.

Thus, avoiding the problem of application out of service.

### User story

#### Story 1

In the scenario that replicas in some member clusters reduce from N to 0 due to placement change, as a user, 
I want the replicas reduction to be delayed until all others clusters' replicas ready, so that there will always be ready replicas works.

#### Story 2

In the scenario that replicas in some member clusters reduce from N to M due to placement change, as a user,
I want the replicas reduction could be as smooth as possible, so that avoiding replicas reduce quickly while increase slowly,
leading to not enough replicas available to handle business traffic.

## Design Details

### API change

* Add a new field named `ReplicaReductionStrategy` to PropagationPolicy and ResourceBinding, this field allows the user 
to specify when the replicas to be reduced.

> this proposal mainly considers the scenario that replicas in some member clusters reduce from N to 0, the reduction can
> be delayed until all others clusters' replicas ready. However, in the future, the demand may expand into that replicas 
> in some member clusters reduce from N to M, the reduction may also be required to be delayed.

```go
// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
    ...
	// ReplicaReductionStrategy
	// +optional
	ReplicaReductionStrategy ReplicaReductionStrategy `json:"replicaReductionStrategy,omitempty"`
}

// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ...
    // ReplicaReductionStrategy
    // +optional
	ReplicaReductionStrategy ReplicaReductionStrategy `json:"replicaReductionStrategy,omitempty"`
}

// ReplicaReductionStrategy indicates how the replicas of workload will be reduced.
type ReplicaReductionStrategy string

const (
    // DelayWorkDeletion 
    DelayWorkDeletion ReplicaReductionStrategy = "DelayWorkDeletion"
)
```

* Add a new field named `ReplicaReductionTasks` to ResourceBinding, to record which cluster's replica reduction tasks are pending.

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ...
    // ReplicaReductionStrategy
	// key is cluster name, value is detail task info
    // +optional
	ReplicaReductionTasks map[string]ReplicaReductionTask `json:"replicaReductionQueue,omitempty"`
}

type ReplicaReductionTask struct {
    // GracePeriodSeconds is the maximum waiting duration in seconds before the item
    // should be deleted. If the application on the new cluster cannot reach a Healthy state,
    // Karmada will delete the item after GracePeriodSeconds is reached.
    // Value must be positive integer.
    // It can not co-exist with SuppressDeletion.
    // +optional
    GracePeriodSeconds *int32 `json:"gracePeriodSeconds,omitempty"`
    
    // SuppressDeletion represents the grace period will be persistent until
    // the tools or human intervention stops it.
    // It can not co-exist with GracePeriodSeconds.
    // +optional
    SuppressDeletion *bool `json:"suppressDeletion,omitempty"`
    
    // CreationTimestamp is a timestamp representing the server time when this object was
    // created.
    // Clients should not set this value to avoid the time inconsistency issue.
    // It is represented in RFC3339 form(like '2021-04-25T10:02:10Z') and is in UTC.
    //
    // Populated by the system. Read-only.
    // +optional
    CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty"`
}
```

### Implementation logic

1) As for `BindingController`, when we sync ResourceBinding to Works, we will find orphan works which no longer relevant to 
certain resourceBinding and directly remove it. Now, we change to judging `ReplicaReductionStrategy` in binding before
directly remove the work, if the `ReplicaReductionStrategy` is `DelayWorkDeletion`, we just record which cluster has a delay task
into `ReplicaReductionTasks` instead.

2) Add a new worker in `BindingController`, this worker focus on the `ReplicaReductionTasks` filed of ResourceBinding.
If not empty, which means replicas of workload in a group of clusters needs to be adjusted. The worker will wait to adjust 
the replicas of these clusters to the expected replicas until the actual ready replicas is equal to the expected replicas in all other clusters.
