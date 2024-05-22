---
title: Binding Priority and Preemption

authors:
- "@LeonZh0u"
- "@seanlaii"
- "@wengyao04"
- "@whitewindmills"
- "@zclyne"

reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@chaunceyjiang"

approvers:
- "@RainbowMango"

creation-date: 2024-05-27

notes: This proposal refers to ResourceBinding and ClusterResourceBinding collectively as binding.
---

# Binding Priority and Preemption

## Summary

Currently, the scheduler only schedules workloads in order, that is, the workload triggered first can be scheduled first.
And when a new binding has certain scheduling requirements that makes it infeasible on any member cluster,
it may stay in the scheduling queue, until sufficient resources are free, and it can be scheduled.

This proposal proposes the concept of priorities and preemption for bindings in Karmada and how the priority impacts scheduling and preemption of bindings
when the member cluster runs out of resources.
- The priority is independent of its workload type and must be one of the valid values, there is a total order on the values.
- Preemption is the action taken when an important binding requires resources or conditions which are not available in all member clusters. So, one or more bindings need to be preempted to make room for the important binding.

## Motivation

It is fairly common in large-scale clusters to have more tasks than what Karmada can handle, so scheduling responses can become slow.
Often times the workload is a mix of high priority critical tasks, and non-urgent tasks that can wait.
Karmada should be able to distinguish these workloads in order to decide which ones should be scheduled sooner, which ones can wait and which ones can be preempted.

### Goals

- How to specify priority and preemption strategy for bindings.
- Define how the order of these priorities are specified.
- Define how new priority levels are added.
- Effect of priority on scheduling and preemption.
- Define the concept of binding preemption in Karmada.
- Define scenarios under which a binding may get preempted.
- Define mechanics of preemption.

### Non-Goals

- How preemption works in spread constraints.

## Proposal

### Terminology

When a new binding has certain scheduling requirements that makes it infeasible on any member cluster,
scheduler may choose to clear the scheduling result of lower priority bindings to satisfy the scheduling requirements of the new binding.
We call this operation "binding preemption". Binding preemption is distinguished from "[policy preemption](../policy-preemption/README.md)" where high-priority policies preempt low-priority policies.

### User Stories (Optional)

#### As a user, I want to the task I submitted to be scheduled as soon as possible.

I submit a workload that is prioritized above other workloads, but do not wish to discard existing work by preempting scheduled bindings.
I hope that the high priority workload will be scheduled ahead of other queued bindings, as soon as sufficient cluster resources "naturally" become free.

![priority_queued](priority-queued.png)

#### As a user, I hope that GPU resources can be provided preferred to high-priority AI training tasks.

I submit a AI workload which occupies preferentially GPU resources of member clusters.
Karmada tries to find feasible clusters that can run the AI workload. If no enough member clusters are found,
I hope Karmada tries to remove bindings with lower priority from some arbitrary member clusters in order to make room for the pending bindings.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

Binding priority and preemption can have unwanted side effects. Here is an example of potential problems and ways to deal with them.

#### Bindings are preempted unnecessarily

Preemption removes existing bindings from some member clusters under resource pressure to make room for higher priority pending bindings.
If you give high priorities to certain bindings by mistake, these unintentionally high priority bindings may cause preemption in your multi-cluster system.

To address the problem, you can disable the preemption behavior for your bindings.

When a binding is preempted, there will be events recorded for the preempted binding.
Preemption should happen only when a member cluster does not have enough resources for a binding.
In such cases, preemption happens only when the priority of the pending binding (preemptor) is higher than the preempted bindings.
Preemption must not happen when there is no pending binding, or when the pending bindings have equal or lower priority than the preempted bindings.
If preemption happens in such scenarios, please feel free to file an issue.

## Design Details

### Effect of priority on scheduling

One could generally expect a binding with higher priority has a higher chance of getting scheduled than the same binding with lower priority.
However, there are other parameters that affect scheduling decisions. So, a high priority binding may or may not be scheduled before lower priority bindings.
For example, a high priority binding is marked as unschedulable due to insufficient resources of member clusters, but the preemption behavior is not enabled.
Karmada will try to schedule others with lower priority.

### Effect of priority on preemption

Generally, lower priority bindings are more likely to get preempted by higher priority bindings when member clusters have reached a threshold.
In such a case, scheduler may decide to preempt lower priority bindings to release enough resources for higher priority pending bindings.
As mentioned before, there are other parameters that affect scheduling decisions, such as cluster affinity and spread constraints.
If scheduler determines that a high priority binding cannot be scheduled even if lower priority bindings are preempted, it will not preempt lower priority bindings.

### New API

#### Priority Classes

This proposal will reuse the PriorityClass API of Kubernetes. The priority class defines the mapping between the priority name and its value.
It can have an optional description. It is an arbitrary string and is provided only as a guideline for users.

Similarly, we will follow the semantics of PriorityClass in Karmada. The following example gives a PriorityClass used by the system by default.
```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: foo
globalDefault: true
preemptionPolicy: PreemptLowerPriority
value: 2000000000
```

**Important notes**: Only one PriorityClass can be marked as `globalDefault`. However, if more than one PriorityClasses exists with their `globalDefault` field set to true, the
smallest value of such global default PriorityClasses will be used as the default priority.

### API change

#### ResourceBinding/ClusterResourceBinding API change

This proposal are going to add two new field: `Priority` and `PreemptionPolicy` for determining the behavior for preempting.
By default, preemption is disabled.

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    // The priority value. The karmada-scheduler component use this field to find the
    // priority of the binding.
    // The higher the value, the higher the priority.
    // +optional
    Priority *int32 `json:"priority,omitempty"`

    // PreemptionBehavior is the Policy for preempting bindings with lower priority.
    // One of Never, PreemptLowerPriority.
    // Defaults to Never if unset.
    // +optional
    PreemptionBehavior PreemptionBehavior `json:"preemptionBehavior,omitempty"`

    ... ...
```

#### PropagationPolicy/ClusterPropagationPolicy API change

User requests sent to Karmada may have PriorityClassName in their `Placement`.
Karmada resolves a PriorityClassName to its corresponding `Priority` and `PreemptionPolicy`, then populates them of the binding spec.
This proposal are going to add a new field `PriorityClassName` for specify PriorityClass.

```go
// Placement represents the rule for select clusters.
type Placement struct {
    // PriorityClassName indicates bindings will use the PriorityClass to resolve it's priority and preemptionPolicy if specified.
    // Any other name must be defined by creating a PriorityClass object with that name.
    // If not specified, the binding will use the global default PriorityClass first.
    // If the global default API does not exist, the binding priority will be zero, binding preemptionPolicy will be "Never".
    // +optional
    PriorityClassName string `json:"priorityClassName,omitempty"`

    ... ...
```

### About priority classes

Priority classes can be added or removed, but we only allow updating `Description`. 
While Karmada can work fine if priority classes are changed at run-time,
the change can be confusing to users as bindings with a priority class which were created before the change will have a different priority and preemptionPolicy than those created after the change.
Deletion of priority classes is allowed, despite the fact that there may be existing bindings that have specified such priority class names in their `placement`.
In other words, there will be no referential integrity for priority classes. This is another reason that all system components should only work with priority and preemptionPolicy and not with the PriorityClassName.

One could delete an existing priority class and create another one with the same name and different content.
By doing so, they can achieve the same effect as updating a priority class, but we still do not allow updating priority classes to prevent accidental changes.

### Preemption scenario

In this proposal, the only scenario under which a group of bindings in Karmada may be preempted is when a higher priority binding cannot be scheduled due to various unmet scheduling requirements,
such as lack of resources, spread constraints, etc., and the preemption of the lower priority bindings allows the higher priority binding to be scheduled.
So, if the preemption of the lower priority bindings does not help with scheduling of the higher priority binding, those lower priority bindings will keep running and the higher priority binding will stay pending.

**Important notes**: 
- The scheduler will only consider bindings whose resource requirements are not empty.
From scheduler's point of view, such a binding needs no resources and its preemption will not release any resources.
- Since the preemption feature is still in the experimental stage, we will not introduce preemption in ClusterResourceBinding first because ClusterResourceBinding are usually more sensitive cluster-level resources.

### Preemption order

When scheduling a pending binding, scheduler tries to place the binding on a member cluster that does not require preemption.
If there is no such a member cluster, scheduler may favor a member cluster where the number and/or priority of victims (preempted bindings) is smallest.
After choosing the member cluster, scheduler considers the lowest priority bindings for preemption first.
Scheduler starts from the lowest priority and considers enough bindings that should be preempted to allow the pending binding to schedule.
Scheduler only considers bindings that have lower priority than the pending binding.

Scheduler will try to minimize the number of preempted bindings.
As a result, it may preempt a binding while leaving lower priority bindings running if preemption of those lower priority bindings is not enough to schedule the pending binding while preemption of the higher priority binding(s) is enough to schedule the pending binding.
For example, if member cluster capacity is 10, and pending binding is priority 10 and requires 5 units of resource, and the running bindings are
```json lines
{
    "priority": 0,
    "request": 1
}
{
    "priority": 1,
    "request": 2
}
{
    "priority": 2,
    "request": 5
}
{
    "priority": 3,
    "request": 2
}
```
, scheduler will preempt the priority 2 binding only and leaves priority 1 and priority 0 running.

### Preemption in multi-scheduler

Karmada allows multiple schedulers to exist in its control plane, This introduces a race condition where multiple schedulers may perform round-robin preemption.
That is, scheduler A may schedule binding A, Scheduler B preempts binding A to schedule binding B which is then preempted by scheduler A to schedule binding A and we go in a loop.

Therefore, we strongly recommend not to enable preemption behavior in multiple schedulers.
If you must enable it, unless you can ensure that they will not have any intersection during preemption.

### Flowchart of the new scheduling algorithm

![binding_priority_preemption](binding-priority-preemption.png)

1. When the pending binding requires 2 units of resource and has 2 replicas, however, the schedule result(`target clusters`) only provide 3 units of resource. 
So we get the resource requirements gap is `2 * 2 - 3 = 1`.
2. If a binding cannot meet the resource requirement of the pending binding,
we don't stop accumulating its resource requirements until the resource requirements of the pending binding are met.
3. If there is no binding can meet the resource requirements of the pending binding, no preemption will be executed when the preemption loop ends.
So we'll try to preempt the accumulated bindings to meet the resource requirements of the pending binding, if can't, just fail.

### Feature gate

This binding preemption feature is an experimental feature, so we will introduce the following feature gates, which are disabled by default.
- ResourceBindingPreemption: indicates if a high-priority binding could preempt a low-priority binding, it's a global enablement.
- CrossNamespaceResourceBindingPreemption: indicates if a high-priority binding could preempt a low-priority binding across namespaces.

### Components change

#### karmada-controller-manager

Currently, a binding will be created or updated when resource templates are matched by propagation policy.
When a binding is created or updated, detector tries to find the priority class by `priorityClassName` to populate the binding spec.
detector will populate the binding spec with default priority and preemptionPolicy if the priority class is not found.

Only priority classes changes (by deleting and adding them with a different content) will not trigger bindings' update.

#### karmada-scheduler

Currently, the scheduler only runs a serial scheduling loop by a FIFO queue.
With this proposal, a priority scheduling queue will be implemented to replace the current work queue.
It should retain all the functionality of the current work queue and additionally implement sorting by priority.

In addition, if preemption behavior is enabled, the scheduler should identify the circumstances under which the pending binding should preempt other bindings,
calculate which bindings need to be preempted to meet current resource requirements.

#### karmada-webhook

Since we have strictly defined the binding priority and preemption, so the webhook should perform extra
validation work to prevent misleading configuration.

### Test Plan

- All current testing should be passed, no break change would be involved by this feature.
- Add new E2E tests to cover the feature, the scope should include:
    * bindings are scheduled by priority.
    * preemption between high-priority bindings and low-priority bindings.
    * preemption is disabled.

## Alternatives
