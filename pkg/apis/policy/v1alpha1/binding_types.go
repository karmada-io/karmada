package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

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
}

// TargetCluster represents the identifier of a member cluster.
type TargetCluster struct {
	// Name of target cluster.
	Name string `json:"name"`
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
	ClusterName string `json:"clusterName"`
	// ResourceStatus represents the status of the resource.
	// +optional
	ResourceStatus ResourceStatus `json:"resourceStatus,omitempty"`
}

// ResourceStatus represents status of all supported resource type.
// Each field owns a specific `Kind`, and only one field will be available(non-nil).
type ResourceStatus struct {
	// Deployment represents the deployment status in the member cluster,
	// only available when the resource kind is Deployment.
	// +optional
	Deployment *DeploymentStatus `json:"deploymentStatus,omitempty"`
	// DaemonSet
	// Job
	// Secret...
}

// DeploymentStatus represents a 'Deployment' status.
type DeploymentStatus struct {
	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Total number of ready pods targeted by this deployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Total number of unavailable pods targeted by this deployment. This is the total number of
	// pods that are still required for the deployment to have 100% available capacity. They may
	// either be pods that are running but not yet available or pods that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceBindingList contains a list of ResourceBinding.
type ResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ResourceBinding.
	Items []ResourceBinding `json:"items"`
}
