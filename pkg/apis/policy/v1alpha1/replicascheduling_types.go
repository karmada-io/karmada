package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=rsp
// +kubebuilder:deprecatedversion

// ReplicaSchedulingPolicy represents the policy that propagates total number of replicas for deployment.
type ReplicaSchedulingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ReplicaSchedulingPolicy.
	Spec ReplicaSchedulingSpec `json:"spec"`
}

// ReplicaSchedulingSpec represents the desired behavior of ReplicaSchedulingPolicy.
type ReplicaSchedulingSpec struct {
	// ResourceSelectors used to select resources.
	// +required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// TotalReplicas represents the total number of replicas across member clusters.
	// The replicas(spec.replicas) specified for deployment template will be discarded.
	// +required
	TotalReplicas int32 `json:"totalReplicas"`

	// Preferences describes weight for each cluster or for each group of cluster.
	// +required
	Preferences ClusterPreferences `json:"preferences"`
}

// ClusterPreferences describes weight for each cluster or for each group of cluster.
type ClusterPreferences struct {
	// StaticWeightList defines the static cluster weight.
	// +required
	StaticWeightList []StaticClusterWeight `json:"staticWeightList"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReplicaSchedulingPolicyList contains a list of ReplicaSchedulingPolicy.
type ReplicaSchedulingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicaSchedulingPolicy `json:"items"`
}
