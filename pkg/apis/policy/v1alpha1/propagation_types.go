package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pp

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

// SpreadFieldValue is the type to define valid values for SpreadConstraint.SpreadByField
type SpreadFieldValue string

// Available fields for spreading are: cluster, region, zone, and provider.
const (
	SpreadByFieldCluster  SpreadFieldValue = "cluster"
	SpreadByFieldRegion   SpreadFieldValue = "region"
	SpreadByFieldZone     SpreadFieldValue = "zone"
	SpreadByFieldProvider SpreadFieldValue = "provider"
)

// SpreadConstraint represents the spread constraints on resources.
type SpreadConstraint struct {
	// SpreadByField represents the fields on Karmada cluster API used for
	// dynamically grouping member clusters into different groups.
	// Resources will be spread among different cluster groups.
	// Available fields for spreading are: cluster, region, zone, and provider.
	// SpreadByField should not co-exist with SpreadByLabel.
	// If both SpreadByField and SpreadByLabel are empty, SpreadByField will be set to "cluster" by system.
	// +kubebuilder:validation:Enum=cluster;region;zone;provider
	// +optional
	SpreadByField SpreadFieldValue `json:"spreadByField,omitempty"`

	// SpreadByLabel represents the label key used for
	// grouping member clusters into different groups.
	// Resources will be spread among different cluster groups.
	// SpreadByLabel should not co-exist with SpreadByField.
	// +optional
	SpreadByLabel string `json:"spreadByLabel,omitempty"`

	// MaxGroups restricts the maximum number of cluster groups to be selected.
	// +optional
	MaxGroups int `json:"maxGroups,omitempty"`

	// MinGroups restricts the minimum number of cluster groups to be selected.
	// Defaults to 1.
	// +optional
	MinGroups int `json:"minGroups,omitempty"`
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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster",shortName=cpp
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterPropagationPolicy represents the cluster-wide policy that propagates a group of resources to one or more clusters.
// Different with PropagationPolicy that could only propagate resources in its own namespace, ClusterPropagationPolicy
// is able to propagate cluster level resources and resources in any namespace other than system reserved ones.
// System reserved namespaces are: karmada-system, karmada-cluster, karmada-es-*.
type ClusterPropagationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterPropagationPolicy.
	Spec PropagationSpec `json:"spec"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterPropagationPolicyList contains a list of ClusterPropagationPolicy.
type ClusterPropagationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPropagationPolicy `json:"items"`
}
