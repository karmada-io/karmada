---
title: Improve Karmada observability
authors:
- "@Poor12"
reviewers:
- "@TBD"
approvers:
- "@TBD"

creation-date: 2022-06-02

---

# Improve Karmada observability

## Summary

Provides metrics to improve Karmada observability

## Motivation

### Goals

- Provides metrics to reflect the status of Karmada resources
- Provides metrics to reflect the key performance of Karmada components
- Non-invasive
- Compliant with Kubernetes metrics specification

### Non-Goals

## Proposal

### User Stories (Optional)

#### Story 1

For now, Karmada lacks the observability of individual components. On the one hand, we need metrics that can reflect the object status in the process of distributing resources to member clusters, such as work and resourceBinding. On the other hand, we need metrics that reflect the performance of Karmada components for upcoming performance test, such as schedule latency.

Now, only Karmada-scheduler has four metrics, `schedule_attempts_total` means numbers of attempts to schedule resourceBindings, `e2e_scheduling_duration_seconds` means E2e scheduling latency in seconds, `scheduling_algorithm_duration_seconds` means scheduling algorithm latency in seconds, `queue_incoming_bindings_total` means numbers of bindings added to scheduling queues by event type.

## Design Details

For object status:

ResourceBinding means delivery situation of the resource template. For resourceBinding, We plan to provide metrics like `karmada_resourcebinding_condition` and `karmada_clusterresourcebinding_condition`. And it will have labels including `name`, `namespace`, `type`.

e.g. If you apply a resource template to member clusters successfully(schedule and create work successfully), you will get metrics below.

`karmada_resourcebinding_condition{name="nginx-deployment", namespace="default", type="Scheduled"} 1`
`karmada_resourcebinding_condition{name="nginx-deployment", namespace="default", type="FullyApplied"} 1`
`karmada_resourcebinding_condition{name="nginx-deployment", namespace="default", type="Deleted"} 0`

Work means a resource instance that propagated to a member cluster. For work, we plan to provide metrics like `karmada_work_condition`. And it will have labels including `name`, `namespace`, `type`.

e.g. If a work is successfully applied to member clusters, you will get metrics below.

`karmada_work_condition{name="nginx-687f7fb96f", namespace="karmada-es-member1", type="Applied"} 1`
`karmada_work_condition{name="nginx-687f7fb96f", namespace="karmada-es-member1", type="Deleted"} 0`

For component performance:

Now Karmada-controller-manager only has metrics created by controller-runtime, we need some metrics to reflect performances of Karmada components.

e.g. During the whole process of Karmada distributing applications to member clusters, in addition to the scheduling latency, we may want to know latency from detecting resources to creating resourceBindings, or from resourceBindings to works.

`karmada_resource_reconcile_duration_seconds_bucket{kind="Deployment, name="nginx", namespace="default", le="0.001"}` XXX
It means that the latency from detecting some resources to reconciling resourceBindings.

`karmada_resourcebinding_reconcile_duration_seconds_bucket{name="nginx-deployment", namespace="default", le="0.001"}` XXX
It means that the latency from resourceBinding to some work in specified member clusters.

`karmada_work_sync_duration_seconds_bucket{name="nginx-687f7fb96f", namespace="karmada-es-member1", le="0.001"}` XXX
It means that the latency to sync work in Karmada control plane to member clusters.

## Alternatives

For optional component, what metrics to add still needs to be discussed. 

### Test Plan

