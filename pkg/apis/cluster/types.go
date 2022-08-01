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

	// Spec represents the specification of the desired behavior of member cluster.
	Spec ClusterSpec

	// Status represents the status of member cluster.
	// +optional
	Status ClusterStatus
}

// ClusterSpec defines the desired state of a member cluster.
type ClusterSpec struct {
	// ID is the unique identifier for the cluster.
	// It is different from the object uid(.metadata.uid) and typically collected automatically
	// from member cluster during the progress of registration.
	//
	// The value is collected in order:
	// 1. If the registering cluster enabled ClusterProperty API and defined the cluster ID by
	//   creating a ClusterProperty object with name 'cluster.clusterset.k8s.io', Karmada would
	//   take the defined value in the ClusterProperty object.
	//   See https://github.com/kubernetes-sigs/about-api for more details about ClusterProperty API.
	// 2. Take the uid of 'kube-system' namespace on the registering cluster.
	//
	// Please don't update this value unless you know what you are doing, because
	// it will/may be used to :
	// - uniquely identify the clusters within the Karmada system.
	// - compose the DNS name of multi-cluster services.
	//
	// +optional
	// +kubebuilder:validation:Maxlength=128000
	ID string `json:"id,omitempty"`

	// SyncMode describes how a cluster sync resources from karmada control plane.
	// +required
	SyncMode ClusterSyncMode

	// The API endpoint of the member cluster. This can be a hostname,
	// hostname:port, IP or IP:port.
	// +optional
	APIEndpoint string

	// SecretRef represents the secret contains mandatory credentials to access the member cluster.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// - secret.data.caBundle
	// +optional
	SecretRef *LocalSecretReference

	// ImpersonatorSecretRef represents the secret contains the token of impersonator.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// +optional
	ImpersonatorSecretRef *LocalSecretReference

	// InsecureSkipTLSVerification indicates that the karmada control plane should not confirm the validity of the serving
	// certificate of the cluster it is connecting to. This will make the HTTPS connection between the karmada control
	// plane and the member cluster insecure.
	// Defaults to false.
	// +optional
	InsecureSkipTLSVerification bool

	// ProxyURL is the proxy URL for the cluster.
	// If not empty, the karmada control plane will use this proxy to talk to the cluster.
	// More details please refer to: https://github.com/kubernetes/client-go/issues/351
	// +optional
	ProxyURL string

	// ProxyHeader is the HTTP header required by proxy server.
	// The key in the key-value pair is HTTP header key and value is the associated header payloads.
	// For the header with multiple values, the values should be separated by comma(e.g. 'k1': 'v1,v2,v3').
	// +optional
	ProxyHeader map[string]string

	// Provider represents the cloud provider name of the member cluster.
	// +optional
	Provider string

	// Region represents the region of the member cluster locate in.
	// +optional
	Region string

	// Zone represents the zone of the member cluster locate in.
	// +optional
	Zone string

	// Taints attached to the member cluster.
	// Taints on the cluster have the "effect" on
	// any resource that does not tolerate the Taint.
	// +optional
	Taints []corev1.Taint
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
	// Namespace is the namespace for the resource being referenced.
	Namespace string

	// Name is the name of resource being referenced.
	Name string
}

// Define valid conditions of a member cluster.
const (
	// ClusterConditionReady means the cluster is healthy and ready to accept workloads.
	ClusterConditionReady = "Ready"
)

// ClusterStatus contains information about the current status of a
// cluster updated periodically by cluster controller.
type ClusterStatus struct {
	// KubernetesVersion represents version of the member cluster.
	// +optional
	KubernetesVersion string

	// APIEnablements represents the list of APIs installed in the member cluster.
	// +optional
	APIEnablements []APIEnablement

	// Conditions is an array of current cluster conditions.
	// +optional
	Conditions []metav1.Condition

	// NodeSummary represents the summary of nodes status in the member cluster.
	// +optional
	NodeSummary *NodeSummary

	// ResourceSummary represents the summary of resources in the member cluster.
	// +optional
	ResourceSummary *ResourceSummary
}

// APIEnablement is a list of API resource, it is used to expose the name of the
// resources supported in a specific group and version.
type APIEnablement struct {
	// GroupVersion is the group and version this APIEnablement is for.
	GroupVersion string

	// Resources is a list of APIResource.
	// +optional
	Resources []APIResource
}

// APIResource specifies the name and kind names for the resource.
type APIResource struct {
	// Name is the plural name of the resource.
	// +required
	Name string

	// Kind is the kind for the resource (e.g. 'Deployment' is the kind for resource 'deployments')
	// +required
	Kind string
}

// NodeSummary represents the summary of nodes status in a specific cluster.
type NodeSummary struct {
	// TotalNum is the total number of nodes in the cluster.
	// +optional
	TotalNum int32

	// ReadyNum is the number of ready nodes in the cluster.
	// +optional
	ReadyNum int32
}

// ResourceSummary represents the summary of resources in the member cluster.
type ResourceSummary struct {
	// Allocatable represents the resources of a cluster that are available for scheduling.
	// Total amount of allocatable resources on all nodes.
	// +optional
	Allocatable corev1.ResourceList

	// Allocating represents the resources of a cluster that are pending for scheduling.
	// Total amount of required resources of all Pods that are waiting for scheduling.
	// +optional
	Allocating corev1.ResourceList

	// Allocated represents the resources of a cluster that have been scheduled.
	// Total amount of required resources of all Pods that have been scheduled to nodes.
	// +optional
	Allocated corev1.ResourceList
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList contains a list of member cluster
type ClusterList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items holds a list of Cluster.
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
