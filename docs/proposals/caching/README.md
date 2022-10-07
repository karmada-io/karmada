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

#### New Search APIGroup

We propose a new component named `karmada-search`, it provides a new api group called `search.karmada.io`, the reason why select the `search` word is because it's more to [OpenSearch](https://opensearch.org) which is a community-driven, Apache 2.0-licensed open source search and analytics suite.

The `karmada-search` component currently supports two types of backend stores, namely `cache` and `opensearch`. It uses the `cache` type as the default backend store of the caching layer for Karmada.

We introduce a new resource type called `ResourceRegistry` in `search.karmada.io` group, it requires end users to manually specify the clusters and resources that need to be cached


```golang
package v1alpha1

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceRegistry represents the configuration of the cache scope, mainly describes which resources in
// which clusters should be cached.
type ResourceRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ResourceRegistry.
	Spec ResourceRegistrySpec `json:"spec,omitempty"`

	// Status represents the status of ResourceRegistry.
	// +optional
	Status ResourceRegistryStatus `json:"status,omitempty"`
}

// ResourceRegistrySpec defines the desired state of ResourceRegistry.
type ResourceRegistrySpec struct {
	// TargetCluster specifies the clusters where the cache system collect resource from.
	// +required
	TargetCluster policyv1alpha1.ClusterAffinity `json:"targetCluster"`

	// ResourceSelectors specifies the resources type that should be cached by cache system.
	// +required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// BackendStore specifies the location where to store the cached items.
	// +optional
	BackendStore *BackendStoreConfig `json:"backendStore,omitempty"`
}

// ResourceSelector specifies the resources type and its scope.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// BackendStoreConfig specifies backend store.
type BackendStoreConfig struct {
	// OpenSearch is a community-driven, open source search and analytics suite.
	// Refer to website(https://opensearch.org/) for more details about OpenSearch.
	// +optional
	OpenSearch *OpenSearchConfig `json:"openSearch,omitempty"`
}

// OpenSearchConfig holds the necessary configuration for client to access and config an OpenSearch server.
type OpenSearchConfig struct {
	// Addresses is a list of node endpoint(e.g. 'https://localhost:9200') to use.
	// For the 'node' concept, please refer to:
	// https://opensearch.org/docs/latest/opensearch/index/#clusters-and-nodes
	// +required
	Addresses []string `json:"addresses"`

	// SecretRef represents the secret contains mandatory credentials to access the server.
	// The secret should hold credentials as follows:
	// - secret.data.userName
	// - secret.data.password
	// +required
	SecretRef clusterv1alpha1.LocalSecretReference `json:"secretRef,omitempty"`

	// More configurations such as transport, index should be added from here.
}

// ResourceRegistryStatus defines the observed state of ResourceRegistry
type ResourceRegistryStatus struct {
	// Conditions contain the different condition statuses.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceRegistryList if a collection of ResourceRegistry.
type ResourceRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ResourceRegistry.
	Items []ResourceRegistry `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Search define a flag for resource search that do not have actual resources.
type Search struct {
	metav1.TypeMeta `json:",inline"`
}
```

#### Example

The following example shows how to create a ResourceRegistry CR.

```yaml
apiVersion: search.karmada.io/v1alpha1
kind:  ResourceRegistry
metadata:
  name: clustercache-sample
spec:
  targetCluster:
    clusterNames:
    - member1
    - member2
    - member3
  resourceSelectors:
    - kind: Pod
      apiVersion: v1
    - kind: Ingress
      apiVersion: networking.k8s.io/v1
    - kind: DaemonSet
      apiVersion: apps/v1
      namespace: kube-system
    - kind: Deployment
      apiVersion: apps/v1
```

### Test Plan
