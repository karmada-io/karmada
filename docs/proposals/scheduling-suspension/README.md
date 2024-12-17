---
title: Support for resource scheduling suspend and resume capabilities

authors:
- "@Vacant2333"
- "@Monokaix"

reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@a7i"

approvers:
- "@kevin-wangzefeng"
- "@RainbowMango"
- "@XiShanYongYe-Chang"
creation-date: 2024-10-13

---

# Support for resource scheduling suspend and resume capabilities

## Summary

<!--
Karmada 目前已经允许用户通过 Suspension 来暂停和恢复资源的 propagation 行为，这提高了 Karmada 的灵活性。

但是目前 Suspension 支持的不够彻底，在 Karmada 中资源的分发可以简单理解为两个阶段，资源的调度和资源的传播。

目前传播阶段可以较为灵活的由用户暂停和恢复，但是没有在调度阶段，没有提供暂停和恢复的能力。

我们希望在 Karmada 的基础之上，通过暂停调度的方式，实现队列和配额管理等高阶能力，从而将集群有限的资源优先调度给高优先级的工作负载，增强 Karmada 在多集群调度的能力，从而满足用户在复杂场景下的需求。

本文基于 [#5118](https://github.com/karmada-io/karmada/pull/5118) 在其中的 Suspension 基础之上，提供了暂停资源调度的能力。
-->

Karmada currently allows users to suspend and resume resource propagation behavior via `Suspension`, which improves the flexibility of Karmada.

However, the current support for Suspension is not thorough enough. Resource propagation in Karmada can be simply understood as two phases, resource scheduling and resource propagation.

At present, the propagation phase can be flexibly suspended and resumed by the user, but there is no suspension and resumption capability in the scheduling phase.

On the basis of Karmada, we hope to realize high-order capabilities such as queue and quota management through pause scheduling, so as to prioritize the limited resources of the cluster to high-priority workloads and enhance Karmada's multi-cluster scheduling capabilities, thereby meeting the needs of users in complex scenarios.

This proposal is based on `Suspension` definition in [#5118](https://github.com/karmada-io/karmada/pull/5118), which provides the ability to suspend resource scheduling.

## Motivation

<!--

当使用Karmada管理不同的集群时，不同的集群可能对用着不同的租户，这些租户在使用资源时需要有优先级的控制，即实现多租场景下公平和优先级调度，
因此需要在调度的时候对不同的工作负载进行优先级的区分，根据不同租户的优先级进行工作负载排序，并且在资源不足时进行排队等待。

不同的租户对资源的使用量也有一些配额限制，因此需要调度时对资源进行一些配额校验，当资源足够时才允许工作负载的调度和下发，否则就忽略工作负载，直到资源得到满足。

在K8s原生的资源中，Pod和Job都提供了类似的调度暂停和准入机制，Pod.Spec.SchedulingGates/Job.Spec.Suspend 字段分别表示了Pod和Job的暂停调度能力，用户可以较为轻松的基于这些字段来暂停负载的调度，
为了在多集群场景解决类似的需求，Karmada也需要提供相应的能力。

因此Karmada实现调度暂停后，就可以给外部controller提供一个契机，用户就可以去使用自己的 Controller，实现多租户队列优先级调度和配额管理等能力。

-->

When using Karmada to manage different clusters, different clusters may use different tenants, these tenants need to have priority control when using resources, i.e., to achieve fair and priority scheduling in multi-tenancy scenarios.

Therefore, it is necessary to prioritize different workloads during scheduling, sort workloads according to the priority of different tenants, and queue up when resources are insufficient.

Different tenants also have some quota restrictions on the amount of resources used, so we need to do some quota checks on the resources when scheduling, when the resources are sufficient to allow workload scheduling and distribution, otherwise the workload is ignored until the resources are satisfied.

In K8s native resources, Pod and Job both provide similar scheduling pause and admission mechanisms, `Pod.Spec.SchedulingGates`/`Job.Spec.Suspend` fields represent the pause scheduling capabilities of Pod and Job respectively, and the user can pause the scheduling of loads based on these fields more easily.

In order to address similar needs in a multi-cluster scenario, Karmada needs to provide the corresponding capabilities.

Therefore, after Karmada realizes scheduling suspension, it can provide an opportunity for external controllers, and users can go to use their own controllers to realize multi-tenant queue priority scheduling and quota management and other capabilities.

### Goals

- Provide the capability to **pause resource scheduling**

- Provide the capability to **resume resource scheduling**

## Proposal

### User Stories

#### Story 1

<!--
作为一名集群管理员，我使用Karmada纳管了不同租户下的集群资源，我希望Karmada在调度工作负载到不同集群时，可以根据不同租户的优先级进行作业调度和分发，
以保证高优先级的工作负载优先调度，同时兼顾公平性，因此我希望Karmada可以提供一种资源调度暂停的能力，这样我可以在上层进行工作负载的排序等操作，待排完序
之后再恢复调度。
-->

As a cluster administrator, I use Karmada to manage cluster resources under different tenants. I hope that Karmada can schedule and distribute workloads according to the priorities of different tenants when scheduling workloads to different clusters.
In order to ensure that high-priority workloads are scheduled first while taking into account fairness, I hope that Karmada can provide a resource scheduling pause capability so that I can perform workload sorting and other operations on the upper layer until the sorting is completed.
and then resume scheduling after that.

#### Story 2

<!--
作为一名集群管理员，我使用Karmada纳管了不同租户下的集群资源，不同租户可以使用的资源配额量不同，我希望Karmada在调度工作负载到不同集群时可以先暂停调度，然后我们在
Karmada之上构建一层配额管理和调度准入机制，只有租户资源足够时才进行工作负载的下发，否则保持工作负载暂停调度，以实现多租户的配额控制。
-->

As a cluster administrator, I use Karmada to manage cluster resources under different tenants. Different tenants have different resource quotas. I hope that Karmada can pause the scheduling first when scheduling workloads to different clusters, and then we can
A layer of quota management and scheduling access mechanism is built on top of Karmada. Workloads will be dispatched only when the tenant resources are sufficient. Otherwise, the workload will be suspended for scheduling to achieve multi-tenant quota control.

## Design Details

<!--
通过拓展 ResourceBinding.ResourceBindingSpec.Spec.Suspension字段，在其结构中加入 Suspension.Scheduling 字段来表示暂停调度，
Karmada Scheduler 组件需要根据 Suspension.Scheduling 的值来决定此时是否要开始调度对应的 ResourceBinding/ClusterResourceBinding，
当 Suspension.Scheduling 为true时，暂停调度，并刷新对用的condition，当 Suspension.Scheduling 为false时，恢复调度。默认创建的ResourceBinding
该字段为空，表示默认允许调度，从而不影响现有行为。
-->

By expanding the `ResourceBinding.ResourceBindingSpec.Spec.Suspension` field, and add the `Suspension.Scheduling` field to its structure to indicate suspension of scheduling.
The Karmada Scheduler component needs to decide whether to start scheduling the corresponding `ResourceBinding/ClusterResourceBinding` at this time based on the value of `Suspension.Scheduling`.
When `Suspension.Scheduling` is true, scheduling is suspended and the corresponding conditions are refreshed. When `Suspension.Scheduling` is false, scheduling is resumed. `ResourceBinding` created by default
This field is empty, indicating that scheduling is allowed by default and does not affect existing behavior.

### API Change

```go
type ResourceBindingSpec struct {
    // Suspension declares the policy for suspending different aspects of propagation.
    // nil means no suspension. no default values. 
    // +optional 
    Suspension *policyv1alpha1.Suspension `json:"suspension,omitempty"`
}

// Suspension defines the policy for suspending different aspects of propagation.
type Suspension struct {
    // Dispatching controls whether dispatching should be suspended.
    // nil means not suspend, no default value, only accepts 'true'.
    // Note: true means stop propagating to all clusters. Can not co-exist
    // with DispatchingOnClusters which is used to suspend particular clusters.
    // +optional
    Dispatching *bool `json:"dispatching,omitempty"`

    // DispatchingOnClusters declares a list of clusters to which the dispatching
    // should be suspended.
    // Note: Can not co-exist with Dispatching which is used to suspend all.
    // +optional
    DispatchingOnClusters *SuspendClusters `json:"dispatchingOnClusters,omitempty"`
	
    // New Added
    // Scheduling controls whether scheduling should be suspended.
    // nil means not suspend, no default value, only accepts 'true'.
    // Karmada scheduler will pause scheduling when value is true and resume scheduling when it's nil.
    Scheduling *bool `json:"scheduling,omitempty"`
}
```

### Implementations

<!--
Karmada scheduler监听rb状态，当ResourceBinding.Suspension.Scheduling=true时忽略该RB资源，暂停调度，当ResourceBinding.Suspension.Scheduling=false时恢复调度。
并且当RB已经调度成功后，不再允许修改rb从非暂停到暂停，同时ResourceBinding.Suspension.Scheduling设置为true或者false时，在RB中的status中刷新相应的condition。
-->
Karmada scheduler listens the `ResourceBinding` resource , ignores the `ResourceBinding` resource when `ResourceBinding.Suspension.Scheduling`=true, pauses scheduling, and resumes scheduling when `ResourceBinding.Suspension.Scheduling`=false.

And when the `ResourceBinding` has been successfully scheduled, it is no longer allowed to modify the `ResourceBinding` from non-suspended to suspended, and when `ResourceBinding.Suspension.Scheduling` is set to true or false, the corresponding condition is refreshed in the status of the `ResourceBinding`.

### User usage example

<!--
用户可以通过自定义的 Webhook + Controller 来实现自己期望的队列/多租户/资源配额能力，通过 Webhook 在 ResourceBinding 创建时将其 Suspension.Scheduling设置为true，
从而交由自定义的队列 Controller 处理，用户的 Workload/Job 就能够有序的按照如优先级来逐个调度。
-->

Users can use customized Webhook + Controller to achieve their desired queue/multi-tenant/resource quota capabilities, and set `Suspension.Scheduling` to true when the ResourceBinding is created through Webhook.

And then handled by the custom controller, and the user's Workload/Job can be scheduled one by one in an orderly manner according to priority.

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: volcano-admission-service-resourcebindings-mutate
webhooks:
  - name: mutateresourcebindings.volcano.sh
    admissionReviewVersions:
      - v1
    clientConfig:
      url: https://volcano-global-webhook.volcano-global.svc:443/resourcebindings/mutate
    failurePolicy: Fail
    matchPolicy: Equivalent
    reinvocationPolicy: Never
    rules:
      - operations: ["CREATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["v1alpha2"]
        resources: ["resourcebindings"]
        scope: "Namespaced"
    sideEffects: None
    timeoutSeconds: 3
```

### Test Plan

#### UT

Add unit tests to cover the new functions.

#### E2E

- Test the resource scheduling suspension capability.
- Test the resource scheduling resume capability.
