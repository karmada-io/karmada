---
title: Caching member cluster resources for Karmada
authors:
- "@huntsman-li"
- "@liys87x"
reviewers:
approvers:

creation-date: 2022-02-14

---

# Caching member cluster resources for Karmada

## Summary

Provides a caching layer for Karmada to cache (member clusters) Kubernetes resources.

## Motivation

### Goals

- Accelerates resource requests processing speed across regions
- Provides a cross-cluster resource view
- Compatible with (multi-cluster) multiple kubernetes resource versions
- Unified resource requests entries
- Reduces API server pressure of member clusters

### Non-Goals

## Proposal

### User Stories (Optional)

#### Story 1

Imagine that we have a cross-region application that needs to run in multiple regions in order to provide service capabilities nearby. We achieve this by deploying a Kubernetes cluster in each region.

Goals:

- Get the distribution of the application in different clusters.
- Get resource information across clusters through a unified endpoint.
- Accelerate processing speed of resource requests across regions.
- Get resources in multiple clusters by labels.

## Design Details

### Define the scope of the cached resource

#### New ClusterCache API

We propose a new CR in `cluster.karmada.io` group.

```golang

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterCache is the Schema for the cluster cache API
type ClusterCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterCacheSpec   `json:"spec,omitempty"`
	Status ClusterCacheStatus `json:"status,omitempty"`
}

// ClusterCacheSpec defines the desired state of ClusterCache
type ClusterCacheSpec struct {
	// ClusterSelectors represents the filter to select clusters.
	// +required
	ClusterSelectors []ClusterSelector `json:"clusterSelectors"`

	// ResourceSelectors used to select resources.
	// +required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// StatusUpdatePeriodSeconds is the period to update the status of the resource.
	// default is 10s.
	// +optional
	StatusUpdatePeriodSeconds uint32 `json:"statusUpdatePeriodSeconds,omitempty"`
}

// ClusterAffinity represents the filter to select clusters.
type ClusterSelector struct {
	// LabelSelector is a filter to select member clusters by labels.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// FieldSelector is a filter to select member clusters by fields.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	FieldSelector *FieldSelector `json:"fieldSelector,omitempty"`

	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`

	// ExcludedClusters is the list of clusters to be ignored.
	// +optional
	ExcludeClusters []string `json:"exclude,omitempty"`
}

// FieldSelector is a field filter.
type FieldSelector struct {
	// A list of field selector requirements.
	MatchExpressions []corev1.NodeSelectorRequirement `json:"matchExpressions,omitempty"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ClusterCacheStatus defines the observed state of ClusterCache
type ClusterCacheStatus struct {
	// +optional
	Resources []ResourceStatusRef `json:"resources,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

type ResourceStatusRef struct {
	// +required
	Cluster string `json:"cluster"`
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// +required
	Kind string `json:"kind"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	State CachePhase `json:"state"`
	// +required
	TotalNum int32 `json:"totalNum"`
	// +required
	UpdateTime *metav1.Time `json:"updateTime"`
}

// CachePhase is the current state of the cache
// +enum
type CachePhase string

// These are the valid statuses of cache.
const (
	CacheRunning CachePhase = "Running"
	CacheFailed  CachePhase = "Failed"
	CacheUnknown CachePhase = "Unknown"
)

type ResourceStateRef struct {
	// +required
	Phase CachePhase `json:"phase"`
	// +optional
	Reason string `json:"reason"`
}

//+kubebuilder:object:root=true

// ClusterCacheList contains a list of ClusterCache
type ClusterCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCache `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterCache{}, &ClusterCacheList{})
}
```

#### Example

The following example shows how to create a ClusterCache CRD.

```yaml
apiVersion: clustercaches.karmada.io/v1alpha1
kind: ClusterCache
metadata:
  name: clustercache-sample
spec:
  clusterSelectors:
    - clusterNames:
        - member1
        - member2
        - member3
  resourceSelectors:
    - kind: Pod
    - kind: Ingress
      apiVersion: networking.k8s.io/v1
    - kind: DaemonSet
      namespace: kube-system
    - kind: Deployment
status:
  startTime: "2020-05-01T00:00:00Z"
  resources:
    - cluster: member1
      kind: Pod
      totalNum: 700
      state:
        phase: Running
      updateTime: "2022-01-01T00:00:00Z"
    - cluster: member1
      kind: Ingress
      totalNum: 0
      state:
        phase: Failed
        reason: the server doesn't have a resource type ingresses
      updateTime: "2022-01-01T00:00:00Z"
```

### Test Plan
