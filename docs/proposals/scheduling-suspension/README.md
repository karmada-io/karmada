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

Karmada currently allows users to suspend and resume resource propagation behavior via `Suspension`, which improves the flexibility of Karmada.

However, the current support for Suspension is not thorough enough. Resource propagation in Karmada can be simply understood as two phases, resource scheduling and resource propagation.

At present, the propagation phase can be flexibly suspended and resumed by the user, but there is no suspension and resumption capability in the scheduling phase.

On the basis of Karmada, we hope to realize high-order capabilities such as queue and quota management through pause scheduling, so as to prioritize the limited resources of the cluster to high-priority workloads and enhance Karmada's multi-cluster scheduling capabilities, thereby meeting the needs of users in complex scenarios.

This proposal is based on `Suspension` definition in [#5118](https://github.com/karmada-io/karmada/pull/5118), which provides the ability to suspend resource scheduling.

## Motivation

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

As a cluster administrator, I use Karmada to manage cluster resources under different tenants. I hope that Karmada can schedule and distribute workloads according to the priorities of different tenants when scheduling workloads to different clusters.
In order to ensure that high-priority workloads are scheduled first while taking into account fairness, I hope that Karmada can provide a resource scheduling pause capability so that I can perform workload sorting and other operations on the upper layer until the sorting is completed.
and then resume scheduling after that.

#### Story 2

As a cluster administrator, I use Karmada to manage cluster resources under different tenants. Different tenants have different resource quotas. I hope that Karmada can pause the scheduling first when scheduling workloads to different clusters, and then we can
A layer of quota management and scheduling access mechanism is built on top of Karmada. Workloads will be dispatched only when the tenant resources are sufficient. Otherwise, the workload will be suspended for scheduling to achieve multi-tenant quota control.

## Design Details

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

Karmada scheduler listens the `ResourceBinding` resource , ignores the `ResourceBinding` resource when `ResourceBinding.Suspension.Scheduling`=true, pauses scheduling, and resumes scheduling when `ResourceBinding.Suspension.Scheduling`=false.

And when the `ResourceBinding` has been successfully scheduled, it is no longer allowed to modify the `ResourceBinding` from non-suspended to suspended, and when `ResourceBinding.Suspension.Scheduling` is set to true or false, the corresponding condition is refreshed in the status of the `ResourceBinding`.

### User usage example

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
