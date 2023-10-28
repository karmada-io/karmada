package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

const (
	// ResourceKindResourceRegistry is the name of the resource registry
	ResourceKindResourceRegistry = "ResourceRegistry"
	// ResourceSingularResourceRegistry is singular name of ResourceRegistry.
	ResourceSingularResourceRegistry = "resourceregistry"
	// ResourcePluralResourceRegistry is plural name of ResourceRegistry.
	ResourcePluralResourceRegistry = "resourceregistries"
	// ResourceNamespaceScopedResourceRegistry is the scope of the ResourceRegistry
	ResourceNamespaceScopedResourceRegistry = false
)

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

// ResourceRegistryList is a collection of ResourceRegistry.
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Proxying define a flag for resource proxying that do not have actual resources.
type Proxying struct {
	metav1.TypeMeta `json:",inline"`
}
