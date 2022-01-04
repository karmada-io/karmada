package cluster

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//revive:disable:exported

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster represents the desire state and status of a member cluster.
type Cluster struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ClusterSpec
	Status ClusterStatus
}

// ClusterSpec defines the desired state of a member cluster.
type ClusterSpec struct {
	SyncMode ClusterSyncMode

	APIEndpoint string

	SecretRef *LocalSecretReference

	// ImpersonatorSecretRef represents the secret contains the token of impersonator.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// +optional
	ImpersonatorSecretRef *LocalSecretReference

	InsecureSkipTLSVerification bool
	ProxyURL                    string
	Provider                    string
	Region                      string
	Zone                        string
	Taints                      []corev1.Taint
}

const (
	// SecretTokenKey is the name of secret token key.
	SecretTokenKey = "token"
	// SecretCADataKey is the name of secret caBundle key.
	SecretCADataKey = "caBundle"
)

// ClusterSyncMode describes the mode of synchronization between member cluster and karmada control plane.
type ClusterSyncMode string

const (
	// Push means that the controller on the karmada control plane will in charge of synchronization.
	// The controller watches resources change on karmada control plane then pushes them to member cluster.
	Push ClusterSyncMode = "Push"

	// Pull means that the controller running on the member cluster will in charge of synchronization.
	// The controller, as well known as 'agent', watches resources change on karmada control plane then fetches them
	// and applies locally on the member cluster.
	Pull ClusterSyncMode = "Pull"
)

// LocalSecretReference is a reference to a secret within the enclosing
// namespace.
type LocalSecretReference struct {
	Namespace string
	Name      string
}

// Define valid conditions of a member cluster.
const (
	// ClusterConditionReady means the cluster is healthy and ready to accept workloads.
	ClusterConditionReady = "Ready"
)

// ClusterStatus contains information about the current status of a
// cluster updated periodically by cluster controller.
type ClusterStatus struct {
	KubernetesVersion string
	APIEnablements    []APIEnablement
	Conditions        []metav1.Condition
	NodeSummary       *NodeSummary
	ResourceSummary   *ResourceSummary
}

// APIEnablement is a list of API resource, it is used to expose the name of the
// resources supported in a specific group and version.
type APIEnablement struct {
	GroupVersion string
	Resources    []APIResource
}

// APIResource specifies the name and kind names for the resource.
type APIResource struct {
	Name string
	Kind string
}

// NodeSummary represents the summary of nodes status in a specific cluster.
type NodeSummary struct {
	TotalNum int32
	ReadyNum int32
}

// ResourceSummary represents the summary of resources in the member cluster.
type ResourceSummary struct {
	Allocatable corev1.ResourceList
	Allocating  corev1.ResourceList
	Allocated   corev1.ResourceList
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList contains a list of member cluster
type ClusterList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []Cluster
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterProxyOptions is the query options to a Cluster's proxy call.
type ClusterProxyOptions struct {
	metav1.TypeMeta

	// Path is the URL path to use for the current proxy request
	Path string
}

//revive:enable:exported
