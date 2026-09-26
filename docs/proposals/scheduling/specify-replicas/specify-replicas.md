---
title: Specify replicas to schedule

authors:
- "@CharlesQQ"

reviewers:
- "@robot"
- TBD
approvers:
- "@robot"
- TBD

creation-date: 2024-07-08

# Specify replicas to schedule

## Summary
<!--
一种新的调度策略及其扩展API,允许调度器按照用户指定的副本数进行调度
-->

A new scheduling strategy and its extension API allow the scheduler to schedule according to the number of replicas specified by the user
## Motivation
<!--
在某些场景下,用户希望服务的副本数分布是自己直接指定的结果,而不是根据权重间接计算得出的,这样更加直观
-->
In some scenarios, users hope that the distribution of the number of replicas in a service can be directly specified by themselves, rather than calculated indirectly based on weights, which is more intuitive.
### Goals
<!--
- 扩展API,允许按照用户使用新的调度策略并指定副本数
- 调度器支持指定副本数调度的机制
-->
- Extended API to allow using new scheduling strategies and specifying the number of replicas by user
- The scheduler supports the mechanism of specifying the number of replicas for scheduling

### Non-Goals

## Proposal

### User Stories (Optional)

#### Story 1: Specify the number of replicas per cluster scenario
<!--
指定每个集群的副本数场景
当deployment第一次接入Karmada的时候, 用户希望服务的副本数分布不变, 例如:
迁移之前的服务实数分布是: `sum=7, cluster1=2,cluster2=5`
迁移之后的服务实数分布依然是: `sum=7, cluster1=2,cluster2=5`
-->

Specify the number of replicas per cluster scenario
When the deployment connects to Karmada for the first time, the user hopes that the distribution of the number of replicas in the service will remain unchanged, for example:
The service real number distribution before migration is: `sum=7, cluster1=2,cluster2=5`
The service real number distribution after migration is still: `sum=7, cluster1=2,cluster2=5`

#### Story 2: Specify the number of replicas for each Region scenario

<!--
指定每个Region的副本数场景
用户希望服务在每个Region的副本数是确定的; 如果有扩缩容,新增或者减少的副本数均匀分摊到region的每个集群中
例如: 
扩容场景
-->

Users hope that the number of replicas of the service in each Region is determined; if there is expansion or contraction, the new or reduced number of replicas will be evenly distributed to each cluster in the region.
For example:

increase replicas scenario
```
before scheduling-->  sum: 7, cluster1(RegionA): 2, cluster2(RegionB): 1, cluster3(RegionB): 4
increase replicas-->  sum:11,  RegionA: 3,  RegionB: 8
After scheduling--> sum:11, cluster1(RegionA): 3, cluster2(RegionB): 3, cluster3(RegionB): 5
```
reduce replicas scenario
```
before scheduling--->  sum: 7, cluster1(RegionA): 2, cluster2(RegionB): 1, cluster3(RegionB): 4
reduce replicas-->  sum:3,  RegionA: 1,  RegionB: 2
After scheduling--> sum:3, cluster1(RegionA): 1, cluster2(RegionB): 0, cluster3(RegionB): 2
```
#### Story 3:  Specify the schedulable region list
<!--
指定可调度的region列表
用户希望指定可调度的region列表和总实副本数,并不关心具体的分布; 如果有扩缩容,新增或者减少的副本数均匀分摊到region的每个集群中
扩容场景
-->

The user wants to specify the schedulable region list and total number of replicas, and does not care about the specific distribution; if there is expansion or contraction, the new or reduced number of replicas will be evenly distributed to each cluster in the region.

increase replicas scenario
```
before scheduling-->  sum: 7, cluster1(RegionA): 2, cluster2(RegionB): 1, cluster3(RegionB): 4
increase replicas-->  sum:11,  schedulable Region list: RegionA,RegionB
After scheduling--> sum:11, cluster1(RegionA): 3, cluster2(RegionB): 3, cluster3(RegionB): 5
```
reduce replicas scenario
```
before scheduling-->  sum: 7, cluster1(RegionA): 2, cluster2(RegionB): 1, cluster3(RegionB): 4
reduce replicas-->  sum:3,  schedulable Region list: RegionA,RegionB
After scheduling--> sum:3, cluster1(RegionA): 1, cluster2(RegionB): 0, cluster3(RegionB): 2
```

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

#### Risk
<!--
在Divided场景下,指定的总副本数之和可能和deployment.spec.replicas不相等
-->

In the Divided scenario, the sum of the specified total number of replicas may not be equal to deployment.spec.replicas

#### Mitigations
<!--
在调度开始的时候,如果是指定副本数的策略,先进行replicas数量的检查,不相等不进行调度,并打印相关的event事件
-->

At the beginning of scheduling, if it is a policy that specifies the number of replicas, the number of replicas will be checked first. If they are not equal, scheduling will not be performed, and the relevant events will be printed.
## Design Details

### Solution one: Extend API and Karmada-scheduler

#### API Change

##### PropagationPolicy/ClusterPropagationPolicy
<!--
通过扩展 PropagationPolicy/ClusterPropagationPolicy API,新增指定副本数的调度策略
-->
By extending the PropagationPolicy/ClusterPropagationPolicy API, a new scheduling policy is added to specify the number of replicas.
```go
package v1alpha1
// ReplicaDivisionPreference describes options of how replicas can be scheduled.
type ReplicaDivisionPreference string

const (
	// ReplicaDivisionPreferenceSpecified  divides replicas according to specified replicas.
	ReplicaDivisionPreferenceSpecified ReplicaDivisionPreference = "Specified"
)

// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// SpecifyPreference
	// If ReplicaDivisionPreference is set to "Specified", and SpecifyPreference is not set, scheduler will weight all clusters the same.
	// +optional
	SpecifyPreference *ClusterPreferences `json:"specifyPreference,omitempty"`
}

// ClusterPreferences describes weight for each cluster or for each group of cluster.
type ClusterPreferences struct {
	// StaticSpecifyList defines the static cluster replicas.
	// +optional
	StaticSpecifyList []StaticClusterSpecified `json:"staticSpecifyList,omitempty"`
}

// StaticClusterSpecified defines the static cluster weight.
type StaticClusterSpecified struct {
	// TargetCluster describes the filter to select clusters.
	// +required
	TargetCluster ClusterAffinity `json:"targetCluster"`

	// Replicas expressing the preference to the cluster(s) specified by 'TargetCluster'.
	// +kubebuilder:validation:Minimum=0
	// +required
	Replicas int64 `json:"replicas"`
}

```

##### ResourceBinding/ClusterResourceBinding
<!--
透传PropagationPolicy/ClusterPropagationPolicy 的扩展字段的内容
-->

Transparently transmit the contents of the extended fields of PropagationPolicy/ClusterPropagationPolicy
```go
package v1alpha1
// ReplicaDivisionPreference describes options of how replicas can be scheduled.
type ReplicaDivisionPreference string

const (
	// ReplicaDivisionPreferenceSpecified  divides replicas according to specified replicas.
	ReplicaDivisionPreferenceSpecified ReplicaDivisionPreference = "Specified"
)

// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// SpecifyPreference
	// If ReplicaDivisionPreference is set to "Specified", and SpecifyPreference is not set, scheduler will weight all clusters the same.
	// +optional
	SpecifyPreference *ClusterPreferences `json:"specifyPreference,omitempty"`
}

// ClusterPreferences describes weight for each cluster or for each group of cluster.
type ClusterPreferences struct {
	// StaticSpecifyList defines the static cluster replicas.
	// +optional
	StaticSpecifyList []StaticClusterSpecified `json:"staticSpecifyList,omitempty"`
}

// StaticClusterSpecified defines the static cluster weight.
type StaticClusterSpecified struct {
	// TargetCluster describes the filter to select clusters.
	// +required
	TargetCluster ClusterAffinity `json:"targetCluster"`

	// Replicas expressing the preference to the cluster(s) specified by 'TargetCluster'.
	// +kubebuilder:validation:Minimum=0
	// +required
	Replicas int64 `json:"replicas"`
}
```

#### User usage example
<!--
指定每个集群的副本数, member1: 2, member2: 2
-->

Specify the number of replicas in each cluster, member1: 2, member2: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Specified
      specifyPreference:
        staticSpecifyList:
        - targetCluster:
          clusterNames:
            - member1
          replicas: 2
        - targetCluster:
          clusterNames:
            - member2
          replicas: 2   
```
<!--
指定每个Region的副本数, RegionA: 2, RegionB: 2
-->

Specify the number of replicas for each Region, Region: 2, Region: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Specified
      specifyPreference:
        staticSpecifyList:
        - targetCluster:
          fieldSelector:
            matchExpressions:
              - key: region
                operator: In
                values:
                  - RegionA
          replicas: 2
        - targetCluster:
          fieldSelector:
            matchExpressions:
              - key: region
                operator: In
                values:
                  - RegionB
          replicas: 2
```
<!--
指定Region列表和总的副本数
-->
Specify the Region list and total number of replicas
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
  replicas: 4
```
### Solution Two: Extend API and customize-karmada-scheduler

#### API Change

##### PropagationPolicy/ClusterPropagationPolicy
<!--
通过扩展 PropagationPolicy/ClusterPropagationPolicy API,增加非特定的调度策略,由用户决定调度策略的名称和字段内容, karmada-scheduler不针对新增的策略实现对应的调度策略, 由自定义调度器实现
-->


By extending the PropagationPolicy/ClusterPropagationPolicy API, non-specific scheduling policies are added, and the user determines the name and field content of the scheduling policy. karmada-scheduler does not implement the corresponding scheduling policy for the new policy, but is implemented by a custom scheduler.
```go
package v1alpha1
// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// ExtendReplicaDivisionPreference represents the name of schedule replicas strategy
	// +optional
	ExtendReplicaDivisionPreference string `json:"extendReplicaDivisionPreference,omitempty"`

	// ExtendClusterPreferences if used for customize-karmada-scheduler
	// +optional
	ExtendClusterPreferences *ExtendClusterPreferences `json:"extendClusterPreferences,omitempty"`
}

// ExtendClusterPreferences  describes replicas schedule  strategy for each cluster or for each group of cluster.
type ExtendClusterPreferences struct {
	ExtendStaticList []ExtendStaticCluster `json:"extendStaticList,omitempty"`
}

// ExtendStaticCluster defines the static cluster replicas schedule strategy.
type ExtendStaticCluster struct {
	// TargetCluster describes the filter to select clusters.
	// +required
	TargetCluster ClusterAffinity `json:"targetCluster"`

	// ExtendFields expressing the preference to the cluster(s) specified by 'TargetCluster'.
	// +required
	ExtendFields map[string]int `json:"extendFields"`
}
```
##### ResourceBinding/ClusterResourceBinding
<!--
透传PropagationPolicy/ClusterPropagationPolicy 的扩展字段的内容
-->

Transparently transmit the contents of the extended fields of PropagationPolicy/ClusterPropagationPolicy
```go
package v1alpha1
// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// ExtendReplicaDivisionPreference represents the name of schedule replicas strategy
	// +optional
	ExtendReplicaDivisionPreference string `json:"extendReplicaDivisionPreference,omitempty"`

	// ExtendClusterPreferences if used for customize-karmada-scheduler
	// +optional
	ExtendClusterPreferences *ExtendClusterPreferences `json:"extendClusterPreferences,omitempty"`
}

// ExtendClusterPreferences  describes replicas schedule  strategy for each cluster or for each group of cluster.
type ExtendClusterPreferences struct {
	ExtendStaticList []ExtendStaticCluster `json:"extendStaticList,omitempty"`
}

// ExtendStaticCluster defines the static cluster replicas schedule strategy.
type ExtendStaticCluster struct {
	// TargetCluster describes the filter to select clusters.
	// +required
	TargetCluster ClusterAffinity `json:"targetCluster"`

	// ExtendFields expressing the preference to the cluster(s) specified by 'TargetCluster'.
	// +required
	ExtendFields map[string]int `json:"extendFields"`
}
```
#### User usage example

<!--
指定每个集群的副本数, member1: 2, member2: 2
-->

Specify the number of replicas per cluster, member 1: 2, member 2: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaSchedulingType: Divided
      extendReplicaDivisionPreference: SpecifiedClusters
      extendClusterPreferences:
        staticSpecifyList:
        - targetCluster:
          clusterNames:
            - member1
          extendFields:
            replicas: 2
        - targetCluster:
          clusterNames:
            - member2
          extendFields:
            replicas: 2  
```
<!--
指定每个Region的副本数, RegionA: 2, RegionB: 2
-->

Specify the number of replicas for each Region, RegionA: 2, RegionB: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: SpecifiedRegions
      specifyPreference:
        staticSpecifyList:
        - targetCluster:
          fieldSelector:
            matchExpressions:
              - key: region
                operator: In
                values:
                  - RegionA
          extendFields:
            replicas: 2
        - targetCluster:
          fieldSelector:
            matchExpressions:
              - key: region
                operator: In
                values:
                  - RegionB
          extendFields:
            replicas: 2
```
<!--
指定Region列表和总的副本数
-->

Specify the Region list and total number of replicas
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Regions
  replicas: 4
```

### Solution Three: Extend Annotation and customize-karmada-scheduler

#### API Change

##### PropagationPolicy/ClusterPropagationPolicy
<!--
通过扩展 PropagationPolicy/ClusterPropagationPolicy 的Annotation,新增指定副本数的调度策略, karmada 不需要做任何适配
-->

By extending the Annotation of PropagationPolicy/ClusterPropagationPolicy, a new scheduling strategy for specifying the number of copies is added, and karmada does not need to make any adaptations.

#### User usage example
<!--
指定每个集群的副本数, member1: 2, member2: 2
-->

Specify the number of replicas per cluster, member 1: 2, member 2: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
  annotation: '{"specifiedClusters": [{"name": "member1", "replicas": 2}, {"name": "member2", "replicas": 2}]}'
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2 
```
<!--
指定每个Region的副本数, RegionA: 2, RegionB: 2
-->

Specify the number of replicas for each Region, RegionA: 2, RegionB: 2
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
  annotation: '{"specifiedIdcs": [{"name": "RegionA", "replicas": 2}, {"name": "RegionB", "replicas": 2}]}'
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
```
<!--
指定Region列表和总的副本数
-->

Specify the Region list and total number of replicas
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
  annotation: '{"idcs": [{"name": "RegionA"}, {"name": "RegionB"}]}'
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
          - key: region
            operator: In
            values:
              - RegionA
              - RegionB
  replicas: 4
```

### Test Plan

TODO

## Alternatives