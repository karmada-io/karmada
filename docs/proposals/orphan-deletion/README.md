---
title: Support orphan deletion of resources in control plane
authors:
- "@Poor12" 
reviewers:
- TBD
approvers:
- TBD

creation-date: 2023-03-24
---
# Support orphan deletion of resource templates in control plane

## Summary

Currently, when resource templates are deleted in Karmada control plane, Karmada will run a foreground deletion, which means that deleting resource templates will delete all applied resources in the member clusters.
However, users hope to choose not to garbage collect the applied resources when a resource template is deleted in some scenes.
For example, for an application published by the multi-cloud platform, users does not want the multi-cloud platform to continue to manage it, but wants it to be autonomous in a single cluster.
It may happen when an application is managed by multiple runtimes.
Or users want to prevent multi-cluster failures caused by accidental deletion/data loss of resource templates in the control plane.
Or for a specific application, users wants to keep its dependdent resources when deleting it.In that case, an orphan deletion way is needed.

## Motivation

### Goals

- Support orphan deletion of resource templates in control plane

### Non-Goals

- Need to consider whether it is necessary to expand the update strategy. If necessary, the implementation will be similar. Refer to https://github.com/karmada-io/karmada/issues/3248.

## Proposal

### User Stories (Optional)

#### Story 1

As a cluster administrator, I want to prevent multi-cluster failures caused by accidental deletion/data loss of resource templates in the control plane.
For example, I use `karmadactl promote` by mistake refer to https://github.com/karmada-io/karmada/issues/3004.
I hope that deleting a resource template in the multi-cloud control plane just means that the resource is no longer managed by Karmada.

#### Story 2

As a user, I want to deploy a few ordered jobs to member clusters. These jobs depend on the same PVC. And I want to destroy the first main job after it's completed and then execute the remaining jobs.
Currently, Karmada will delete the first job and meanwhile delete the PVC. This will cause the last deleted PVC to be re-installed after the second job starts.
In that case, I want to keep the dependent resources when deleting a resource template.

## Design Details

We may provide several delete strategies:

* Foreground: Delete all applied resources with resources they depend on(use [PropagateDeps](https://karmada.io/docs/userguide/scheduling/propagate-dependencies) feature) in member clusters.
* Orphan: Do nothing in member clusters.
* KeepDependence: Delete applied resources but keep the dependent resources.

Option One:

Karmada provides a label/annotation for resource templates. In the process of propagating, labels/annotations of ResourceBinding and Work will inherits directly from those of resource templates.
When deleting the resources, execution_controller recognizes this field and handles it accordingly.

Naming can refer to the following: `karmada.io/deleteOption`: `Foreground`/`Orphan`/`KeepDependence`

Option Two:

Karmada provides policy-level deletion options which means resources hit by the same PropagationPolicy will use the same DeleteOption. This configuration will inherit from Policy to Binding, finally to Work.

A reference implementation:

```golang
const (
    DeleteOptionTypeForeground DeleteOptionType = "Foreground"
    DeleteOptionTypeOrphan DeleteOptionType = "Orphan"
    DeleteOptionTypeKeepDependence DeleteOptionType = "KeepDependence"
)

type PropagationSpec struct {
    DeleteOption DeleteOptionType `json:"deleteOption,omitempty"`
}

type ResourceBindingSpec struct {
    DeleteOption DeleteOptionType `json:"deleteOption,omitempty"`
}

type WorkSpec struct {
    // Workload represents the manifest workload to be deployed on managed cluster.
    Workload []WorkloadTemplate `json:"workload,omitempty"`
}

type WorkloadTemplate struct {
    Manifest Manifest `json:"manifests,omitempty"`
    DeleteOption DeleteOptionType `json:"deleteOption,omitempty"`
}
```

### Test Plan

#### UT

Test cover the new method.

#### E2E

Add a case to test orphan deletions of resource templates in Karmada control plane.
