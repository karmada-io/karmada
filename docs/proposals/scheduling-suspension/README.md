---
title: Support for resource scheduling suspend and resume capabilities
authors:
- "@Vacant2333"
reviewers:
- TBD
approvers:
- TBD

creation-date: 2024-10-13

---

# Support for resource scheduling suspend and resume capabilities

## Summary

<!--
Karmada 目前已经允许用户通过 Suspension 来暂停和恢复资源的 propagation 行为，这提高了 Karmada 的灵活性。

但是我认为目前 Suspension 支持的不够彻底，在 Karmada 中资源的分发可以简单理解为两个阶段，资源的调度和资源的传播。
目前传播阶段可以较为灵活的由用户暂停和恢复，但是调度阶段用户无法介入。

在 Pod/Job 资源中，Pod.Spec.SchedulingGates/Job.Spec.Suspend 提供了暂停调度的能力。用户可以较为轻松的基于这些字段来暂停负载的调度，
通过自定义控制器来实现优先级/队列的能力后逐个恢复负载的调度。但是在 Karmada 中无法这样做到，目前 Karmada Scheduler 没有队列的能力，
所有的负载遵循先来后到进行调度。我们希望通过调度暂停的形式在Karmada之外实现队列能力，从而将集群有限的资源优先调度给高优先级的负载和任务。

本文提供了一种在 Suspension 的基础之上，暂停资源调度的能力。
-->

**Karmada** currently allows users to pause and resume resource propagation behavior through **Suspension**, enhancing Karmada's flexibility.

However, I believe the support for Suspension is not thorough enough. In Karmada, resource distribution can be simplified into two stages: the scheduling and the propagation of resources. Currently, the propagation stage can be paused and resumed flexibly by users, but the scheduling stage does not allow user intervention.

In the context of **Pod/Job** resources, `Pod.Spec.SchedulingGates` and `Job.Spec.Suspend` provide the ability to pause scheduling. Users can easily pause workload scheduling based on these fields and, with a custom controller, implement priority/queue capabilities to gradually resume workload scheduling. However, this is not achievable in Karmada. The current **Karmada Scheduler** lacks queuing capabilities, and all workloads are scheduled on a first-come, first-served basis. We hope to implement queuing capabilities outside of Karmada by pausing scheduling, thereby prioritizing the cluster's limited resources for high-priority workloads and tasks.

This article proposes a method to pause resource scheduling based on Suspension.

## Motivation

<!--
当前的 Karmada 无法提供队列和优先级等能力，来将有限的资源分配给高优先级任务/负载的能力。当用户有这类需求时，他们只能手动按顺序创建
PropagationPolicy 来优先调度。在负载和任务较少时这样没有问题，但是当一个联邦系统有大量的Job类负载时，手动无疑是效率低下和不精确的。

同时不同的用户会有不同的需求，如多租户/多队列/优先级队列调度能力。当 ResourceBinding 支持暂停调度后，用户就可以去实现自己的 Controller 并
设计自己的队列系统。
-->

**Karmada** currently lacks capabilities such as queuing and prioritization, which are necessary to allocate limited resources to high-priority tasks/workloads. When users have such needs, they can only manually create `PropagationPolicy` in sequence to prioritize scheduling. This is not a problem when there are fewer workloads and tasks, but it is inefficient and imprecise when a federated system has a large number of job-type workloads.

Different users also have different requirements, such as multi-tenant, multi-queue, and priority queue scheduling capabilities. Once `ResourceBinding` supports the suspension of scheduling, users will be able to implement their own Controllers and design their own queuing systems.

### Goals

- Provide the capability to **pause resource scheduling**
- Provide the capability to **resume resource scheduling**

## Proposal

### User Stories

#### Story 1

<!--
作为一名管理者，我希望 Karmada 能够允许我们通过一些方式在外部实现队列能力，从而在调度各部门的 Job 时能够限制各自的容量以及优先级，
当资源不足时能够做到 Karmada 优先调度高优先级任务，这将帮助集群更好的利用有限的资源。
-->

As a manager, I hope that **Karmada** could allow us to implement external queue capabilities, which would enable us to limit the capacity and priority of jobs from different departments when scheduling. When resources are scarce, Karmada should prioritize higher-priority tasks. This will help the cluster make better use of limited resources.

#### Story 2

<!--
作为一名用户，我希望当我创建了负载和对应的 PropagationPolicy 之后不要立刻调度，直到我取消调度暂停。
-->

As a user, I would like the scheduling not to start immediately after I have created the workload and the corresponding **PropagationPolicy**, until I lift the scheduling pause.

## Design Details

<!--
通过拓展 PropagationPolicy/ClusterPropagationPolicy.Spec.Suspension，在其结构中我们引入了 SuspendScheduling 字段来表示暂停调度，
同时此字段会传递到 ResourceBinding/ClusterResourceBinding 资源，Karmada Scheduler 组件需要根据 SuspendScheduling 来决定此时
是否要开始调度对应的 ResourceBinding/ClusterResourceBinding。
-->

By extending `PropagationPolicy/ClusterPropagationPolicy.Spec.Suspension`, we have introduced the `SuspendScheduling` field to indicate a pause in scheduling. This field is also propagated to `ResourceBinding/ClusterResourceBinding` resources. The **Karmada Scheduler** component needs to determine whether to start scheduling the corresponding `ResourceBinding/ClusterResourceBinding` based on `SuspendScheduling`.

### API Change

```go
// Suspension defines the policy for suspending different aspects of propagation.
type Suspension struct {
	...
	
    // SuspendScheduling controls whether scheduling should be suspended.
    // +optional
    SuspendScheduling *bool `json:"suspendScheduling,omitempty"`
}
```

### User usage example

#### Story 1 example

<!--
用户可以通过自定义的 Webhook + Controller 来实现自己期望的队列和多租户能力，通过 Webhook 在 ResourceBinding 创建时将其 Suspend，
从而交由自定义的队列 Controller 处理，用户的 Workload/Job 就能够有序的按照如优先级来逐个调度。
-->

Users can implement their desired queue and multi-tenancy capabilities through a custom **Webhook + Controller**. By using the Webhook to suspend the `ResourceBinding` at the time of creation, the custom queue Controller can take over, allowing users' `Workload/Job` to be scheduled in an orderly manner according to priority.

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

#### Story 2 example

<!--
用户在 PropagationPolicy 中设置 Deployment(default/nginx) 资源调度为暂停状态，Karmada 将不会开始调度该负载，
同时也不会有对应的 Work 资源存在，只会创建出待调度的 ResourceBinding 资源。
-->

When a user sets the scheduling of the `Deployment(default/nginx)` resource to a paused state in the `PropagationPolicy`, **Karmada** will not begin scheduling that workload. No corresponding `Work` resources will exist; only the `ResourceBinding` resource awaiting scheduling will be created.

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
  suspension:
    suspendScheduling: true
```

### Test Plan

#### UT

Add unit tests to cover the new functions.

#### E2E

- Test the resource scheduling suspension capability.
- Test the resource scheduling resume capability.
