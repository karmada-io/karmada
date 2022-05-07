package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindResourceRegistry is the name of the resource registry
	ResourceKindResourceRegistry = "ResourceRegistry"
	// ResourceSingularResourceRegistry is singular name of ResourceRegistry.
	ResourceSingularResourceRegistry = "resourceRegistry"
	// ResourcePluralResourceRegistry is plural name of ResourceRegistry.
	ResourcePluralResourceRegistry = "resourceRegistries"
	// ResourceNamespaceScopedResourceRegistry is the scope of the ResourceRegistry
	ResourceNamespaceScopedResourceRegistry = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"

// ResourceRegistry defines a list of member cluster to be cached.
type ResourceRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ResourceRegistry.
	Spec ResourceRegistrySpec `json:"spec,omitempty"`

	// Status represents the status of ResoruceRegistry.
	// +optional
	Status ResourceRegistryStatus `json:"status,omitempty"`
}

// ResourceRegistrySpec defines the desired state of ResourceRegistry.
type ResourceRegistrySpec struct {
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

// ClusterSelector represents the filter to select clusters.
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
