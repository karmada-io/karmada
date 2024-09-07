---
title: Introducing a scheduling strategy to customize cluster propagation priorities
authors:
- "@chaosi-zju"
reviewers:
- "@TBD"
approvers:
- "@RainbowMango"

creation-date: 2024-09-07
---

# Introducing a scheduling strategy to customize cluster propagation priorities

## Summary

This proposal introduces a new user scenario for scheduling, where users have preferences for clusters with 
varying priorities and want replicas assigned to their preferred clusters.

To support this scenario, we propose two alternative solutions: one is adding `aggregatePreference` to the existing 
`Weighted` replica scheduling strategy, and the other is enhancing the capability of multiple scheduling groups.

Ultimately, we compare these two options and decide on the best implementation to provide a scheduling strategy 
that allows users to customize cluster propagation priorities.

## Motivation

Karmada currently supports two `Divided` type scheduling strategies, these are `Weighted` and `Aggregated`.
In addition, in `Weighted` scheduling, users can also customize their clusters weight.
However, the current strategies are still not satisfied in certain user scenarios. 

For example, the user has multiple clusters, and he wants to use some preferred clusters first. 
Only if these preferred clusters are not sufficiently allocated, the remaining clusters are used.

From the perspective of a deployment, it should be like this:
* When first propagation, prioritize allocation to preferred clusters as much as possible.
* When replicas scaling up, prioritize replicas expansion in preferred clusters (existing replicas remain unchanged).
* When replicas scaling down, prioritize replicas reduction in non-preferred clusters.

### Goals

* Provides a scheduling strategy that allows users to customize cluster propagation priorities.

### Non-Goals

* Whether clusters with the same priority are `Aggregated` or `Weighted` is not specially required.

## Proposal

### User Stories (Optional)

#### Story 1

As a cluster administrator, I manage two clusters, one is a cheap private cloud cluster and the other is an expensive public cloud cluster.

I hope that the user's replicas will give priority to the specified cheaper cluster, whether it is first propagation or scaling up.
Only when the cheaper cluster has insufficient capacity, the expensive cluster will be used, so that I can save more costs.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

Here are two solutions have now been proposed, and we need to confirm the final solution.

### Solution one: Add aggregatePreference to Weighted replicaSchedulingStrategy

In the `Divided` replica scheduling type, here are two replica division preference, `Weighted` and `Aggregated`.
Since the `Weighted` division type can specify the weight for each cluster through the `weightPreference` field,
so for the `Aggregated` division type, it is a reasonable design to introduce an `aggregatePreference` to 
customize clusters' aggregation priority.

#### API changes

add a new field `aggregatePreference` to `ReplicaSchedulingStrategy` of the PropoagationPolicy, as following:

```go
// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// ReplicaDivisionPreference determines how the replicas is divided
	// when ReplicaSchedulingType is "Divided". Valid options are Aggregated and Weighted.
	// "Aggregated" divides replicas into clusters as few as possible,
	// while respecting clusters' resource availabilities during the division.
	// "Weighted" divides replicas by weight according to WeightPreference.
	// +kubebuilder:validation:Enum=Aggregated;Weighted
	// +optional
	ReplicaDivisionPreference ReplicaDivisionPreference `json:"replicaDivisionPreference,omitempty"`

	// WeightPreference describes weight for each cluster or for each group of cluster
	// If ReplicaDivisionPreference is set to "Weighted", and WeightPreference is not set, scheduler will weight all clusters the same.
	// +optional
	WeightPreference *ClusterPreferences `json:"weightPreference,omitempty"`

+	// AggregatePreference describes aggregation priority for each cluster or for each group of cluster
+	// If ReplicaDivisionPreference is set to "Aggregated", and AggregatePreference is not set, scheduler will sort the clusters by score.
+	// +optional
+	AggregatePreference *AggregatePreference `json:"aggregatePreference"`
}
```

the newly added type `AggregatePreference` is defined as following:

```go
// AggregatePreference describes aggregation priority for each cluster or for each group of cluster
type AggregatePreference struct {
    // PreferredAggregationList defines the clusters' preferred aggregation priority.
    // +optional
	PreferredAggregationList []ClusterAggregationPriority `json:"preferredAggregationList"`
}

// ClusterAggregationPriority defines the clusters' preferred aggregation priority.
type ClusterAggregationPriority struct {
    // TargetCluster describes the filter to select clusters.
    // +required
    TargetCluster ClusterAffinity `json:"targetCluster"`
    
    // Priority expressing the priority of the cluster(s) specified by 'TargetCluster'. 
    // larger numbers mean higher priority.
    // +kubebuilder:validation:Minimum=1
    // +required
    Priority int64 `json:"priority"`
}
```

#### User usage example

Supposing the user has two clusters (member1/member2), and he wants replicas to be aggregated first in member1 clusters,
then he can define the policy as following:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: default-pp
spec:
  #...
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Aggregated
      aggregatePreference:
        preferredAggregationList:
          - targetCluster:
              clusterNames:
                - member1
            priority: 2
          - targetCluster:
              clusterNames:
                - member2
            priority: 1
```

As you see, the user specifies the cluster aggregation priority through `spec.placement.replicaScheduling.aggregatePreference` 
field, the member1 cluster has a higher priority and will be aggregated first.

#### Controller logic changes

The logic of the scheduler refers to the static weight scheduling. Before `selectClusters` step, it will still 
go through `filter` and `score` plug-in:

* The `filter` plug-in will take effect, and the feasible clusters will be the filtered clusters basing on user provided clusters.
* The `score` plug-in will not take effect, and clusters will be sorted according to the priority specified by the user.

As for `assignReplicas` step, the detail assign methods remain unchanged, but the order of the input available clusters is different.

* Previous: first sort by `MaxAvailableReplicas`, and then resort to prioritize the scheduled clusters in advance.
* Now: by user specified.

#### Corner case

Supposing the user has two clusters: member1 (priority=2) and member2 (priority=1)

In this scheduling type, a deployment is assigned to member2 when first propagation, for the reason of member1 is filtered
due to some reason. Then, this deployment scaling up replicas after member1 cluster recovered, at this time, 
should the new replicas be aggregated to previous member2 cluster or be assigned to member1 cluster?

If it is the former, it violates the user's priority, while if it is the latter, it violates the concept of aggregation.

### Solution two: Using multiple scheduling group

#### API changes

The current design of multiple scheduling groups cannot meet this requirement, mainly due to when choosing next clusterAffinity,
we treat all clusters in that group equally. However, in this requirement, it is expected to prioritize the clusters which
already appeared in the previous clusterAffinity.

Considering compatibility with previous multiple scheduling group, we introduce a new field `clusterAffinitiesOrder` to
`placement` of policy and binding. When it equals to value `sequential`, we would prioritize the clusters which already 
appeared in the previous clusterAffinity. When left empty, we treat all clusters in that group equally.

```go
type Placement struct {
    // ...
    // +optional
    ClusterAffinities []ClusterAffinityTerm `json:"clusterAffinities,omitempty"`

+   // ...
+   // +kubebuilder:validation:Enum=sequential
+   // +optional
+   ClusterAffinitiesOrder ClusterAffinitiesOrder `json:"clusterAffinitiesOrder,omitempty"`
}
```

#### User usage example

Supposing the user has two clusters (member1/member2), and he wants replicas to be aggregated first in member1 clusters,
then he can define the policy as following:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: default-pp
spec:
  #...
  placement:
    clusterAffinitiesOrder: sequential
    clusterAffinities:
    - affinityName: primary-clusters
      clusterNames:
      - member1
    - affinityName: backup-clusters
      clusterNames:
      - member1
      - member2
  #...
```

#### Controller logic changes

To meet the goal, when `clusterAffinitiesOrder` is `sequential`, the corresponding implementation needs to be modified:

* if each clusterAffinity is `Aggregated`: just like solution one, when doing `resortAvailableClusters`, we should 
prioritize the clusters which appeared in previous clusterAffinities in advance
* if each clusterAffinity is `Dynamic Weighted`: it will be more complicated, as we should re-write the 
logic of SpreadReplicasByTargetClusters.

> tip: multiple scheduling group never support static weighted scheduling type.

### Solution comparison

1) If the user doesn't consider clusters with same priority or the expected replicas assignment of same priority clusters is `Aggregated`:

Both solutions involve modifying the api, both can be implemented with similar implementation methods and costs,
but the api of solution one is more clear and friendly to users.

2) If the user consider clusters with same priority and the expected replicas assignment of same priority clusters is `Dynamic Weighted`:

Solution one not support it, and not intend to achieve it. Solution two can achieve it, despite the difficulty of implementation.

## Test Plan
