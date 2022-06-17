---
title: Adoption of ClusterID

authors:
- "@XiShanYongYe-Chang"
- "@RainbowMango"

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-06-16

---

# Adoption of ClusterID

## Summary

In the progress of registering a cluster into Karmada, we need to confirm the cluster identity to prevent the cluster from being added to Karmada repeatedly. [Kubernetes sig-multicluster](https://github.com/kubernetes/community/tree/master/sig-multicluster) have made a KEP: [ClusterId for ClusterSet identification](https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/2149-clusterid#kep-2149-clusterid-for-clusterset-identification) to identify a cluster in a Kubernetes-native way. We want to use this [ClusterID](https://github.com/kubernetes-sigs/about-api) to confirm the cluster identifier. 

## Motivation

Kubernetes' enhancements has proposed a [KEP-2149](https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/2149-clusterid). This KEP proposes an API called `ClusterProperty` which provides a way to store identification related, cluster scoped information. We can adopt this API to identify each cluster that joins Karmada.

### Goals

- Set the cluster identifier for the cluster that has been joined to Karmada.

### Non-Goals

- The API describing cluster properties upgrade.

## Proposal

### User Stories (Optional)

#### Story 1

As a cluster administrator, I want each cluster could be assigned with a unique ID, so that the registering process can be rejected if a cluster tries to register with another name.

[Issue](https://github.com/karmada-io/karmada/issues/880) raises this situation.

#### Story 2

As a cluster administrator, when I create a `id.k8s.io ClusterProperty` in a kubernetes cluster, I want to use it as its cluster ID after joining Karmada, so that to uniquely identify the cluster.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

We can divide the implementation into two phases:

- Using cluster's `kube-system` namespace uuid as the cluster id, and collect it into the [Cluster](https://github.com/karmada-io/karmada/blob/3039ebbba31358fc6d8a1a0374cef09dfc17cf5e/pkg/apis/cluster/v1alpha1/types.go#L26) object.
- Adopting `ClusterProperty` CRD to identify a cluster in Karmada.

There are two reasons for this: First, we want to fix the problem of repeat cluster join as soon as possible (preferably in v1.3); and second, we need to wait for the organization structure of Project [about-api](https://github.com/kubernetes-sigs/about-api) to be adjusted so that we can import the `ClusterProperty` API.

### Phase one: collect kube-system ns uuid

We propose to introduce an `ID` filed to the Cluster API:

```go
// ClusterSpec defines the desired state of a member cluster.
type ClusterSpec struct {
	// ID represents the cluster's unique identifier.
    // +optional
    ID string `json:"ID"`
  
	...
}
```

During the cluster joining process, Karmada will just collect kubernetes cluster's `kube-system` namespace uuid as the cluster id.

To ensure the uniqueness of the cluster in Karmada, we need to traverse all clusters that have been joined to Karmada and checkout whether their id are same as the cluster to be joined to Karmada. If they are the same, we will reject the join request, otherwise accept it.

This check can be performed in two places: one in the join phase and the other in the Cluster object creation phase. We can choose one of them.

### Phase two: adopt ClusterProperty API

After waiting for the organization of the `about-io` project to be restructured, we can successfully import the `ClusterProperty` API.

We propose to introduce a `ClusterProperty` filed to the Cluster API:

```go
// ClusterStatus contains information about the current status of a
// cluster updated periodically by cluster controller.
type ClusterStatus struct {
    // ClusterProperty represents the cluster identity information.
    // +optional
    ClusterProperty map[string]string `json:"clusterProperty,omitempty"`
  
	...
}
```

We provide a flag (such as `create-clusterproperty-not-exist`) to tell Karmada whether to create `id.k8s.io ClusterProperty` when it does not exist in the Kubernetes Cluster. During the cluster joining process, Karmada will look for if the kubernetes cluster has defined the cluster identifier by `id.k8s.io ClusterProperty`. If so, we just take the `id.k8s.io ClusterProperty` value as cluster id; otherwise, when the `create-clusterproperty-not-exist` flag is true, we'll just use kubernetes cluster's `kube-system` namespace uuid to create `id.k8s.io ClusterProperty`, when the `create-clusterproperty-not-exist` flag is false, we refuse to join the cluster into Karmada.

> Note: We will use the latest version of ClusterProperty CRD to create `id.k8s.io ClusterProperty`. If the ClusterProperty version in the kubernetes cluster does not match, the user needs to upgrade the ClusterProperty version.

Just as phase one, we need to use the same check to ensure the uniqueness of the cluster in Karmada.

Again, this check can be performed in two places: one in the join phase and the other in the Cluster object creation phase. We can choose one of them.

When a cluster is removed from Karmada, `id.k8s.io ClusterProperty` created in the cluster do not need to be deleted.

### Test Plan

#### UT

Test cover the new method.

#### E2E

Add a case to test repeatedly joining a cluster into the Karmada.

## Alternatives
