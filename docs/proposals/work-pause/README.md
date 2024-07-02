---
title: Support for cluster-level resource propagation pause and resume capabilities

authors:
- "@XiShanYongYe-Chang"

reviewers:
- "@robot"
- TBD

approvers:
- "@robot"
- TBD

creation-date: 2024-07-01

---

# Support for cluster-level resource propagation pause and resume capabilities

## Summary

<!--
Karmada 资源分发的流程经历了从 资源模板 到 ResourceBinding/ClusterResourceBinding 到 Work 再到 同步至成员集群 的阶段，这三个阶段目前来看可以说是一气呵成的，用户唯一可以在中间过程中介入的点是 ResourceBinding/ClusterResourceBinding 生成 Work 时会应用 OverridePolicy 资源，但这个点也需要用户提前创建好 OverridePolicy 资源，对于用户需要根据调度结果来定制 OverridePolicy 的情况就无能为力了。因此，如果能将资源的分发过程停下来，将为用户提供更强的控制力与灵活度。

本文提供了一种支持集群粒度的资源分发暂停与恢复能力，同时在方案设计过程中也充分考虑到了其他几种暂停能力，如：Policy资源绑定暂停、资源调度暂停等，使得方案整体考虑更加充分。
-->

The process of resource propagation in Karmada has gone through stages from the resource template to `ResourceBinding/ClusterResourceBinding`, to `Work`, and then to synchronization with member clusters. These three stages can be said to be seamless at present. The only point where users can intervene in the middle process is when `ResourceBinding/ClusterResourceBinding` generates `Work`, which applies the `OverridePolicy/ClusterOverridePolicy` resource. However, this point also requires users to have created the `OverridePolicy/ClusterOverridePolicy` resource in advance, and there is no way to handle the situation where users need to customize the `OverridePolicy/ClusterOverridePolicy` based on the scheduling results. Therefore, if the resource propagation process can be paused, it will provide users with greater control and flexibility.

This article provides a capability that supports the pause and resume of resource propagation at the cluster granularity, and during the proposal design process, it also fully considers several other pause capabilities, such as: Policy resource binding pause, resource scheduling pause, etc., making the overall proposal more comprehensive.

## Motivation

<!--
当前 Karmada 系统对资源的变化较为敏感，当 资源模板/ResourceBinding/OverridePolicy 发生变化时，将会立即同步至成员集群中。在大多数情况下这是预期的行为，但仍存在某些情况，用户不希望这样的变化立即同步至成员集群中，更进一步说，用户希望能够由自己来控制上述资源变化的同步时机，为联邦系统的开发与运维带来更多的可能性。
-->

The current Karmada system is quite sensitive to changes in resources. When there are changes in the resource template, `PropagationPolicy/ClusterPropagationPolicy`, or `OverridePolicy/ClusterOverridePolicy`, they will be immediately synchronized to the member clusters. In most cases, this is the expected behavior, but there are still some situations where users do not want such changes to be immediately synchronized to the member clusters. To go further, users would like to control the timing of the synchronization of the above resource changes themselves, bringing more possibilities to the development and operation and maintenance of the federated system.

### Goals

<!--
- 提供资源分发暂停的能力；
- 提供资源分发恢复的能力；
- 提供集群粒度的资源分发暂停与恢复的编排能力；
-->

- Provide the capability to pause the propagation of resources;
- Provide the capability to resume the propagation of resources;
- Provide orchestration capabilities for pausing and resuming the propagation of resources at the cluster granularity.

### Non-Goals

<!--
- 提供开箱即用的适用于多集群场景的金丝雀发布、滚动式发布、蓝绿发布等能力；
- 提供自动化的按照指定集群顺序的应用负载发布；

> 待 https://github.com/karmada-io/karmada/pull/5056 提案中设计的特性实现之后，我们可以逐一适配不同发布，并提供最佳实践
-->

- Provide out-of-the-box capabilities suitable for multi-cluster scenarios such as canary releases, rolling releases, and blue-green deployments;
- Provide automated workload releasing according to a specified cluster sequence.

## Proposal

### User Stories (Optional)

#### Story 1

<!--
作为一名运维者，我希望当资源分发产生调度结果之后，分发流程能够暂停下来，这样我能根据调度结果创建 OverridePolicy 资源，对同步至不同成员集群的资源进行差异化配置。相关issue：https://github.com/karmada-io/karmada/issues/1567 和 https://github.com/karmada-io/karmada/issues/4421。

在用户升级应用之前，将该应用相关的 Works 暂停，然后根据应用的调度结果，生成或修改 OverridePolicy 来将应用注解中的 partition 进行拆分。

这个用例其实与小红书公司的用例 https://github.com/karmada-io/karmada/pull/5056 有点像，区别是这个用例中需要处理的 partition 字段是位于 annotation 中的。

https://github.com/karmada-io/karmada/issues/4421 与 https://github.com/karmada-io/karmada/issues/1567 相关，前者其实是后者的一个具体解，但前者描述的实际上也是一个 Y 问题，X 问题其实是“在联邦层面，如果保证Deployment应用发布与单集群保持一致”。

Kubernetes中单个Deployment资源实际上不支持金丝雀发布/分批次发布，issue中描述的金丝雀发布是陌陌公司选择的金丝雀发布实践中的一种，陌陌针对该实践进行了额外的开发工作，使其能够满足单个Deployment资源即可进行金丝雀发布。
-->

As an operator, I hope that after the resource propagation generates a scheduling result, the propagation process can be paused, so that I can create an `OverridePolicy` resource based on the scheduling result to apply differentiated configurations to the resources synchronized to different member clusters.

#### Story 2

<!--
作为一名运维者，当 Karmada 控制面与成员集群争夺资源的控制权时，会出现资源反复被更新的情况，我希望能够暂停该资源向成员集群进行同步的流程，从而能够帮助我快速进行问题定位。

PS：为什么不是直接暂停 ResourceBinding 和 Work 的生成呢？这是因为生成 Work 表示 Karmada 控制面中的工作已经全部完成，只剩资源同步了，这样可以更好的帮忙用户进行紧急维护或故障排除。
-->

As an operator, when the Karmada control plane competes with member clusters for control of resources, there is a situation where resources are repeatedly updated. I hope to pause the process of synchronizing the resource to the member clusters, which would help me quickly locate the problem.

#### Story 3

<!--
作为一名管理者，我想在资源同步到成员集群之前，将资源分发暂停，然后执行多个修补操作后恢复资源的分发，这样避免触发不必要的分发操作。参考相关 Issue：https://github.com/karmada-io/karmada/issues/4688 中描述的用例2。

类似于 deployment pause 字段，使得用户能够在暂停和恢复执行之间应用多个修补程序，而不会触发不必要的分发操作。

这个用例可以参考 https://github.com/karmada-io/karmada/issues/4688 中描述的用例2，当执行完这些步骤之后，https://github.com/karmada-io/karmada/issues/4688#issuecomment-2065332647，用户应该会需要执行多个操作，如：修改 Deployment 副本数为3，修改 PropagationPolicy 将 cluster-blue 移除 clusteraffinify 中，删除 OverridePolicy 等，当所有这些操作完成后，再将 work 从暂停中恢复，这样不会因中间的某个操作导致资源被同步下去，从而对用户业务产生影响。
-->

As a manager, I want to suspend the propagation of resources before they are synchronized to member clusters. Then, after performing multiple patching operations, I would resume the propagation of resources. This approach avoids triggering unnecessary propagation actions. Refer to use case 2 described in the relevant issue: https://github.com/karmada-io/karmada/issues/4688.

#### Story 4

<!--
作为一名管理者，我希望在应用版本发布的过程中，指定集群的发布顺序，当前一个集群发布成功之后，再进行下一个集群的发布。
-->

As a manager, I wish to specify the release order of clusters during the application version rollout. I want to proceed with the release to the next cluster only after the release has successfully been deployed to the current one.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

1. When the workload is in a paused state, even if the user has enabled the Failover feature gate, the fault migration will be paused until the user cancels the pause.

## Design Details

<!--
当前，我提出了两种方案来实现当前特性，请大家给出一些具体的意见，也欢迎大家提出新的方案。让我们一同来选择一种方案，推进该特性向前发展。
-->

Currently, I've proposed two approaches to implement the current feature, and I'd appreciate concrete feedback from all of you. I also welcome suggestions for alternative solutions. Let's collaborate to select an approach and move forward with developing this feature.

### Solution one

<!--
在 PP 上增加字段来描述各种暂停需求，包括：PP功能暂停、分发功能暂停、调度功能暂停、TODO：指定集群暂停。

对于 Binding 来说，暂停包含：分发功能暂停、调度功能暂停、TODO：指定集群暂停；
对于 Work 来说，暂停包含：分发功能暂停。

通过扩展 PropagationPolicy/ClusterPropagationPolicy API，新增相关的暂停结构体，并将关键的暂停信息透传到 ResourceBinding/ClusterResourceBinding 资源，进而透传至 Work 资源上，Karmada-controller-manager 组件中的控制器会根据 Work 资源中的暂停信息来决定是否将资源同步至目标成员集群中去。
-->

By extending the `PropagationPolicy/ClusterPropagationPolicy` API, we introduce new suspension structures and pass critical suspension information through to the `ResourceBinding/ClusterResourceBinding` resources, which in turn are propagated to the `Work` resources. The controller within the `Karmada-controller-manager` component will then determine whether to synchronize resources to target member clusters based on the suspension information found in the `Work` resources.

#### API Change

```go
// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
	...

	// SuspendStrategy is used to specify the suspension strategy.
	SuspendStrategy *SuspendStrategy `json:"suspendStrategy,omitempty"`
}

type SuspendStrategy struct {
	// ResourceMatchSuspension is used to specify the suspension condition for resource
	// matching under the current Policy, set to false indicates the suspension is canceled.
	// Default value is false.
	// Note: The extension design is not a focal point of the current proposal.
	ResourceMatchSuspension *bool `json:"ResourceMatchSuspension,omitempty"`

	// ScheduleSuspension is used to specify the suspension scheduling of resources
	// matched by the current policy, set to false indicates the suspension is canceled.
	// Default value is false.
	// Note: The extension design is not a focal point of the current proposal.
	ScheduleSuspension *bool `json:"scheduleSuspension,omitempty"`

	// DistributionSuspendClusters is used to specify a list of clusters where resource
	// synchronization needs to be paused. Clusters in this list indicate that resource
	// synchronization for targeted resources will be suspended to these clusters.
	// Clusters removed from this list indicate that resource synchronization should be resumed.
	// If there is a cluster element with an asterisk "*" in the list, it indicates that
	// the synchronization of resources to all clusters is paused for the matched resources.
	DistributionSuspendClusters []string `json:"distributionSuspendClusters,omitempty"`
}
```

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ... 

	SuspendStrategy *SuspendStrategy `json:"suspendStrategy,omitempty"`
}

type SuspendStrategy struct {
	// ScheduleSuspension is used to specify the suspension scheduling of resources
	// matched by the current policy, set to false indicates the suspension is canceled.
	// Default value is false.
	// Note: The extension design is not a focal point of the current proposal.
	ScheduleSuspension *bool `json:"scheduleSuspension,omitempty"`

	// DistributionSuspendClusters is used to specify a list of clusters where resource
	// synchronization needs to be paused. Clusters in this list indicate that resource
	// synchronization for targeted resources will be suspended to these clusters.
	// Clusters removed from this list indicate that resource synchronization should be resumed.
	// If there is a cluster element with an asterisk "*" in the list, it indicates that
	// the synchronization of resources to all clusters is paused for the matched resources.
	DistributionSuspendClusters []string `json:"distributionSuspendClusters,omitempty"`
}
```

```go
// WorkSpec defines the desired state of Work.
type WorkSpec struct {
	...

	// DistributionSuspension indicates a pause in synchronizing the resource
	// templates in the current Work to the target cluster.
	// It sets to false indicates the suspension is canceled.
	// Default value is false.
	DistributionSuspension *bool `json:"distributionSuspension,omitempty"`
}
```

#### User usage example

<!--
用户在 PropagationPolicy 中设置 Deployment(default/nginx) 资源分发暂停，当用户修改了该 deploy 之后，Karmada 并不会将变化同步至所有目标集群中。
-->

Users set the Deployment(default/nginx) resource distribution to be paused in the `PropagationPolicy`. When users modify this deploy, Karmada will not synchronize the changes to all target clusters.

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
  suspendStrategy:
    distributionSuspendClusters:
      - *
```

<!--
当用户在 PropagationPolicy 中取消暂停后，将会恢复 Deployment(default/nginx) 在所有集群中的资源同步操作。
-->

When users cancel the pause in the PropagationPolicy, it will resume the resource synchronization operation of Deployment(default/nginx) in all clusters.

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
```

<!--
又例如，Deployment(default/nginx) 被分发到了 member1, member2, member3 集群中，当用户按照如下文件进行配置，并更新 Deployment(default/nginx) 资源的镜像，发布一个新的版本时，新的资源版本将
仅被同步至 member1 集群中去：
-->

For instance, if the Deployment(default/nginx) has been propagated to the clusters member1, member2, and member3, and when a user configures according to the following file and updates the image of the Deployment(default/nginx) resource to release a new version, the new version of the resource will only be synchronized to the member1 cluster.


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
  suspendStrategy:
    distributionSuspendClusters:
      - member2
      - member3
```

<!--
当用户确定 member1 中的 deployment 资源发布完成时，修改同步配置如下时，即可继续将新的资源版本同步至 member2 集群中：
-->

When the user confirms that the deployment resource in member1 has been successfully published, and modifies the synchronization configuration as follows, the new version of the resource can then be synchronized to the member2 cluster:

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
  suspendStrategy:
    distributionSuspendClusters:
      - member3
```

<!--
如果在将新的资源版本同步至 member1 成员集群后，发现该应用出现问题，导致发布失败，用户可以在 Karmada 控制面中将 Deployment 资源版本回退，从而保护 member2、member3集群中的业务不受影响。
-->

If, after synchronizing the new version of the resource to the member1 cluster, issues are found with the application that cause the release to fail, the user can roll back the version of the Deployment resource in the Karmada control plane, thereby protecting the business in the member2 and member3 clusters from being affected.

Q&A: How to remove the annotation `propagation.karmada.io/instruction` from Work? It can be carried by the `DistributionSuspension` field in the Work.

### Solution two

<!--
新增一个策略配置型 CRD，用户通过配置该 CRD 的 CR 来决定暂停或者恢复指定资源模板向目标成员集群的同步情况。
-->

Add a policy configuration type CRD, and users can decide to pause or resume the synchronization of specified resource templates to the target member clusters by configuring the CR of this CRD.

#### API Change

```go
type SynchronizationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SynchronizationSpec `json:"spec"`
}

type SynchronizationSpec struct {
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// DistributionSuspendClusters is used to specify a list of clusters where resource
	// synchronization needs to be paused. Clusters in this list indicate that resource
	// synchronization for targeted resources will be suspended to these clusters.
	// Clusters removed from this list indicate that resource synchronization should be resumed.
	// If there is a cluster element with an asterisk "*" in the list, it indicates that
	// the synchronization of resources to all clusters is paused for the matched resources.
	DistributionSuspendClusters []string `json:"distributionSuspendClusters,omitempty"`
}
```

```go
// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
	...

	// ResourceMatchSuspension is used to specify the suspension condition for resource
	// matching under the current Policy, set to false indicates the suspension is canceled.
	// Default value is false.
	// Note: The extension design is not a focal point of the current proposal.
	ResourceMatchSuspension *bool `json:"ResourceMatchSuspension,omitempty"`
}
```

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
	...

	// ScheduleSuspension is used to specify the suspension scheduling of resources
	// matched by the current policy, set to false indicates the suspension is canceled.
	// Default value is false.
	// Note: The extension design is not a focal point of the current proposal.
	ScheduleSuspension *bool `json:"scheduleSuspension,omitempty"`
}
```
<!--
在 WorkSpe 中新增 Pause字段，来指示是否暂停将当前 Work 中的资源模板同步至目标集群。对于出现在 SynchronizationPolicy 中的资源，对应集群 execution 命名空间下的 Work 将会被设置为暂停。
-->

```go
// WorkSpec defines the desired state of Work.
type WorkSpec struct {
	...

	// DistributionSuspension indicates a pause in synchronizing the resource
	// templates in the current Work to the target cluster.
	// It sets to false indicates the suspension is canceled.
	// Default value is false.
	DistributionSuspension *bool `json:"distributionSuspension,omitempty"`
}
```

#### User usage example

<!--
当用户按照如下 YAML 文件将 SynchronizationPolicy 资源应用到环境中时，将会暂停 Deployment(default/nginx) 在所有集群中的资源同步操作：
-->

When users apply the SynchronizationPolicy resource to the environment according to the following YAML file, it will pause the resource synchronization operation of Deployment(default/nginx) in all clusters:


```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: SynchronizationPolicy
metadata:
  name: nginx-deploy-synchronization
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  distributionSuspendClusters:
    - *
```

<!--
当用户删除 SynchronizationPolicy 资源中的 distributionSuspendClusters 字段或删除该 SynchronizationPolicy 资源时，将会恢复 Deployment(default/nginx) 在所有集群中的资源同步操作。
-->

When users delete the `distributionSuspendClusters` field in the SynchronizationPolicy resource or delete the SynchronizationPolicy resource itself, it will resume the resource synchronization operation of Deployment(default/nginx) in all clusters.

<!--
又例如，Deployment(default/nginx) 被分发到了 member1, member2, member3 集群中，当用户按照如下文件进行配置，并更新 Deployment(default/nginx) 资源的镜像，发布一个新的版本时，新的资源版本将
仅被同步至 member1 集群中去：
-->

For instance, if the Deployment(default/nginx) has been propagated to the clusters member1, member2, and member3, and when a user configures according to the following file and updates the image of the Deployment(default/nginx) resource to release a new version, the new version of the resource will only be synchronized to the member1 cluster.


```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: SynchronizationPolicy
metadata:
  name: nginx-deploy-synchronization
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  distributionSuspendClusters:
    - member2
    - member3
```

<!--
当用户确定 member1 中的 deployment 资源发布完成时，修改同步配置如下时，即可继续将新的资源版本同步至 member2 集群中：
-->

When the user confirms that the deployment resource in member1 has been successfully published, and modifies the synchronization configuration as follows, the new version of the resource can then be synchronized to the member2 cluster:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: SynchronizationPolicy
metadata:
  name: nginx-deploy-synchronization
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  distributionSuspendClusters:
    - member3
```

<!--
如果在将新的资源版本同步至 member1 成员集群后，发现该应用出现问题，导致发布失败，用户可以在 Karmada 控制面中将 Deployment 资源版本回退，从而保护 member2、member3集群中的业务不受影响。
-->

If, after synchronizing the new version of the resource to the member1 cluster, issues are found with the application that cause the release to fail, the user can roll back the version of the Deployment resource in the Karmada control plane, thereby protecting the business in the member2 and member3 clusters from being affected.

Q&A: How to remove the annotation `propagation.karmada.io/instruction` from Work? It can be carried by the `DistributionSuspension` field in the Work.

<!--
待决策点：
1、资源创建操作是否需要暂停？创建操作包括调度结果发生变化，在新的集群中资源分发。
1、资源删除操作是否需要暂停？资源删除分两种，一种是资源模板被删除了，另一种是调度结果发生的变化，之前调度的集群上的实例现在不需要了。
2、资源同步暂停是否影响资源的状态收集操作？
-->

Decision Points:
1. Should the resource creation operation be paused? The creation operation includes changes in the scheduling result, where resources are distributed to new clusters.
2. Should the resource deletion operation be paused? There are two types of resource deletion: one is when the resource template is deleted, and the other is when there is a change in the scheduling result, and the instances on the previously scheduled clusters are no longer needed.
3. Does the pause of resource synchronization affect the status collection operation of the resources?

### Test Plan

TODO

## Alternatives
