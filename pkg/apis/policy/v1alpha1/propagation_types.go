package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// PropagationPolicy represents the policy that propagates a group of resources to one or more clusters.
type PropagationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of PropagationPolicy.
	Spec PropagationSpec `json:"spec"`
}

// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
	// ResourceSelectors used to select resources.
	// nil represents all resources.
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// Association tells if relevant resources should be selected automatically.
	// e.g. a ConfigMap referred by a Deployment.
	// default false.
	// +optional
	Association bool `json:"association,omitempty"`

	// Placement represents the rule for select clusters to propagate resources.
	Placement Placement `json:"placement,omitempty"`

	// DependentOverrides represents the list of overrides(OverridePolicy)
	// which must present before the current PropagationPolicy takes effect.
	//
	// It used to explicitly specify overrides which current PropagationPolicy rely on.
	// A typical scenario is the users create OverridePolicy(ies) and resources at the same time,
	// they want to ensure the new-created policies would be adopted.
	//
	// Note: For the overrides, OverridePolicy(ies) in current namespace and ClusterOverridePolicy(ies),
	// which not present in this list will still be applied if they matches the resources.
	// +optional
	DependentOverrides []string `json:"dependentOverrides,omitempty"`

	// SchedulerName represents which scheduler to proceed the scheduling.
	// If specified, the policy will be dispatched by specified scheduler.
	// If not specified, the policy will be dispatched by default scheduler.
	SchedulerName string `json:"schedulerName,omitempty"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means inherit from the parent object scope.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the target resource.
	// Default is empty, which means selecting all resources.
	// +optional
	Name string `json:"name,omitempty"`

	// A label query over a set of resources.
	// If name is not empty, labelSelector will be ignored.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// FieldSelector is a field filter.
type FieldSelector struct {
	// A list of field selector requirements.
	MatchExpressions []corev1.NodeSelectorRequirement `json:"matchExpressions,omitempty"`
}

// Placement represents the rule for select clusters.
type Placement struct {
	// ClusterAffinity represents scheduling restrictions to a certain set of clusters.
	// If not set, any cluster can be scheduling candidate.
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// ClusterTolerations represents the tolerations.
	ClusterTolerations []corev1.Toleration `json:"clusterTolerations,omitempty"`

	// SpreadConstraints represents a list of the scheduling constraints.
	SpreadConstraints []SpreadConstraint `json:"spreadConstraints,omitempty"`
}

// SpreadConstraint represents the spread constraints on resources.
type SpreadConstraint struct {
	// SpreadByField represents the field used for grouping member clusters into units.
	// Resources will be spread among different cluster units.
	// Available field for spreading are: region, zone, cluster and provider.
	// +optional
	SpreadByField string `json:"spreadByField,omitempty"`

	// SpreadByLabel represents the label key used for grouping member clusters into units.
	// Resources will be spread among different cluster units.
	// +optional
	SpreadByLabel string `json:"spreadByLabel,omitempty"`

	// Maximum restricts the maximum number of cluster units to be selected.
	// +optional
	Maximum int `json:"maximum,omitempty"`

	// Minimum restricts the minimum number of cluster units to be selected.
	// +optional
	Minimum int `json:"minimum,omitempty"`
}

// ClusterAffinity represents the filter to select clusters.
type ClusterAffinity struct {
	// LabelSelector is a filter to select member clusters by labels.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// FieldSelector is a filter to select member clusters by fields.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	FieldSelector *FieldSelector `json:"fieldSelector,omitempty"`

	// ClusterNames is the list of clusters to be selected.
	ClusterNames []string `json:"clusterNames,omitempty"`

	// ExcludedClusters is the list of clusters to be ignored.
	ExcludeClusters []string `json:"exclude,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PropagationPolicyList contains a list of PropagationPolicy.
type PropagationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PropagationPolicy `json:"items"`
}
