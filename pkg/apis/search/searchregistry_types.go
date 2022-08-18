package search

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceRegistry defines a list of member cluster to be cached.
type ResourceRegistry struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec represents the desired behavior of ResourceRegistry.
	Spec ResourceRegistrySpec

	// Status represents the status of ResourceRegistry.
	// +optional
	Status ResourceRegistryStatus
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
	APIVersion string

	// Kind represents the kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +optional
	Namespace string
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
	Conditions []metav1.Condition
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceRegistryList if a collection of ResourceRegistry.
type ResourceRegistryList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items holds a list of ResourceRegistry.
	Items []ResourceRegistry
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Search define a flag for resource search that do not have actual resources.
type Search struct {
	metav1.TypeMeta
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Proxying define a flag for resource proxying that do not have actual resources.
type Proxying struct {
	metav1.TypeMeta
}
