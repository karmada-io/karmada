---
title: Your short, descriptive title
authors:
- "@robot" # Authors' github accounts here.
reviewers:
- "@robot"
- TBD
approvers:
- "@robot"
- TBD

creation-date: yyyy-mm-dd

---

# Your short, descriptive title

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

## Summary

With the development of multi-cluster technology, some users begin to want to migrate stateful applications to multi-cluster scenarios, which cannot be covered well by the current version of karmada. Each user needs to implement a `karmada-operator` that does something similar, so we can propose an API to handle the common coordination related work logic, while setting aside a mechanism for the user's `karmada-operator` to complete the specific logic, such as rolling upgrades, scaling, etc.

## Motivation

Provides a new API for stateful applications across clusters, enabling karmada to adapt to stateful applications across clusters.

An obvious advantage is that karmada natively supports cross-cluster Statefulset, preventing each user from repeatedly implementing his own cross-cluster Statefulset.

### Goals

- Defining an API(crossClusterStatefulset) enables users to implement specific logic for stateful operator applications across clusters.
-  Propose the implementation ideas for involved components, including the new controller of `cross_cluster_statefulset_controller` in karmada-controller-manager.



### Non-Goals

## Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

### User Stories (Optional)

#### As a user, I want to deploy a cross cluster stateful app.

In order to enable my stateful application to achieve disaster recovery in multiple clusters and obtain better regional services,I want to let my kubernetes operator from member cluster can working for cross cluster stateful with karmada and not need to implement other operator for karmada.

#### Story 1

#### Story 2

### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? 

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

## Design Details


### New API


```golang
type CrossClusterStatefulsetSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the MultiClusterIngress.
	// +optional
	Spec CrossClusterStatefulsetSetSpec `json:"spec,omitempty"`
}

// CrossClusterStatefulsetSetSpec is the desired state of the CrossClusterStatefulsetSet.
type CrossClusterStatefulsetSetSpec struct {
	ResourceSelector policyv1alpha1.ResourceSelector `json:"resourceSelector,omitempty"`
  // updateStrategy indicates the CrossClusterStatefulsetSetStrategy that will be
	// employed to update CRs when a revision is made to Template.
	UpdateStrategy CrossClusterStatefulSetUpdateStrategy
}

// StatefulSetUpdateStrategy indicates the strategy that the StatefulSet
// controller will use to perform updates. It includes any additional parameters
// necessary to perform the update for the indicated strategy.
type CrossClusterStatefulSetUpdateStrategy struct {
	// Type indicates the type of the CrossClusterStatefulSetUpdateStrategy.
	Type CrossClusterStatefulSetUpdateStrategyType
}

// CrossClusterStatefulSetUpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the CrossClusterStatefulSet controller.
type CrossClusterStatefulUpdateStrategyType string

const (
	// RollingUpdateCrossClusterStatefulSetStrategyType indicates that update will be
	// applied to all CRs in the StatefulSet with respect to the StatefulSet
	// ordering constraints. When a scale operation is performed with this
	// strategy, new CR will be created from the specification version indicated
	// by the CrossClusterStatefulSet's updateRevision.
	RollingUpdateCrossClusterStatefulSetStrategyType CrossClusterStatefulSetUpdateStrategyType = "RollingUpdate"
)

```

### Two part here for user CRD

### propagate schedule result to all of member cluster for CRD

propagate schedule result with CRD annotation,just like:

```yaml
# member1
Name:         xline
Namespace:    xline
API Version:  apps.my.io/v1alpha1
Kind:         XlineCluster
Annotations:
    corssclusterstatefulset.karmada.io/replicas: [
      {"member1":"1","self":true},
      {"member1":"2","self":false}
      ]
...
---
# member2
Name:         xline
Namespace:    xline
API Version:  apps.my.io/v1alpha1
Kind:         XlineCluster
Annotations:
    corssclusterstatefulset.karmada.io/replicas: [
      {"member1":"1","self":false},
      {"member1":"2","self":true}
      ]
...
```

The above yaml means the member cluster of member1 have 1 replcas for this resource and member cluster of member2 have 1 replicas of this resource. The current cluster CR monitored by each member cluster will be different. The annotation of `self` indicates whether it is the number of copies required for the CR resource in the current cluster. It is mainly used to establish the network topology of global resources.

Then the user's operator can use this messgae to init they app cluster at member cluster, Before to really init cluster, they will splice the network so that resources between clusters can access each other, like: `xline-0=xline-0.member1.karmada,xline-1=xline-1.member2.karmada`,It depends on how the user connects to the network of member clusters, which is not within the scope of the API's responsibilities.

### Strategy for rollingupdate

#### First step: decide which ones member cluster is processer 

Propagate processor with CRD annotation:


```yaml
# member1
Name:         xline
Namespace:    xline
API Version:  apps.my.io/v1alpha1
Kind:         XlineCluster
Annotations:
    corssclusterstatefulset.karmada.io/processer: true #true/false
---
# member2
Name:         xline
Namespace:    xline
API Version:  apps.my.io/v1alpha1
Kind:         XlineCluster
Annotations:
    corssclusterstatefulset.karmada.io/processer: true #true/false
```

The above yaml means the member cluster of member1 & member2 need to create/update this resource.

Then the user's operator can use this information to determine whether you need to perform a create/update operation.

#### Second step: cross_cluster_statefulset_controller determines whether the user's operator in the member cluster has completed the update.

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: CrossClusterStatefulset
metadata:
  name: xline
spec:
  resourceSelector:
    apiVersion: apps.my.io/v1alpha1
    kind: XlineCluster
    name: xline
    namespace: default
  updateStrategy:
    type: RollingUpdate
```

In order to check whether the update has been performed, the user must specify a ResourceInterpreterCustomization or webhook corresponding to the resource,And then `cross_cluster_statefulset_controller` will check the InterpretHealth or a new operation of Interpreter if necessarily.


### Test Plan

<!--
**Note:** *Not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all test cases, just the general strategy. Anything
that would count as tricky in the implementation, and anything particularly
challenging to test, should be called out.

-->

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->