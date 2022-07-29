package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rb,categories={karmada-io}

// ResourceBinding represents a binding of a kubernetes resource with a propagation policy.
type ResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior.
	Spec ResourceBindingSpec `json:"spec"`

	// Status represents the most recently observed status of the ResourceBinding.
	// +optional
	Status ResourceBindingStatus `json:"status,omitempty"`
}

// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
	// Resource represents the Kubernetes resource to be propagated.
	Resource ObjectReference `json:"resource"`
	// Clusters represents target member clusters where the resource to be deployed.
	// +optional
	Clusters []TargetCluster `json:"clusters,omitempty"`
}

// ObjectReference contains enough information to locate the referenced object inside current cluster.
type ObjectReference struct {
	// APIVersion represents the API version of the referent.
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the referent.
	Kind string `json:"kind"`

	// Namespace represents the namespace for the referent.
	// For non-namespace scoped resources(e.g. 'ClusterRole')，do not need specify Namespace,
	// and for namespace scoped resources, Namespace is required.
	// If Namespace is not specified, means the resource is non-namespace scoped.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name represents the name of the referent.
	Name string `json:"name"`

	// ResourceVersion represents the internal version of the referenced object, that can be used by clients to
	// determine when object has changed.
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// ReplicaResourceRequirements represents the resources required by each replica.
	// +optional
	ReplicaResourceRequirements corev1.ResourceList `json:"resourcePerReplicas,omitempty"`

	// Replicas represents the replica number of the referencing resource.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// TargetCluster represents the identifier of a member cluster.
type TargetCluster struct {
	// Name of target cluster.
	Name string `json:"name"`
	// Replicas in target cluster
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
	// Conditions contain the different condition statuses.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// AggregatedStatus represents status list of the resource running in each member cluster.
	// +optional
	AggregatedStatus []AggregatedStatusItem `json:"aggregatedStatus,omitempty"`
}

// AggregatedStatusItem represents status of the resource running in a member cluster.
type AggregatedStatusItem struct {
	// ClusterName represents the member cluster name which the resource deployed on.
	// +required
	ClusterName string `json:"clusterName"`

	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Status *runtime.RawExtension `json:"status,omitempty"`
	// Applied represents if the resource referencing by ResourceBinding or ClusterResourceBinding
	// is successfully applied on the cluster.
	// +optional
	Applied bool `json:"applied,omitempty"`

	// AppliedMessage is a human readable message indicating details about the applied status.
	// This is usually holds the error message in case of apply failed.
	// +optional
	AppliedMessage string `json:"appliedMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceBindingList contains a list of ResourceBinding.
type ResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ResourceBinding.
	Items []ResourceBinding `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster",shortName=crb,categories={karmada-io}
// +kubebuilder:subresource:status

// ClusterResourceBinding represents a binding of a kubernetes resource with a ClusterPropagationPolicy.
type ClusterResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior.
	Spec ResourceBindingSpec `json:"spec"`

	// Status represents the most recently observed status of the ResourceBinding.
	// +optional
	Status ResourceBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterResourceBindingList contains a list of ClusterResourceBinding.
type ClusterResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ClusterResourceBinding.
	Items []ClusterResourceBinding `json:"items"`
}
