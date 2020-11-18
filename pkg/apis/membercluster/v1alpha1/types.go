package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MemberCluster represents the desire state and status of a member cluster.
type MemberCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the specification of the desired behavior of member cluster.
	Spec MemberClusterSpec `json:"spec"`

	// Status represents the status of member cluster.
	// +optional
	Status MemberClusterStatus `json:"status,omitempty"`
}

// MemberClusterSpec defines the desired state of a member cluster.
type MemberClusterSpec struct {
	// ManageMode specifies the relationship between control plane and member cluster,
	// the mode determines how to reach each other.
	// +optional
	ManageMode ClusterManageMode `json:"manageMode,omitempty"`

	// Accepted represents if the member cluster has been accepted by control plane.
	// Default value is false.
	// If member cluster working in 'Delegation' mode, this always be true.
	// If member cluster working in 'SelfManagement' mode, this will turn to true only after administrator
	// accepted the request from member cluster.
	// +optional
	Accepted bool `json:"accepted,omitempty"`

	// The API endpoint of the member cluster. This can be a hostname,
	// hostname:port, IP or IP:port.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// SecretRef represents the secret contains mandatory credentials to access the member cluster.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// - secret.data.caBundle
	// +optional
	SecretRef *LocalSecretReference `json:"secretRef,omitempty"`

	// Provider represents the cloud provider name of the member cluster.
	// +optional
	Provider string `json:"provider,omitempty"`

	// Region represents the region of the member cluster locate in.
	// +optional
	Region string `json:"region,omitempty"`

	// Zone represents the zone of the member cluster locate in.
	// +optional
	Zone string `json:"zone,omitempty"`

	// Taints attached to the member cluster.
	// Taints on the cluster have the "effect" on
	// any resource that does not tolerate the Taint.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`
}

// ClusterManageMode presents member clusters working mode.
type ClusterManageMode string

const (
	// Delegation represents a member cluster will be managed directly by control plane.
	Delegation ClusterManageMode = "Delegation"
	// SelfManagement represents a member cluster will be managed by itself.
	SelfManagement ClusterManageMode = "SelfManagement"
)

// LocalSecretReference is a reference to a secret within the enclosing
// namespace.
type LocalSecretReference struct {
	// Namespace is the namespace for the resource being referenced.
	Namespace string `json:"namespace"`

	// Name is the name of resource being referenced.
	Name string `json:"name"`
}

// MemberClusterStatus contains information about the current status of a
// cluster updated periodically by cluster controller.
type MemberClusterStatus struct {
	// KubernetesVersion represents version of the member cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// APIEnablements represents the list of APIs installed in the member cluster.
	// +optional
	APIEnablements []APIEnablement `json:"apiEnablements,omitempty"`

	// Conditions is an array of current cluster conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// NodeSummary represents the summary of nodes status in the member cluster.
	NodeSummary NodeSummary `json:"nodeSummary,omitempty"`
}

// APIEnablement is a list of API resource, it is used to expose the name of the
// resources supported in a specific group and version.
type APIEnablement struct {
	// GroupVersion is the group and version this APIEnablement is for.
	GroupVersion string `json:"groupVersion"`
	// Resources contains the name of the resources.
	Resources []string `json:"resources"`
}

// NodeSummary represents the summary of nodes status in a specific cluster.
type NodeSummary struct {
	// TotalNum is the total number of nodes in the cluster.
	TotalNum int `json:"totalNum,omitempty"`
	// ReadyNum is the number of ready nodes in the cluster.
	ReadyNum int `json:"readyNum,omitempty"`
	// Allocatable represents the allocatable resources across all nodes.
	Allocatable corev1.ResourceList `json:"allocatable,omitempty"`
	// Used represents the resources have been used across all nodes.
	Used corev1.ResourceList `json:"used,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MemberClusterList contains a list of member cluster
type MemberClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemberCluster `json:"items"`
}
