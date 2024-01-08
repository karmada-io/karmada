---
title: Introduce ExcludedResources to Policy to exclusionary selecting resource
authors:
  - "@chaosi-zju"
reviewers:
  - "@RainbowMango"
  - "TBD"
approvers:
  - "@RainbowMango"
  - "TBD"

creation-date: 2024-01-30
---

# Introduce ExcludedResources to Policy to exclusionary selecting resource

## Background

Users may use a broad Policy to manage many resource templates. As the business develops, users may want to specify 
new customized policies for some of these resources (such as certain types of resources, certain label groups, 
or certain namespaces) to replace the management.

Therefore, users hope that after creating a new Policy, they can exclude some managed resources from the original Policy. 
In this way, the resources will be managed and adjusted for propagation by the new Policy.

### Goals

* Provides a mechanism to exclude and filter some resources from a Policy

### Applicable scenario

This feature may help in a scenario where:

* Customize Policy for individual cases (take over a part of resources from the original Policy having a large selecting scope).
* Cluster resource migration (migrate resources following the new Policy by iteratively excluding resources from the original Policy).

## Proposal

### Overview

This proposal are introducing adding a `excludedResources` filed to PropagationPolicy/ClusterPropagationPolicy so that 
users can exclusionary selecting resource.

When `resourceSelectors` and `excludedResources` are defined at the same time, resources will be filtered
based on `resourceSelectors` first, then continue filtering based on `excludedResources`,
and finally get resources that match with Policy.

In this case, users can use this filed to exclude a part of resources from current bound Policy so that 
it can match to other new Policies.

### User Story

Here is a story in which the user wants to achieve cluster migration by modifying the target cluster of the Policy, 
the Policy just like this:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp
spec:
  priority: 10
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
  - apiVersion: v1
    kind: ConfigMap
```

The user wants to change `spec.placement.clusterAffinity.clusterNames` to `["member3", "member4"]`.

However, its policy has a wide range of filtering, and once changed, it will affect a large number of workload changes with 
significant migration risks. He hopes that the migration action can be smaller, for example, migrating at the namespace level.
By `excludedResources` introduced by this proposal, he can follow these four steps to achieve it.

#### Step 1

Write and apply a new Policy with expected clusters. It is worth noting that the new Policy is better to be given a relatively 
highest `priority` to ensure that excluded resources can be taken over by this Policy.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp-v2
spec:
  priority: 9999       # relatively highest priority
  placement:
    clusterAffinity:
      clusterNames:    # changed clusters
      - member3
      - member4
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
  - apiVersion: v1
    kind: ConfigMap
```

#### Step 2

The user wants to change the resources managed by the original Policy under the `ns1` namespace to be managed by the new Policy,
he just needs to update the original Policy as follows.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp
spec:
  priority: 10
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
    - apiVersion: v1
      kind: ConfigMap
  excludedResources:
    - namespace: ns1     # exclude ns1 namespace resources for observation and confirmation
```

Then he can observe and confirm that the migration results of resources under the `ns1` namespace meet expectations.

#### Step 3

Since `ns1` namespace migrated success, the user wants to change the resources managed by the original Policy under 
the `ns2` namespace to be managed by the new Policy, he just needs to continue to update the original Policy as follows.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp
spec:
  priority: 10
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
    - apiVersion: v1
      kind: ConfigMap
  excludedResources:
    - namespace: ns1
    - namespace: ns2     # exclude ns2 namespace resources for observation and confirmation
```

Then he can observe and confirm that the migration results of resources under the `ns2` namespace meet expectations.

Besides, if there are other critical namespaces need to be gradually rolled out, by analogy.

#### Step 4

Delete the original Policy so that all the left referred resources can be token over by new Policy.

After all this, change the `priority` of new Policy back to the same as original Policy, and this iterative migration done successful.

### Scheme Rationality

This feature allows Policy to filter resources more flexibly.

### Notes/Constraints/Caveats

none

## Design Details

### API change

```go
// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
    ......
    // ExcludedResources used to exclude resources.
    // It consists of a slice of exclusion rules and a resource will be excluded
    // if it matches any of the rules.
    //
    // Note that, if a resource has been propagated and then excluded, its behavior
    // is equivalent to the deletion of PropagationPolicy or ClusterPropagationPolicy.
    // That is the propagated resource will continue to exist in member cluster(s)
    // until it is removed or took over by another policy.
    // +optional
    ExcludedResources []ExcludedResource `json:"excludedResources,omitempty"`
    ......
}

// ExcludedResource specifies which resources need to be excluded.
type ExcludedResource struct {
    // APIVersion represents the API version of the target resources.
    // If not empty, resources under this API group will be excluded.
    // +optional
    APIVersion string `json:"apiVersion,omitempty"`
    
    // Kind represents the Kind of the target resources.
    // If not empty, resources with this Kind will be excluded.
    // +optional
    Kind string `json:"kind,omitempty"`
    
    // Namespace of the target resource.
    // If not empty, resources under this namespace will be excluded.
    // +optional
    Namespace string `json:"namespace,omitempty"`
    
    // Name of the target resource.
    // If not empty, resources with this name will be excluded.
    // +optional
    Name string `json:"name,omitempty"`
    
    // A label query over a set of resources.
    // If not empty, resources match the label query will be excluded.
    // +optional
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}
```

### System Behavior Design

The `excludedResources` field has five sub-field, then how to judge `excludedResources` match the resource?

* If only one sub-field is set, like only `namespace` is set, then if the resource's `namespace` is matchable, the resource
  should be excluded by this Policy.
* If multi sub-field is set, only when all the corresponding sub-field matches, the resource can be excluded by this Policy.
  e.g: the `apiVersion`、`kind` and `labelSelector` are all set in `excludedResources`, if a resource only matches the 
  `apiVersion` and `kind`, it can't be excluded, while if it matches all these three sub-field, it shall be excluded.