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

To support this scenario, we propose a new scheduling strategy that allows users to customize the priority of clusters,
ensuring that replicas are preferentially allocated to higher-priority clusters.

## Motivation

Karmada currently supports three `Divided` type scheduling strategies, 
these are `Dynamic Weighted`, `Static Weight` and `Aggregated`.
However, the current strategies are still not satisfied in certain user scenarios.

<!-- 例如，假设用户有多个集群，用户希望副本优先分配到他偏好的集群，只有当他偏好的集群资源不足时，再分配到其他的集群 -->

For example, the user has multiple clusters, and he wants to use some preferred clusters first.
Only if these preferred clusters are not sufficiently allocated, the remaining clusters are used.

From the perspective of a deployment, it should be like this:
* When first propagation, prioritize allocation to preferred clusters as much as possible.
* When replicas scaling up, prioritize replicas expansion in preferred clusters (existing replicas remain unchanged).
* When replicas scaling down, prioritize replicas reduction in non-preferred clusters.

### Goals

<!-- 本文旨在提供一种调度策略，让用户可以自定义集群的优先级，让副本优先分配到优先级高的集群 -->

* Aims to providing a scheduling strategy that allows users to customize the priority of clusters, 
  ensuring that replicas are preferentially allocated to higher-priority clusters.

### Non-Goals

<!-- 
用户对相同优先级集群怎么分配没有特殊诉求，合理即可，
本文对相同优先级集群采取聚合的策略分配(将副本划分到尽可能少的集群)，不过多发散到其他复杂场景 
-->

* Users have no specific requests for allocating clusters with the same priority, as long as it is reasonable. 
  Here uses an `Aggregated` strategy to treat that case, dividing replicas into as few clusters as possible, avoiding more complex scenarios.

## Proposal

### User Stories (Optional)

#### Story 1

As a cluster administrator, I manage two clusters, one is a cheap private cloud cluster and the other is an expensive public cloud cluster.

I hope that the user's replicas will give priority to the specified cheaper cluster, whether it is first propagation or scaling up.
Only when the cheaper cluster has insufficient capacity, the expensive cluster will be used, so that I can save more costs.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

<!-- 
本段落提出一种名为 PriorityAggregated 的新的调度策略，在此调度策略下：
* 用户可以在 Policy 中对这个集群指定优先级
* 对于不同优先级的集群，副本优先分配到高优先级的集群，只有在高优先级集群资源不足后，副本才会分配给低优先级集群
* 对于相同优先级的集群，将副本划分到尽可能少的集群
-->

This section introduces a new scheduling strategy called `PriorityAggregated`:

* Users can specify the priority of clusters in the policy.
* Replicas are allocated to higher-priority clusters first, 
  only when resources are insufficient in those clusters will replicas be assigned to lower-priority clusters.
* For clusters with the same priority, replicas are divided into as few clusters as possible.

#### API changes

add a new `ReplicaDivisionPreference` type named `PriorityAggregated`

```go
// ReplicaDivisionPreference describes options of how replicas can be scheduled.
type ReplicaDivisionPreference string

const (
    ReplicaDivisionPreferenceAggregated ReplicaDivisionPreference = "Aggregated"
    ReplicaDivisionPreferenceWeighted ReplicaDivisionPreference = "Weighted"
+	// ReplicaDivisionPreferencePriorityAggregated assigns replicas to higher-priority clusters first, 
+	// then to lower-priority clusters if resources are insufficient.
+	ReplicaDivisionPreferencePriorityAggregated ReplicaDivisionPreference = "PriorityAggregated"
)
```

add a new field `priorityPreference` to `ReplicaSchedulingStrategy` of the PropoagationPolicy, as following:

```go
// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// ReplicaDivisionPreference determines how the replicas is divided
	// when ReplicaSchedulingType is "Divided". Valid options are Aggregated and Weighted.
	// "Aggregated" divides replicas into clusters as few as possible,
	// while respecting clusters' resource availabilities during the division.
	// "Weighted" divides replicas by weight according to WeightPreference.
+	// "PriorityAggregated" assigns replicas to higher-priority clusters first, 
+	// then to lower-priority clusters if resources are insufficient.
-	// +kubebuilder:validation:Enum=Aggregated;Weighted
+	// +kubebuilder:validation:Enum=Aggregated;Weighted;PriorityAggregated
	// +optional
	ReplicaDivisionPreference ReplicaDivisionPreference `json:"replicaDivisionPreference,omitempty"`

	// WeightPreference describes weight for each cluster or for each group of cluster
	// If ReplicaDivisionPreference is set to "Weighted", and WeightPreference is not set, scheduler will weight all clusters the same.
	// +optional
	WeightPreference *ClusterPreferences `json:"weightPreference,omitempty"`

+	// PriorityPreference describes allocation priority for each cluster or for each group of cluster
+	// If ReplicaDivisionPreference is set to "PriorityAggregated", and PriorityPreference is not set, scheduler will prioritize all clusters equally.
+	// +optional
+	PriorityPreference *PriorityPreference `json:"priorityPreference,omitempty"`
}
```

the newly added type `PriorityPreference` is defined as following:

```go
// PriorityPreference describes allocation priority for each cluster or for each group of cluster
type PriorityPreference struct {
    // PreferredPriorityList defines the clusters' preferred allocation priority.
    // +optional
    PreferredPriorityList []ClusterPreferredPriority `json:"preferredPriorityList"`
}

// ClusterPreferredPriority defines the clusters' preferred allocation priority.
type ClusterPreferredPriority struct {
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

#### Usage Example

Supposing the user has four clusters (member1/member2/member3/member4), 
and he wants replicas to be aggregated first in member1/member2 clusters, then he can define the policy as following:

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
        - member3
        - member4
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: PriorityAggregated
      priorityPreference:
        preferredPriorityList:
          - targetCluster:
              clusterNames:
                - member1
                - member2
            priority: 2
          - targetCluster:
              clusterNames:
                - member3
                - member4
            priority: 1
```

As you see, the user specifies the cluster aggregation priority through `spec.placement.replicaScheduling.priorityPreference`
field, the member1/member2 cluster has a higher priority and will be aggregated first.

#### Use case and its behavior

Basing on above usage example, assuming the max available replicas for each cluster is `10`.

Then under the `PriorityAggregated` scheduling strategy, the actual allocation results for different desired replicas are as follows:

| cluster \ sum replicas  | 8 | 16 | 28 | 36 |
|-------------------------|---|----|----|----|
| member1 (high proirity) | 8 | 8  | 10 | 10 |
| member2 (high proirity) | 0 | 8  | 10 | 10 |
| member3 (low proirity)  | 0 | 0  | 8  | 8  |
| member4 (low proirity)  | 0 | 0  | 0  | 8  |

#### Controller logic changes

The logic of the scheduler refers to the `Aggregated` scheduling. Before `selectClusters` step, it will still
go through `filter` and `score` plug-in, however:

* The `filter` plug-in will take effect, and the feasible clusters will be the filtered clusters basing on user provided clusters.
* The `score` plug-in will not take effect, and the clusters will finally be sorted according to the priority specified by the user.

As for `assignReplicas` step, the detail assign methods remain unchanged, but the order of the input available clusters is different.

* Previous: first sort by `MaxAvailableReplicas`, and then resort to prioritize the scheduled clusters in advance.
* Now: by user specified.

<!-- 
当前 Aggregated 调度策略的 `assignReplicas` 实现存在一些可优化点，例如副本分配到两个集群，期望的结果是先把第一个集群装满，
剩下的副本分配到第二个集群，这种更符合 Aggregated 的语义；但当前实现是依据最大可用副本数均摊到两个集群。

如果我们考虑将当前 Aggregated 的实现修改为上述第一种结果，那么新增的 PriorityAggregated 策略实现起来就很简单了，
直接复用 Aggregated 策略的 `assignReplicas` 代码，只需在进入 `assignReplicas` 分配函数前，对集群按照指定的优先级排个序。
因此，PriorityAggregated 相当于一种特殊的、定制化的 Aggregated 策略。
--> 

<!-- 其他方案 -->
## Alternatives

<!-- 
将集群优先级当作比调度策略更高维度的字段，集群优先级可以搭配静态权重、动态权重、聚合等调度策略使用：
* 对于相同优先级的集群，遵循原调度策略分发，逻辑不变
* 对于不同优先级的集群，不管调度策略如何，都优先分配给高优先级的集群
-->

Cluster priority is treated as a higher-dimensional field than the scheduling strategy and can be combined with static and dynamic weights, as well as aggregated:
* Same-priority clusters follow the original scheduling strategy.
* Higher-priority clusters are allocated first, regardless of the strategy.

### implicitly declare cluster priority

<!-- 利用多调度组隐式表达集群优先级，如下例中member1/member2优先级更高，当选到第二个调度组时，依然优先分配给member1/member2集群 -->

```yaml
clusterAffinities:
  - affinityName: primary-clusters
    clusterNames:
      - member1
      - member2
  - affinityName: backup-clusters
    clusterNames:
      - member1
      - member2
      - member3
      - member4
replicaScheduling:
  replicaSchedulingType: Divided
  replicaDivisionPreference: Aggregated
```

### explicitly declare cluster priority

<!-- 新增字段显式声明集群的优先级 -->

```yaml
clusterAffinity:
  clusterNames:
    - member1
    - member2
    - member3
    - member4
clusterPriority:
  preferredPriorityList:
    - targetCluster:
        clusterNames:
          - member1
          - member2
      priority: 2
    - targetCluster:
        clusterNames:
          - member3
          - member4
      priority: 1
replicaScheduling:
  replicaSchedulingType: Divided
  replicaDivisionPreference: Aggregated
```

Discussion results: this kind of `cluster priority` dimension has a significant impact on the current scheduling strategy, 
which is not being considered now.
