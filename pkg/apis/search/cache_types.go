package search

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Status represents the status of ResoruceRegistry.
	// +optional
	Status ResourceRegistryStatus
}

// ResourceRegistrySpec defines the desired state of ResourceRegistry.
type ResourceRegistrySpec struct {
	// ClusterSelectors represents the filter to select clusters.
	// +required
	ClusterSelectors []ClusterSelector

	// ResourceSelectors used to select resources.
	// +required
	ResourceSelectors []ResourceSelector

	// StatusUpdatePeriodSeconds is the period to update the status of the resource.
	// default is 10s.
	// +optional
	StatusUpdatePeriodSeconds uint32
}

// ClusterSelector represents the filter to select clusters.
type ClusterSelector struct {
	// LabelSelector is a filter to select member clusters by labels.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	LabelSelector *metav1.LabelSelector

	// FieldSelector is a filter to select member clusters by fields.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	FieldSelector *FieldSelector

	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string

	// ExcludedClusters is the list of clusters to be ignored.
	// +optional
	ExcludeClusters []string
}

// FieldSelector is a field filter.
type FieldSelector struct {
	// A list of field selector requirements.
	MatchExpressions []corev1.NodeSelectorRequirement
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string

	// Kind represents the Kind of the target resources.
	// +required
	Kind string

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +optional
	Namespace string
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
