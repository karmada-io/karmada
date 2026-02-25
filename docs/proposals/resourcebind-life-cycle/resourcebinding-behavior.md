---
title: Behavior Of ResourceBind When ProPagation Policy ' s ResourceSelector Change

authors:
- "@olderTaoist"

reviewers:
- "@XiShanYongYe - Chang"
- "@weilaaa"

approvers:

creation-date: 2023-06-27

---

# Behavior Of ResourceBind When ProPagation Policy ' s ResourceSelector Change

## Summary

Current there are two situations that `ResourceBinding` will be deleted :
- the releated resource template be deleted
- the releated propagation policy be deleted
  
When change ResourceSelector of `ProPagationPolicy` or `ClusterProPagationPolicy`, The `ResourceBinding` of older 
matched resource template is still exist , But users want `ResourceBinding` to be deleted in some scenario.
  
This proposal aims to provide a solution for `ResourceBinding` whether to be deleted depend on users.


## Motivation

Users want controller the behavior(delete or reserve) of ResourceBinding when change ResourceSelector 
of `ProPagationPolicy` or `ClusterProPagationPolicy`

### Goals

- Provide a solution to custom behavior(delete or reserve) of `ResourceBinding` depend on users.


### Non-Goals


## Proposal


### User Stories (Optional)


#### As a user , I want resourcebinding to be deleted .

There some resource in member clusters controller by karmada with a propagation policy. When i change the ResourceSelector
of `ProPagationPolicy`, I want old resources(not match new ResourceSelector) in member clusters to be deleted,
The current situation is the old resource still exist in member cluster.



### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

add `NotAllowOrphanRB` filed in `ResourceSelector` struct. User can use `NotAllowOrphanRB` field determinate whether to
delete `ResourceBinding` when change `ResourceSelector` 

### Older ResourceSelector Struct

```go
// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	APIVersion    string `json:"apiVersion"`
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace,omitempty"`
	Name          string `json:"name,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}
```

### New ResourceSelector Struct

```go
// ResourceSelector the resources will be selected.
type ResourceSelector struct {
    APIVersion    string `json:"apiVersion"`
    Kind          string `json:"kind"`
    Namespace     string `json:"namespace,omitempty"`
    Name          string `json:"name,omitempty"`
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

    // delete or reserve resourcebinding when change resource selector .
    // Default is false , which means reserve resourcebinding .
    // +optional 
    NotAllowOrphanRB bool `json:"notAllowOrphanRB,omitempty"`

}
```

### Example
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: karmada-pod-policy
  namespace: j036x0- paas
spec :
  placement :
    clusterAffinity :
      clusterNames :
      - 312c282ea8826b7f
      - 3e5f993f63c1f0cc
  priority: 0
  resourceSelectors :
  - apiVersion: apps/v1
    kind: Deployment
    name: nacos-operator-karmada
    namespace: j036x0-paas
    notAllowOrphanRB : true
  schedulerName : default - scheduler


```


### Test Plan


## Alternatives
