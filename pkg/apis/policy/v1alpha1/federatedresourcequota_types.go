package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindFederatedResourceQuota is kind name of FederatedResourceQuota.
	ResourceKindFederatedResourceQuota = "FederatedResourceQuota"
	// ResourceSingularFederatedResourceQuota is singular name of FederatedResourceQuota.
	ResourceSingularFederatedResourceQuota = "federatedresourcequota"
	// ResourcePluralFederatedResourceQuota is plural name of FederatedResourceQuota.
	ResourcePluralFederatedResourceQuota = "federatedresourcequotas"
	// ResourceNamespaceScopedFederatedResourceQuota indicates if FederatedResourceQuota is NamespaceScoped.
	ResourceNamespaceScopedFederatedResourceQuota = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories={karmada-io}
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// FederatedResourceQuota sets aggregate quota restrictions enforced per namespace across all clusters.
type FederatedResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired quota.
	// +required
	Spec FederatedResourceQuotaSpec `json:"spec"`

	// Status defines the actual enforced quota and its current usage.
	// +optional
	Status FederatedResourceQuotaStatus `json:"status,omitempty"`
}

// FederatedResourceQuotaSpec defines the desired hard limits to enforce for Quota.
type FederatedResourceQuotaSpec struct {
	// Overall is the set of desired hard limits for each named resource.
	// +required
	Overall corev1.ResourceList `json:"overall"`

	// StaticAssignments represents the subset of desired hard limits for each cluster.
	// Note: for clusters not present in this list, Karmada will set an empty ResourceQuota to them, which means these
	// clusters will have no quotas in the referencing namespace.
	// +optional
	StaticAssignments []StaticClusterAssignment `json:"staticAssignments,omitempty"`

	// DynamicAssignments represents the rule about how to assign and ensure the quota for each cluster.
	// E.g.
	//   - The rule about how to assign the total amount of quotas(specified by Overall) to Clusters.
	//   - The rule about how to balance the quota between clusters in a dynamic way.
	//   - The rule about how to prevent workload from scheduling to cluster without quota.
	//   - The rule about how to prevent workload from creating to Karmada control plane.
	//   - The rule about how to cooperate with FederatedHPA to scale workloads.
	//
	// Note: We are in the progress of gathering user cases, if you are interested in
	//  this feature/capacity, please feel free to let us know.
	//
	// +optional
	// DynamicAssignments []DynamicClusterAssignment `json:"dynamicAssignments,omitempty"`
}

// StaticClusterAssignment represents the set of desired hard limits for a specific cluster.
type StaticClusterAssignment struct {
	// ClusterName is the name of the cluster the limits enforce to.
	// +required
	ClusterName string `json:"clusterName"`

	// Hard is the set of desired hard limits for each named resource.
	// +required
	Hard corev1.ResourceList `json:"hard"`
}

// FederatedResourceQuotaStatus defines the enforced hard limits and observed use.
type FederatedResourceQuotaStatus struct {
	// Overall is the set of enforced hard limits for each named resource.
	// +optional
	Overall corev1.ResourceList `json:"overall,omitempty"`

	// OverallUsed is the current observed total usage of the resource in the namespace.
	// +optional
	OverallUsed corev1.ResourceList `json:"overallUsed,omitempty"`

	// AggregatedStatus is the observed quota usage of each cluster.
	// +optional
	AggregatedStatus []ClusterQuotaStatus `json:"aggregatedStatus,omitempty"`
}

// ClusterQuotaStatus represents the set of desired limits and observed usage for a specific cluster.
type ClusterQuotaStatus struct {
	// ClusterName is the name of the cluster the limits enforce to.
	// +required
	ClusterName string `json:"clusterName"`

	corev1.ResourceQuotaStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederatedResourceQuotaList contains a list of FederatedResourceQuota.
type FederatedResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederatedResourceQuota `json:"items"`
}
