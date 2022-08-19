package cluster

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceName is the name identifying various resources in a ResourceList.
type ResourceName string

// Resource names must be not more than 63 characters, consisting of upper- or lower-case alphanumeric characters,
// with the -, _, and . characters allowed anywhere, except the first or last character.
// The default convention, matching that for annotations, is to use lower-case names, with dashes, rather than
// camel case, separating compound words.
// Fully-qualified resource typenames are constructed from a DNS-style subdomain, followed by a slash `/` and a name.
const (
	// ResourceCPU in cores. (e,g. 500m = .5 cores)
	ResourceCPU ResourceName = "cpu"
	// ResourceMemory in bytes. (e,g. 500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	ResourceMemory ResourceName = "memory"
	// ResourceStorage is volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024)
	ResourceStorage ResourceName = "storage"
	// ResourceEphemeralStorage is local ephemeral storage, in bytes. (e,g. 500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// The resource name for ResourceEphemeralStorage is alpha and it can change across releases.
	ResourceEphemeralStorage ResourceName = "ephemeral-storage"
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

	// ResourceModels is the list of resource modeling in this cluster. Each modeling quota can be customized by the user.
	// Modeling name must be one of the following: cpu, memory, storage, ephemeral-storage.
	// If the user does not define the modeling name and modeling quota, it will be the default model.
	// The default model grade from 0 to 8.
	// When grade = 0 or grade = 1, the default model's cpu quota and memory quota is a fix value.
	// When grade greater than or equal to 2, each default model's cpu quota is [2^(grade-1), 2^grade), 2 <= grade <= 7
	// Each default model's memory quota is [2^(grade + 2), 2^(grade + 3)), 2 <= grade <= 7
	// E.g. grade 0 likes this:
	// - grade: 0
	//   ranges:
	//   - name: "cpu"
	//     min: 0 C
	//     max: 1 C
	//   - name: "memory"
	//     min: 0 GB
	//     max: 4 GB
	//
	// - grade: 1
	//   ranges:
	//   - name: "cpu"
	//     min: 1 C
	//     max: 2 C
	//   - name: "memory"
	//     min: 4 GB
	//     max: 16 GB
	//
	// - grade: 2
	//   ranges:
	//   - name: "cpu"
	//     min: 2 C
	//     max: 4 C
	//   - name: "memory"
	//     min: 16 GB
	//     max: 32 GB
	//
	// - grade: 7
	//   range:
	//   - name: "cpu"
	//     min: 64 C
	//     max: 128 C
	//   - name: "memory"
	//     min: 512 GB
	//     max: 1024 GB
	//
	// grade 8, the last one likes below. No matter what Max value you pass,
	// the meaning of Max value in this grade is infinite. You can pass any number greater than Min value.
	// - grade: 8
	//   range:
	//   - name: "cpu"
	//     min: 128 C
	//     max: MAXINT
	//   - name: "memory"
	//     min: 1024 GB
	//     max: MAXINT
	//
	// +optional
	ResourceModels []ResourceModel
}

// ResourceModel describes the modeling that you want to statistics.
type ResourceModel struct {
	// Grade is the index for the resource modeling.
	// +required
	Grade uint

	// Ranges describes the resource quota ranges.
	// +required
	Ranges []ResourceModelRange
}

// ResourceModelRange describes the detail of each modeling quota that ranges from min to max.
// Please pay attention, by default, the value of min can be inclusive, and the value of max cannot be inclusive.
// E.g. in an interval, min = 2, max =10 is set, which means the interval [2,10).
// This rule ensure that all intervals have the same meaning. If the last interval is +∞,
// it is definitely unreachable. Therefore, we define the right interval as the open interval.
// For a valid interval, the value on the right is greater than the value on the left,
// in other words, max must be greater than min.
// It is strongly recommended that the [Min, Max) of all ResourceModelRanges can make a continuous interval.
type ResourceModelRange struct {
	// Name is the name for the resource that you want to categorize.
	// +required
	Name ResourceName

	// Min is the minimum amount of this resource represented by resource name.
	// Note: The Min value of first grade(usually 0) always acts as zero.
	// E.g. [1,2) equal to [0,2).
	// +required
	Min resource.Quantity

	// Max is the maximum amount of this resource represented by resource name.
	// Special Instructions, for the last ResourceModelRange, which no matter what Max value you pass,
	// the meaning is infinite. Because for the last item,
	// any ResourceModelRange's quota larger than Min will be classified to the last one.
	// Of course, the value of the Max field is always greater than the value of the Min field.
	// It should be true in any case.
	// +required
	Max resource.Quantity
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

	// AllocatableModelings represents the statistical resource modeling.
	// +optional
	AllocatableModelings []AllocatableModeling
}

// AllocatableModeling represents the number of nodes in which allocatable resources in a specific resource model grade.
// E.g. AllocatableModeling{Grade: 2, Count: 10} means 10 nodes belong to resource model in grade 2.
type AllocatableModeling struct {
	// Grade is the index of ResourceModel.
	// +required
	Grade uint

	// Count is the number of nodes that own the resources delineated by this modeling.
	// +required
	Count int
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
