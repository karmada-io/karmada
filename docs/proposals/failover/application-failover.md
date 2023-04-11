---
title: Application failover mechanism 
authors:
  - "@Poor12"
reviewers:
  - "@RainbowMango"
  - "@Garrybest"
  - "@chaunceyjiang"
  - "@XiShanYongYe-Chang"
  - "@kevin-wangzefeng"
  - "@GitHubxsy"
approvers:
  - "@RainbowMango"
  - "@kevin-wangzefeng"
  - "@GitHubxsy"

create-date: 2023-02-10
---
# Application failover mechanism

## Summary

Kubernetes has fault tolerance and self-healing capabilities, which can automatically migrate workloads to other normal nodes when nodes fail.
When kubelet detects that the node resources are insufficient and reaches the eviction threshold, kubelet will evict the low-priority Pod from the node and reschedule it to another node with sufficient resources.
Also, when a node is abnormal or a user shuts down a node for maintenance, Kubernetes ensures the HA operation of Pods based on the taint eviction mechanism.

However, the above solutions still cannot solve some scenarios.
When there are not enough resources in the cluster to run an application, the evicted application will not be able to be scheduled and end up in a suspended state for a long time.
At this time, users hope to provide a multi-cluster failover mechanism to schedule applications to another cluster with sufficient resources.

The same situation will occur in clusters with resource preemption. if no node is found that satisfies all the specified requirements of the Pod, preemption logic tries to find a node where removal of one or more Pods with lower priority than scheduling Pod would enable it to be scheduled on that node.
If such a node is found, one or more lower priority Pods get evicted from the node. After the Pods are gone, the Pod with high priority can be scheduled.
In that case, Pods with low priority may be evicted frequently and have fewer scheduling opportunities.
The reason for preemption may be due to resource shortage, or it may be due to affinity between Pods.
At this time, single-cluster failover cannot effectively solve the problem. Multi-cluster failover may be a reasonable way.

In the multi-cluster scenario, user workloads may be deployed in multiple clusters to improve service high availability.
Karmada already supports multi-cluster failover when detecting a cluster fault. It's a consideration from a cluster perspective.
However, the application may still be unavailable when the control plane of the cluster is in a healthy state.
Therefore, Karmada needs to provide a means of fault migration from an application perspective.

## Motivation

The cluster administrators want to ensure that the application can run normally, regardless of which cluster the application is running. When the application is unavailable and it cannot self-heal within a single cluster,
they want Karmada to reschedule the application and avoid clusters that failed to run before.

## Goals

- Extend the API of PropagationPolicy to provide users with the rescheduled configuration.
- Extend the API of ResourceBinding to record previously evicted clusters.
- Propose the implementation ideas for involved components, including `karmada-controller-manager`, `karmada-webhook` and `karmada-scheduler`.
  For example, `karmada-controller-manager` need to add a component to observe the health status of the application. `Karmada-scheduler` need to
  add a plugin to filter the evicted cluster.

## Non-Goals

## Proposal

### User Stories (Optional)

#### As a user, I deploy an application in multiple clusters with preemptive scheduling. When business peaks, low-priority applications cannot run normally for a long time.

Supposing I deploy an application in a cluster with preemptive scheduling. When cluster resources are in short supply, low-priority applications that were running normally are preempted and cannot run normally for a long time.
At this time, applications cannot self-heal within a cluster. Users wants to try to schedule it to another cluster to ensure that the service is continuously served.

#### As a user, I use the cloud vendor's spot instance to deploy my application. As a result, the application may fail due to insufficient resources.

Spot instances are a mode of instances that feature a discounted price and a system interruption mechanism, which means that you can purchase a spot instance at a discounted price, but the system may automatically repossess it.
When users use spot instances to deploy applications, the application may fail to run due to resources being recycled. In this scenario, the amount of resources perceived by the scheduler is the size of the resource quota, not the actual available resources.
At this time, users want to schedule the application to a cluster other than the one that failed previously.

Link:

* en: [Use preemptible instances(Alibaba Cloud)](https://www.alibabacloud.com/help/en/container-service-for-kubernetes/latest/use-preemptible-instances), [Spot Mode(Tencent Cloud)](https://www.tencentcloud.com/document/product/457/46967), [Amazon EC2 Spot(AWS)](https://aws.amazon.com/ec2/spot/details/)
* cn: [使用抢占式实例（阿里云）](https://help.aliyun.com/document_detail/165053.html), [竞价模式说明（腾讯云）](https://cloud.tencent.com/document/product/457/59364), [Amazon EC2 Spot功能（AWS）](https://aws.amazon.com/cn/ec2/spot/details/?nc1=h_ls)

### Dependency

Karmada's `Resource Interpreter Framework` is designed for interpreting resource structure. It provides users with a interpreter operation to tell Karmada
how to figure out the health status of a specific object. It is up to users to decide when to reschedule.

For example:

```yaml
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: declarative-configuration-example
spec:
  target:
    apiVersion: apps/v1
    kind: Deployment
  healthInterpretation:
    luaScript: >
      function InterpretHealth(observedObj)
        return observedObj.status.readyReplicas == observedObj.spec.replicas
      end
```

**Note**: In the application failover scenario, the `Health` field not only represents the health status of the application,
but also one of the conditions to determine whether the application is migrated. Please configure this field carefully when using failover.

### Risks and Mitigations

This proposal maintains the backward compatibility, the system built with previous versions of Karmada can be seamlessly migrated to the new version.
The previous configurations(yamls) could be applied to the new version of Karmada and without any behavior change.

## Design Details

Users are able to define the behavior of failover by defining policies.
The configuration of failover behavior includes:

* When to trigger application failover
* How to migrate from one cluster to another
* Time at which the migrated cluster can be recovered

Sometimes, you have to deal with legacy applications that might require an additional startup time on their first initialization. Or you want to
avoid invalid failovers due to misconfigurations. In such cases, we need some preconditions to start the failover behavior.
Optional configuration items include:

* Healthy state. We just want to migrate those applications that have run successfully. For those applications that failed at the beginning, we prefer to solve them in the single cluster.
* A period of time. We can wait a period of time before the application failover is initiated.

Also, there are some decision conditions of performing the failover process.

* Healthy state. We can define in what state the application triggers the migration. Valid options are "Unhealthy", "Unknown" or both.
* A period of time. For some short-lived exceptions(which can be recovered after a period of time), we don't want to immediately migrate the application to other clusters which may make the migration very frequent.
  Therefore, we need a parameter to tolerate the unhealthy application.

### API change

#### PropagationPolicy API change

```golang

// PurgeMode represents that how to deal with the legacy applications on the
// cluster from which the application is migrated.
type PurgeMode string

const (
    // Immediately represents that Karmada will immediately evict the legacy
    // application.
    Immediately PurgeMode = "Immediately"
    // Graciously represents that Karmada will wait for the application to 
    // come back to healthy on the new cluster or after a timeout is reached 
    // before evicting the application.
    Graciously PurgeMode = "Graciously"
    // Never represents that Karmada will not evict the application and
    // users manually confirms how to clean up redundant copies.
    Never PurgeMode = "Never"
)


// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
    // ....
  
    PropagateDeps bool `json:"propagateDeps,omitempty"`
  
    // Failover indicates how Karmada migrates applications in case of failures.
    // If this value is nil, failover is disabled.
    // +optional
    Failover *FailoverBehavior `json:"failover,omitempty"`
  
    // ...
}

// FailoverBehavior indicates failover behaviors in case of an application or
// cluster failure.
type FailoverBehavior struct {
    // Application indicates failover behaviors in case of application failure.
    // If this value is nil, failover is disabled.
    // If set, the PropagateDeps should be true so that the dependencies could
    // be migrated along with the application.
    // +optional
    Application *ApplicationFailoverBehavior `json:"application,omitempty"`
  
    // Cluster indicates failover behaviors in case of cluster failure.
    // If this value is nil, failover is disabled.
    // +optional
    // Cluster *ClusterFailoverBehavior `json:"cluster,omitempty"`
}

// ApplicationFailoverBehavior indicates application failover behaviors.
type ApplicationFailoverBehavior struct {
    // PreConditions indicates the preconditions of the failover process.
    // If specified, only when all conditions are met can the failover process be started.
    // Currently, PreConditions includes several conditions:
    // - DelaySeconds (optional)
    // - HealthyState (optional)
    // +optional
    PreConditions *PreConditions `json:"preConditions,omitempty"`
  
    // DecisionConditions indicates the decision conditions of performing the failover process.
    // Only when all conditions are met can the failover process be performed.
    // Currently, DecisionConditions includes several conditions:
    // - TolerationSeconds (optional)
    // - HealthyState (mandatory)
    // +required
    DecisionConditions *DecisionConditions `json:"decisionConditions,omitempty"`
  
    // PurgeMode represents how to deal with the legacy applications on the
    // cluster from which the application is migrated.
    // Valid options are "Immediately", "Graciously" and "Never".
    // Defaults to "Graciously".
    // +kubebuilder:default=Graciously
    // +optional
    PurgeMode PurgeMode `json:"purgeMode,omitempty"`
  
    // BlockPredecessorSeconds represents the period of time the cluster from which the
    // application was migrated from can be schedulable again.
    // During the period of BlockPredecessorSeconds, clusters are forcibly filtered out by the scheduler. 
	// If not specified, the scheduler may still schedule the application to the evicted cluster when rescheduling.
    // Defaults to 600s. Zero means the cluster will never be schedulable.
    // +kubebuilder:default=600
    // +optional
    BlockPredecessorSeconds *int32 `json:"blockPredecessorSeconds,omitempty"`
}

// PreConditions represents the preconditions of the failover process.
type PreConditions struct {
    // DelaySeconds refers to a period of time after the control plane collects
    // the status of the application for the first time.
    // If specified, the failover process will be started after DelaySeconds is reached.
    // It can be used simultaneously with HealthyState and does not affect each other.
    // +optional
    DelaySeconds *int32 `json:"delaySeconds,omitempty"`
  
    // HealthyState refers to the healthy status reported by the Karmada resource
    // interpreter.
    // Valid options are "Healthy".
    // If specified, the failover process will be started when the application reaches the healthy state.
    // It can be used simultaneously with DelaySeconds and does not affect each other.
    // +optional
    HealthyState ResourceHealth `json:"healthyState,omitempty"`
}

// DecisionConditions represents the decision conditions of performing the failover process.
type DecisionConditions struct {
    // TolerationSeconds represents the period of time Karmada should wait
    // after reaching the desired state before performing failover process.
    // If not specified, Karmada will immediately perform failover process.
    // Defaults to 10s.
    // +kubebuilder:default=10
    // +optional
    TolerationSeconds *int32 `json:"tolerationSeconds,omitempty"`
  
    // HealthyState refers to the healthy status reported by the Karmada resource
    // interpreter.
    // Valid options are "Unhealthy", "Unknown", or both.
    // When the application reaches the desired HealthyState, Karmada will perform failover process
    // after TolerationSeconds is reached.
    // +kubebuilder:validation:MinItems=1
    // +required
    HealthyState []ResourceHealth `json:"healthyState,omitempty"`
}
```

For example:

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
    - apiVersion: apps/v1
      kind: StatefulSet
      name: mysql
  failover:
    application:
      preConditions:
        delaySeconds: 5000
      decisionConditions:
        tolerationSeconds: 200
        healthyState: 
        - Unhealthy
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

#### ResourceBinding API change

```golang
type ResourceBindingSpec struct {
    ...
    // ActiveEvictionHistory represents the eviction history which might affect the scheduler's
    // scheduling decisions. 
    // The scheduler tends not to schedule applications on clusters with a history of eviction.
    // +optional
    ActiveEvictionHistory []ActiveEvictionHistory `json:"activeEvictionHistory,omitempty"`
  
    // FailoverBehavior represents the failover config for the referencing resource.
    // It inherits directly from the associated PropagationPolicy(or ClusterPropagationPolicy).
    // +optional
    FailoverBehavior *policyv1alpha1.FailoverBehavior `json:"failoverBehavior,omitempty"`
    ...
}

// AggregatedStatusItem represents status of the resource running in a member cluster.
type AggregatedStatusItem struct {
    // ClusterName represents the member cluster name which the resource deployed on.
    // +required
    ClusterName string `json:"clusterName"`
  
    // Status reflects running status of current manifest.
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    Status *runtime.RawExtension `json:"status,omitempty"`

    // Applied represents if the resource referencing by ResourceBinding or ClusterResourceBinding
    // is successfully applied on the cluster.
    // +optional
    Applied bool `json:"applied,omitempty"`
  
    // Settled represents if the resource referencing by ResourceBinding or ClusterResourceBinding 
    // is once Healthy on the cluster. 
    // +optional
    Settled bool `json:settled,omitempty`
  
    // AppliedMessage is a human readable message indicating details about the applied status.
    // This is usually holds the error message in case of apply failed.
    // +optional
    AppliedMessage string `json:"appliedMessage,omitempty"`
  
    // Health represents the healthy state of the current resource.
    // There maybe different rules for different resources to achieve health status.
    // +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
    // +optional
    Health ResourceHealth `json:"health,omitempty"`
  
    // CreationTimestamp is a timestamp representing the server time when this AggregatedStatusItem was
    // created.
    // It represents the timestamp when the control plane first collects status of the resource running in a member cluster.
    //
    // Clients should not set this value to avoid the time inconsistency issue.
    // It is represented in RFC3339 form(like '2021-04-25T10:02:10Z') and is in UTC.
    //
    // Populated by the system. Read-only.
    // +optional
    CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
}

type ActiveEvictionHistory struct {
    // ClusterName represents the evicted cluster name.
    // +required
    ClusterName string `json:"clusterName,omitempty"`
  
    // CreationTimestamp is a timestamp representing the server time when this cluster was
    // evicted. After specific reset timeout, the evicted cluster will become schedulable again.
    //
    // Clients should not set this value to avoid the time inconsistency issue.
    // It is represented in RFC3339 form(like '2021-04-25T10:02:10Z') and is in UTC.
    //
    // Populated by the system. Read-only.
    // +optional
    CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
}
```

For example:

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: nginx-pod
spec:
  clusters:
  - name: member1
  activeEvictionHistory:
  - clusterName: member2
    createTimestamp: "2023-04-03T01:44:31Z"
  failover:
    application:
      preConditions:
        delaySeconds: 5000
      decisionConditions:
        tolerationSeconds: 200
        healthyState: 
        - Unhealthy
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
  resource:
    apiVersion: v1
    kind: Pod
    name: nginx
    namespace: default
  schedulerName: default-scheduler
status:
  aggregatedStatus:
  - applied: true
    settled: true
    clusterName: member1
    health: Healthy
    status:
      phase: Running
    createTimestamp: "2023-04-03T02:44:31Z"
```

### Components change

#### karmada-controller-manager

karmada-controller-manager should add a controller which will watch the change of the health field of ResourceBindingStatus,
and requeue according to the toleration time. If it finds that it's unhealthy twice in a row, which means the application is unhealthy during the period,
it will trigger the original eviction logic.

#### karmada-webhook

Since we add some new fields to APIs, karmada-webhook should perform extra validation work to prevent misleading configuration.

#### karmada-scheduler

karmada-scheduler should add a new filter plugin, to filter the evicted cluster so that rescheduling will not be scheduled to
clusters in `ActionEvictionHistory`.

### Test Plan

- Unit Test covering.
- E2E Test covering.

## FAQ

### What's difference between this proposal and karmada-descheduler?

karmada-descheduler focuses more on `Replicas`, whereas this proposal focuses more on `Application` deployed in one specific cluster.

For example, assume there is one deployment deployed to multiple clusters. Its status is like:

```yaml
status:
    aggregatedStatus:
    - applied: true
      clusterName: member1
      health: Unhealthy
      status:
        replicas: 2
        availableReplicas: 1
    - applied: true
      clusterName: member2
      health: healthy
      status:
        replicas: 1
        availableReplicas: 1
```

karmada-descheduler tend to keep the healthy replica and reschedule that pending replica to other clusters such as:

```yaml
status:
    aggregatedStatus:
    - applied: true
      clusterName: member1
      health: healthy
      status:
        replicas: 1
        availableReplicas: 1
    - applied: true
      clusterName: member2
      health: healthy
      status:
        replicas: 2
        availableReplicas: 2
```

However, this proposal tend to remove all replicas and reschedule the deploy to a new cluster such as:

```yaml
status:
    aggregatedStatus:
    - applied: true
      clusterName: member3
      health: healthy
      status:
        replicas: 2
        availableReplicas: 2
    - applied: true
      clusterName: member2
      health: healthy
      status:
        replicas: 1
        availableReplicas: 1
```

What's more, karmada-descheduler performs rescheduling based on the cluster resource margin detected by the estimator. It cannot resolve errors that are not caused by insufficient resources
or scenarios where resources cannot be properly probed, such as serverless computing. This proposal performs rescheduling based on past failed attempts to run.
It does not depend on the estimator and it also does not have the above limitations.

### How to determine whether the failure could be solved by rescheduling?

Only users can know whether th failover could be solved by rescheduling. Users can custom the health definition of resource objects by `Resource Interpret Framework` to decide when to reschedule.
In extreme cases, if the application is unavailable due to configuration errors and its status cannot recognize this error, migrating to any cluster may not solve the problem.
Users can configure the policy to determine whether to reschedule and the corresponding rescheduling config.

## Alternatives

For the `InitiateGates` and `ActiveGates` of application failover behavior, one option is using the condition of the resource as a trigger.

For a policy that applies to multiple resources, we expect a behavior to apply to all hit resources. However, different resources have different conditions.
Therefore, when `Condition` is used as the trigger condition, the situation of multiple applications cannot be solved.
We need to add `apiVersion/kind` to indicate different resource objects in the policy.

```golang
type PropagationSpec struct {
    // ....
  
    PropagateDeps bool `json:"propagateDeps,omitempty"`
  
    // Failover indicates how Karmada migrates applications in case of failures.
    // If this value is nil, failover is disabled.
    // +optional
    Failover []FailoverBehavior `json:"failover,omitempty"`
  
    // ...
}

// FailoverBehavior indicates failover behaviors in case of an application or
// cluster failure.
type FailoverBehavior struct {
    // APIVersion represents the API version of the target resources.
    // +optional
    APIVersion string `json:"apiVersion,omitempty"`
  
    // Kind represents the Kind of the target resources.
    // +optional
    Kind string `json:"kind,omitempty"`

    // Behavior represents the behavior of the target resources.
    // +optional
    Behavior *Behavior `json:"behavior,omitempty"`
}

type Behavior struct {
    // Application indicates failover behaviors in case of application failure.
    // If this value is nil, failover is disabled.
    // If set, the PropagateDeps should be true so that the dependencies could
    // be migrated along with the application.
    // +optional
    Application *ApplicationFailoverBehavior `json:"application,omitempty"`
  
    // Cluster indicates failover behaviors in case of cluster failure.
    // If this value is nil, failover is disabled.
    // +optional
    Cluster *ClusterFailoverBehavior `json:"cluster,omitempty"`
}

// ApplicationFailoverBehavior indicates application failover behaviors.
type ApplicationFailoverBehavior struct {
    // InitiateGates indicates the preconditions of the failover process.
    // If specified, all gates will be evaluated for prerequisites.
    // +optional
    InitiateGates []FailoverInitiateGate `json:"initiateGates,omitempty"`
  
    // ActiveGates indicates the conditions of performing the failover process.
    //
    // +kubebuilder:validation:MinItems=1
    // +required
    ActiveGates []FailoverActiveGate `json:"activeGates"`
  
    // TolerationSeconds represents the period of time the Karmada should wait
    // before performing failover. Defaults to 10s.
    //
    // +kubebuilder:default=10
    // +optional
    TolerationSeconds int32
  
    // PurgeMode represents how to deal with the legacy applications on the
    // cluster from which the application is migrated.
    // Valid options are "Immediately", "Graciously" and "Never".
    // Defaults to "Graciously".
    //
    // +kubebuilder:validation:Enum=Immediately;Graciously;Never
    // +optional
    PurgeMode string `json:"purgeMode,omitempty"`
  
    // EscapeSeconds represents the period of time the cluster from which the
    // application was migrated from should be escaped.
    // Defaults to 600s.
    //
    // +kubebuilder:default=600
    // +optional
    EscapeSeconds int32 `json:"escapeSeconds,omitempty"`
}

// FailoverInitiateGate represents the preconditions of the failover process.
type FailoverInitiateGate struct {
    // ConditionType refers to a condition in the object's condition list with
    // matching type. If the status of the specified condition is "true" means pass.
    // +optional
    ConditionType string `json:"conditionType,omitempty"`
  
    // HealthyState refers to the healthy status reported by the Karmada resource
    // interpreter.
    // Valid options are "Healthy" and "Unknown".
    // +optional
    HealthyState string `json:"healthyState,omitempty"`
}

// FailoverActiveGate represents the conditions of performing the failover process.
type FailoverActiveGate struct {
    // ConditionType refers to a condition in the object's condition list with
    // matching type. If the status of the specified condition is "true" means pass.
    // +optional
    ConditionType string `json:"conditionType,omitempty"`
  
    // HealthyState refers to the healthy status reported by the Karmada resource
    // interpreter.
    // Valid options are "Unhealthy" and "Unknown".
    // +optional
    HealthyState string `json:"healthyState,omitempty"`
}
```

For example:

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
    - apiVersion: apps/v1
      kind: StatefulSet
      name: mysql
  failover:
    - apiVersion: apps/v1
      kind: Deployment
      behavior:
        application:
          activeGates:
            - healthyState: Unhealthy
    - apiVersion: apps/v1
      kind: StatefulSet
      behavior:
        application:
          activeGates:
            - healthyState: Unhealthy
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```
