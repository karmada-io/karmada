---
title: Support for cluster-level resource propagation pause and resume capabilities

authors:
- "@XiShanYongYe-Chang"

reviewers:
- "@a7i"
- "@CharlesQQ"
- "@chaunceyjiang"
- "@Monokaix"
- "@RainbowMango"
- "@Vacant2333"
- "@whitewindmills"

approvers:
- "@kevin-wangzefeng"
- "@RainbowMango"

creation-date: 2024-07-01

---

# Support for cluster-level resource propagation pause and resume capabilities

## Summary

The process of resource propagation in Karmada has gone through stages from the Resource Template to `ResourceBinding/ClusterResourceBinding`, to `Work`, and then to synchronization with member clusters. These three stages can be said to be seamless at present. The only point where users can intervene in the middle process is when `ResourceBinding/ClusterResourceBinding` generates `Work`, which applies the `OverridePolicy/ClusterOverridePolicy` resource. However, this point also requires users to have created the `OverridePolicy/ClusterOverridePolicy` resource in advance, and there is no way to handle the situation where users need to customize the `OverridePolicy/ClusterOverridePolicy` based on the scheduling results. Therefore, if the resource propagation process can be paused, it will provide users with greater control and flexibility.

This article provides a capability that supports the pause and resume of resource propagation at the cluster granularity.

## Motivation

The current Karmada system is quite sensitive to changes in resources. When there are changes in the Resource Template, `PropagationPolicy/ClusterPropagationPolicy`, or `OverridePolicy/ClusterOverridePolicy`, they will be immediately synchronized to the member clusters. In most cases, this is the expected behavior, but there are still some situations where users do not want such changes to be immediately synchronized to the member clusters. To go further, users would like to control the timing of the synchronization of the above resource changes themselves, bringing more possibilities to the development and operation and maintenance of the federated system.

### Goals

- Provide the capability to pause the propagation of resources;
- Provide the capability to resume the propagation of resources;
- Provide orchestration capabilities for pausing and resuming the propagation of resources at the cluster granularity.

### Non-Goals

- Provide out-of-the-box capabilities suitable for multi-cluster scenarios such as canary releases, rolling releases, and blue-green deployments;
- Provide automated workload releasing according to a specified cluster sequence.

## Proposal

### User Stories (Optional)

#### Story 1

As an operator, when the Karmada control plane competes with member clusters for control of resources, there is a situation where resources are repeatedly updated. I hope to pause the process of synchronizing the resource to the member clusters, which would help me quickly locate the problem.

#### Story 2

As a manager, I wish to specify the release order of clusters during the application version rollout. I want to proceed with the release to the next cluster only after the release has successfully been deployed to the current one.

#### Story 3

During the business deployment, by setting the partition field in some workload-type resources, such as StatefulSet, you can specify the update of a subset of instances, achieving a certain amount of canary deployment.

As an operator, when I specify to update a portion of instances for a Deployment resource in the federated control plane, it is necessary to split the number of replicas to be updated when distributing to different clusters. I would like to pause the propagation process of the Deployment resource when calculating the split number for different clusters. After the calculation is complete and the results are applied to the system, I would then resume the propagation process of the Deployment resource to achieve the goal of canary deployment.

Related issue: https://github.com/karmada-io/karmada/issues/1567, https://github.com/karmada-io/karmada/issues/4421.

#### Story 4

As an operator, I hope that when making business changes in the federated control plane, the changes are not directly synchronized to the member clusters, but are instead retained within the control plane. This allows me to check the changes in the control plane's resources, such as the contents of ResourceBinding, Work, and other resources, to determine if the system is executing as expected. Once I am certain that everything is in order, I can then resume the business changes and synchronize the results to the member clusters.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

1. When the workload is in a paused state, even if the user has enabled the Failover feature gate, the fault migration will be paused until the user cancels the pause.

## Design Details

By extending the `PropagationPolicy/ClusterPropagationPolicy` API, we introduce new suspension structures and pass critical suspension information through to the `ResourceBinding/ClusterResourceBinding` resources, which in turn are propagated to the `Work` resources. The controller within the `Karmada-controller-manager` component will then determine whether to synchronize resources to target member clusters based on the suspension information found in the `Work` resources.

### API Change

#### PropagationPolicy/ClusterPropagationPolicy

```go
// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
	...

	// Suspension declares the policy for suspending different aspects of propagation.
	// nil means no suspension. no default values.
	// +optional
	Suspension *Suspension `json:"suspension,omitempty"`
}

// Suspension defines the policy for suspending different aspects of propagation.
type Suspension struct {
	// SuspendScheduling controls whether scheduling should be suspended.
	// +optional
	// Note: Postpone this until there is a solid use case.
	// SuspendScheduling *bool `json:"suspendScheduling,omitempty"`

	// SuspendDispatching controls whether dispatching should be suspended.
	// nil means not suspend.
	// Note: true means stop propagating to all clusters. Can not co-exist
	// with DispatchingOnClusters which is used to suspend particular clusters.
	// +optional
	SuspendDispatching *bool `json:"suspendDispatching,omitempty"`

	// SuspendDispatchingOnClusters declares a list of clusters to which
	// the dispatching should be suspended.
	// Note: Can not co-exist with Dispatching which is used to suspend all.
	// +optional
	SuspendDispatchingOnClusters *SuspendClusters `json:"suspendDispatchingOnClusters,omitempty"`
}

// SuspendClusters represents a group of clusters that should be suspended from propagating.
// Note: No plan to introduce the label selector or field selector to select clusters yet, as it
// would make the system unpredictable.
type SuspendClusters struct {
	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}
```

#### ResourceBinding/ClusterResourceBinding

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ...

	// Suspension declares the policy for suspending different aspects of propagation.
	// nil means no suspension. no default values.
	// +optional
	Suspension *Suspension `json:"suspension,omitempty"`
}
```

#### Work

```go
// WorkSpec defines the desired state of Work.
type WorkSpec struct {
	...

	// SuspendDispatching controls whether dispatching should
	// be suspended, nil means not suspend.
	// Note: true means stop propagating to all clusters.
	// +optional
	SuspendDispatching *bool `json:"suspendDispatching,omitempty"`
}
```

A new ConditionType called `Dispatching` is introduced to describe the distribution suspension status of the Work resource. Meanwhile, the system will also report events to record the suspension of resource distribution.

### User usage example

Users set the Deployment(default/nginx) resource distribution to be paused in the `PropagationPolicy`. When users modify this deployment, Karmada will not synchronize the changes to all target clusters.

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
        - member3
  suspension:
    suspendDispatching: true
```

At this point, all `Work` resources related to this `Deployment` on the Karmada control plane will be in a paused state, taking the `Work` resource corresponding to the member1 cluster as an example:

```yaml
apiVersion: work.karmada.io/v1alpha1
kind: Work
metadata:
  name: nginx-xxx
  namespace: karmada-es-member1
spec:
  workload:
    manifests:
      - xxx
  suspendDispatching: true
status:
  conditions:
    - lastTransitionTime: "2024-07-01T08:33:28Z"
      message: Work dispatching is in a suspended state.
      reason: SuspendDispatching
      status: "True"
      type: Dispatching
```

When users cancel the pause in the PropagationPolicy, it will resume the resource synchronization operation of Deployment(default/nginx) in all clusters.

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
        - member3
  suspension:
    suspendDispatchingOnClusters:
      clusterNames:
        - member2
        - member3
```

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
        - member3
  suspension:
    suspendDispatchingOnClusters:
      clusterNames:
        - member3
```

If, after synchronizing the new version of the resource to the member1 cluster, issues are found with the application that cause the release to fail, the user can roll back the version of the Deployment resource in the Karmada control plane, thereby protecting the business in the member2 and member3 clusters from being affected.

Q&A: How to remove the annotation `propagation.karmada.io/instruction` from Work? It can be carried by the `Dispatching` field in the Work.

### Corner Case Consideration

1. Resource dispatching suspension does not block resource deletion.

When a user specifies through PropagationPolicy that a resource is in a dispatching suspension, and then performs a deletion operation with the intention of removing the resource, the resource will be deleted normally without blocking the deletion, thus preventing leftover resources in the member clusters.

### Test Plan

#### UT

Add unit tests to cover the new functions.

#### E2E

- Test the resource dispatch suspension capability.
- Test the resource dispatch resume capability.
- Test the cluster-level resource dispatch suspension capability.
- Test at the namespace level and the cluster level respectively.

## Alternatives

Add a policy configuration type CRD, and users can decide to pause or resume the synchronization of specified resource templates to the target member clusters by configuring the CR of this CRD.

### API Change

```go
type RolloutPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RolloutSpec `json:"spec"`
}

type RolloutSpec struct {
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// Suspension declares the policy for suspending different
	// aspects of propagation. nil means no suspension.
	// +optional
	Suspension *Suspension `json:"suspension,omitempty"
}

// Suspension defines the policy for suspending different aspects of propagation.
type Suspension struct {
	// SuspendScheduling controls whether scheduling should be suspended.
	// +optional
	// Note: Postpone this until there is a solid use case.
	// SuspendScheduling *bool `json:"suspendScheduling,omitempty"`

	// SuspendDispatching controls whether dispatching should be suspended.
	// nil means not suspend.
	// Note: true means stop propagating to all clusters. Can not co-exist
	// with DispatchingOnClusters which is used to suspend particular clusters.
	// +optional
	SuspendDispatching *bool `json:"suspendDispatching,omitempty"`

	// SuspendDispatchingOnClusters declares a list of clusters to which the dispatching
	// should be suspended.
	// Note: Can not co-exist with Dispatching which is used to suspend all.
	// +optional
	SuspendDispatchingOnClusters *SuspendClusters `json:"suspendDispatchingOnClusters,omitempty"`
}

// SuspendClusters represents a group of clusters that should be suspended from propagating.
// Note: No plan to introduce the label selector or field selector to select clusters yet, as it
// would make the system unpredictable.
type SuspendClusters struct {
	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}
```

```go
// WorkSpec defines the desired state of Work.
type WorkSpec struct {
	...

	// SuspendDispatching controls whether dispatching should be suspended.
	// nil means not suspend.
	// Note: true means stop propagating to all clusters.
	// +optional
	SuspendDispatching *bool `json:"suspendDispatching,omitempty"`
}
```

A new ConditionType called `SuspendDispatching` is introduced to describe the distribution pause status of the Work resource. Meanwhile, the system will also report events to record the suspension of resource distribution.

### User usage example

When users apply the RolloutPolicy resource to the environment according to the following YAML file, it will pause the resource synchronization operation of Deployment(default/nginx) in all clusters:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: RolloutPolicy
metadata:
  name: nginx-deploy-rollout-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  suspension:
    suspendDispatching: true
```

At this point, all `Work` resources related to this `Deployment` on the Karmada control plane will be in a paused state, taking the `Work` resource corresponding to the member1 cluster as an example:

```yaml
apiVersion: work.karmada.io/v1alpha1
kind: Work
metadata:
  name: nginx-xxx
  namespace: karmada-es-member1
spec:
  workload:
    manifests:
      - xxx
  suspendDispatching: true
status:
  conditions:
    - lastTransitionTime: "2024-07-01T08:33:28Z"
      message: resource distribution is in a pause state.
      reason: Dispatching
      status: "True"
      type: Dispatching
```

When users delete the `suspension` field in the RolloutPolicy resource or delete the RolloutPolicy resource itself, it will resume the resource synchronization operation of Deployment(default/nginx) in all clusters.

For instance, if the Deployment(default/nginx) has been propagated to the clusters member1, member2, and member3, and when a user configures according to the following file and updates the image of the Deployment(default/nginx) resource to release a new version, the new version of the resource will only be synchronized to the member1 cluster.


```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: RolloutPolicy
metadata:
  name: nginx-deploy-synchronization
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  suspension:
    suspendDispatchingOnClusters:
      clusterNames:
        - member2
        - member3
```

When the user confirms that the deployment resource in member1 has been successfully published, and modifies the synchronization configuration as follows, the new version of the resource can then be synchronized to the member2 cluster:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: RolloutPolicy
metadata:
  name: nginx-deploy-synchronization
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      namespace: default
      name: nginx
  suspension:
    suspendDispatchingOnClusters:
      clusterNames:
        - member3
```

If, after synchronizing the new version of the resource to the member1 cluster, issues are found with the application that cause the release to fail, the user can roll back the version of the Deployment resource in the Karmada control plane, thereby protecting the business in the member2 and member3 clusters from being affected.

Q&A: How to remove the annotation `propagation.karmada.io/instruction` from Work? It can be carried by the `Dispatching` field in the Work.

### Advantages and Disadvantages Analysis

Solution one:

Advantages:
- For users, it is concise and intuitive, making it easy to control.
Disadvantages:
- For users, it is not possible to specify the distribution pause for a specific resource individually.
- For the system, the ResourceBinding resources also need to include a description of the distribution pause strategy and synchronize it to the Work.

Solution two (alternative solution):

Advantages:
- For users, it is possible to specify the distribution pause for a specific resource individually.
Disadvantages:
- For users, it increases the learning cost.
- For the system, this API feature is somewhat singular, and it requires a separate controller for resource processing, which increases the probability of resource update conflicts.
