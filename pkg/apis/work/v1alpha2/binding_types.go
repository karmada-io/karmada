package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// ResourceKindResourceBinding is kind name of ResourceBinding.
	ResourceKindResourceBinding = "ResourceBinding"
	// ResourceSingularResourceBinding is singular name of ResourceBinding.
	ResourceSingularResourceBinding = "resourcebinding"
	// ResourcePluralResourceBinding is plural name of ResourceBinding.
	ResourcePluralResourceBinding = "resourcebindings"
	// ResourceNamespaceScopedResourceBinding indicates if ResourceBinding is NamespaceScoped.
	ResourceNamespaceScopedResourceBinding = true

	// ResourceKindClusterResourceBinding is kind name of ClusterResourceBinding.
	ResourceKindClusterResourceBinding = "ClusterResourceBinding"
	// ResourceSingularClusterResourceBinding is singular name of ClusterResourceBinding.
	ResourceSingularClusterResourceBinding = "clusterresourcebinding"
	// ResourcePluralClusterResourceBinding is kind plural of ClusterResourceBinding.
	ResourcePluralClusterResourceBinding = "clusterresourcebindings"
	// ResourceNamespaceScopedClusterResourceBinding indicates if ClusterResourceBinding is NamespaceScoped.
	ResourceNamespaceScopedClusterResourceBinding = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rb,categories={karmada-io}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Scheduled")].status`,name="Scheduled",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="FullyApplied")].status`,name="FullyApplied",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

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

	// PropagateDeps tells if relevant resources should be propagated automatically.
	// It is inherited from PropagationPolicy or ClusterPropagationPolicy.
	// default false.
	// +optional
	PropagateDeps bool `json:"propagateDeps,omitempty"`

	// ReplicaRequirements represents the requirements required by each replica.
	// +optional
	ReplicaRequirements *ReplicaRequirements `json:"replicaRequirements,omitempty"`

	// Replicas represents the replica number of the referencing resource.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Clusters represents target member clusters where the resource to be deployed.
	// +optional
	Clusters []TargetCluster `json:"clusters,omitempty"`

	// RequiredBy represents the list of Bindings that depend on the referencing resource.
	// +optional
	RequiredBy []BindingSnapshot `json:"requiredBy,omitempty"`
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

	// UID of the referent.
	// +optional
	UID types.UID `json:"uid,omitempty"`

	// ResourceVersion represents the internal version of the referenced object, that can be used by clients to
	// determine when object has changed.
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// ReplicaRequirements represents the requirements required by each replica.
type ReplicaRequirements struct {
	// NodeClaim represents the node claim HardNodeAffinity, NodeSelector and Tolerations required by each replica.
	// +optional
	NodeClaim *NodeClaim `json:"nodeClaim,omitempty"`

	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty"`
}

// NodeClaim represents the node claim HardNodeAffinity, NodeSelector and Tolerations required by each replica.
type NodeClaim struct {
	// A node selector represents the union of the results of one or more label queries over a set of
	// nodes; that is, it represents the OR of the selectors represented by the node selector terms.
	// Note that only PodSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	// is included here because it has a hard limit on pod scheduling.
	// +optional
	HardNodeAffinity *corev1.NodeSelector `json:"hardNodeAffinity,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// TargetCluster represents the identifier of a member cluster.
type TargetCluster struct {
	// Name of target cluster.
	Name string `json:"name"`
	// Replicas in target cluster
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// BindingSnapshot is a snapshot of a ResourceBinding or ClusterResourceBinding.
type BindingSnapshot struct {
	// Namespace represents the namespace of the Binding.
	// It is required for ResourceBinding.
	// If Namespace is not specified, means the referencing is ClusterResourceBinding.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name represents the name of the Binding.
	// +required
	Name string `json:"name"`

	// Clusters represents the scheduled result.
	// +optional
	Clusters []TargetCluster `json:"clusters,omitempty"`
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

// Conditions definition
const (
	// Scheduled represents the condition that the ResourceBinding or ClusterResourceBinding has been scheduled.
	Scheduled string = "Scheduled"

	// FullyApplied represents the condition that the resource referencing by ResourceBinding or ClusterResourceBinding
	// has been applied to all scheduled clusters.
	FullyApplied string = "FullyApplied"
)

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
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Scheduled")].status`,name="Scheduled",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="FullyApplied")].status`,name="FullyApplied",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

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
