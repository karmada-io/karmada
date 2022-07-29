package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindPropagationPolicy is kind name of PropagationPolicy.
	ResourceKindPropagationPolicy = "PropagationPolicy"
	// ResourceSingularPropagationPolicy is singular name of PropagationPolicy.
	ResourceSingularPropagationPolicy = "propagationpolicy"
	// ResourcePluralPropagationPolicy is kind plural name of PropagationPolicy.
	ResourcePluralPropagationPolicy = "propagationpolicies"
	// ResourceNamespaceScopedPropagationPolicy indicates if PropagationPolicy is NamespaceScoped.
	ResourceNamespaceScopedPropagationPolicy = true

	// ResourceKindClusterPropagationPolicy is kind name of ClusterPropagationPolicy.
	ResourceKindClusterPropagationPolicy = "ClusterPropagationPolicy"
	// ResourceSingularClusterPropagationPolicy is singular name of ClusterPropagationPolicy.
	ResourceSingularClusterPropagationPolicy = "clusterpropagationpolicy"
	// ResourcePluralClusterPropagationPolicy is plural name of ClusterPropagationPolicy.
	ResourcePluralClusterPropagationPolicy = "clusterpropagationpolicies"
	// ResourceNamespaceScopedClusterPropagationPolicy indicates if ClusterPropagationPolicy is NamespaceScoped.
	ResourceNamespaceScopedClusterPropagationPolicy = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=pp,categories={karmada-io}

// PropagationPolicy represents the policy that propagates a group of resources to one or more clusters.
type PropagationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of PropagationPolicy.
	// +required
	Spec PropagationSpec `json:"spec"`
}

// PropagationSpec represents the desired behavior of PropagationPolicy.
type PropagationSpec struct {
	// ResourceSelectors used to select resources.
	// +required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// Association tells if relevant resources should be selected automatically.
	// e.g. a ConfigMap referred by a Deployment.
	// default false.
	// Deprecated: in favor of PropagateDeps.
	// +optional
	Association bool `json:"association,omitempty"`

	// PropagateDeps tells if relevant resources should be propagated automatically.
	// Take 'Deployment' which referencing 'ConfigMap' and 'Secret' as an example, when 'propagateDeps' is 'true',
	// the referencing resources could be omitted(for saving config effort) from 'resourceSelectors' as they will be
	// propagated along with the Deployment. In addition to the propagating process, the referencing resources will be
	// migrated along with the Deployment in the fail-over scenario.
	//
	// Defaults to false.
	// +optional
	PropagateDeps bool `json:"propagateDeps,omitempty"`

	// Placement represents the rule for select clusters to propagate resources.
	// +optional
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
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`
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
	// +optional
	ClusterTolerations []corev1.Toleration `json:"clusterTolerations,omitempty"`

	// SpreadConstraints represents a list of the scheduling constraints.
	// +optional
	SpreadConstraints []SpreadConstraint `json:"spreadConstraints,omitempty"`

	// ReplicaScheduling represents the scheduling policy on dealing with the number of replicas
	// when propagating resources that have replicas in spec (e.g. deployments, statefulsets) to member clusters.
	// +optional
	ReplicaScheduling *ReplicaSchedulingStrategy `json:"replicaScheduling,omitempty"`
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

// ReplicaSchedulingType describes scheduling methods for the "replicas" in a resource.
type ReplicaSchedulingType string

const (
	// ReplicaSchedulingTypeDuplicated means when propagating a resource,
	// each candidate member cluster will directly apply the original replicas.
	ReplicaSchedulingTypeDuplicated ReplicaSchedulingType = "Duplicated"
	// ReplicaSchedulingTypeDivided means when propagating a resource,
	// each candidate member cluster will get only a part of original replicas.
	ReplicaSchedulingTypeDivided ReplicaSchedulingType = "Divided"
)

// ReplicaDivisionPreference describes options of how replicas can be scheduled.
type ReplicaDivisionPreference string

const (
	// ReplicaDivisionPreferenceAggregated divides replicas into clusters as few as possible,
	// while respecting clusters' resource availabilities during the division.
	ReplicaDivisionPreferenceAggregated ReplicaDivisionPreference = "Aggregated"
	// ReplicaDivisionPreferenceWeighted divides replicas by weight according to WeightPreference.
	ReplicaDivisionPreferenceWeighted ReplicaDivisionPreference = "Weighted"
)

// ReplicaSchedulingStrategy represents the assignment strategy of replicas.
type ReplicaSchedulingStrategy struct {
	// ReplicaSchedulingType determines how the replicas is scheduled when karmada propagating
	// a resource. Valid options are Duplicated and Divided.
	// "Duplicated" duplicates the same replicas to each candidate member cluster from resource.
	// "Divided" divides replicas into parts according to number of valid candidate member
	// clusters, and exact replicas for each cluster are determined by ReplicaDivisionPreference.
	// +kubebuilder:validation:Enum=Duplicated;Divided
	// +optional
	ReplicaSchedulingType ReplicaSchedulingType `json:"replicaSchedulingType,omitempty"`

	// ReplicaDivisionPreference determines how the replicas is divided
	// when ReplicaSchedulingType is "Divided". Valid options are Aggregated and Weighted.
	// "Aggregated" divides replicas into clusters as few as possible,
	// while respecting clusters' resource availabilities during the division.
	// "Weighted" divides replicas by weight according to WeightPreference.
	// +kubebuilder:validation:Enum=Aggregated;Weighted
	// +optional
	ReplicaDivisionPreference ReplicaDivisionPreference `json:"replicaDivisionPreference,omitempty"`

	// WeightPreference describes weight for each cluster or for each group of cluster
	// If ReplicaDivisionPreference is set to "Weighted", and WeightPreference is not set, scheduler will weight all clusters the same.
	// +optional
	WeightPreference *ClusterPreferences `json:"weightPreference,omitempty"`
}

// ClusterPreferences describes weight for each cluster or for each group of cluster.
type ClusterPreferences struct {
	// StaticWeightList defines the static cluster weight.
	// +optional
	StaticWeightList []StaticClusterWeight `json:"staticWeightList,omitempty"`
	// DynamicWeight specifies the factor to generates dynamic weight list.
	// If specified, StaticWeightList will be ignored.
	// +kubebuilder:validation:Enum=AvailableReplicas
	// +optional
	DynamicWeight DynamicWeightFactor `json:"dynamicWeight,omitempty"`
}

// StaticClusterWeight defines the static cluster weight.
type StaticClusterWeight struct {
	// TargetCluster describes the filter to select clusters.
	// +required
	TargetCluster ClusterAffinity `json:"targetCluster"`

	// Weight expressing the preference to the cluster(s) specified by 'TargetCluster'.
	// +kubebuilder:validation:Minimum=1
	// +required
	Weight int64 `json:"weight"`
}

// DynamicWeightFactor represents the weight factor.
// For now only support 'AvailableReplicas', more factors could be extended if there is a need.
type DynamicWeightFactor string

const (
	// DynamicWeightByAvailableReplicas represents the cluster weight list should be generated according to
	// available resource (available replicas).
	// Example:
	//   The scheduler selected 3 clusters (A/B/C) and should divide 12 replicas to them.
	//   Workload:
	//     Desired replica: 12
	//   Cluster:
	//     A: Max available replica: 6
	//     B: Max available replica: 12
	//     C: Max available replica: 18
	//   The weight of cluster A:B:C will be 6:12:18 (equals to 1:2:3). At last, the assignment would be 'A: 2, B: 4, C: 6'.
	DynamicWeightByAvailableReplicas DynamicWeightFactor = "AvailableReplicas"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PropagationPolicyList contains a list of PropagationPolicy.
type PropagationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PropagationPolicy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster",shortName=cpp,categories={karmada-io}
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterPropagationPolicy represents the cluster-wide policy that propagates a group of resources to one or more clusters.
// Different with PropagationPolicy that could only propagate resources in its own namespace, ClusterPropagationPolicy
// is able to propagate cluster level resources and resources in any namespace other than system reserved ones.
// System reserved namespaces are: karmada-system, karmada-cluster, karmada-es-*.
type ClusterPropagationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterPropagationPolicy.
	// +required
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
