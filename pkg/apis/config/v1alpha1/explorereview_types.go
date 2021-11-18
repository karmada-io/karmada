package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExploreReview describes an explore review request and response.
type ExploreReview struct {
	metav1.TypeMeta `json:",inline"`

	// Request describes the attributes for the explore request.
	// +optional
	Request *ExploreRequest `json:"request,omitempty"`

	// Response describes the attributes for the explore response.
	// +optional
	Response *ExploreResponse `json:"response,omitempty"`
}

// ExploreRequest describes the explore.Attributes for the explore request.
type ExploreRequest struct {
	// UID is an identifier for the individual request/response.
	// The UID is meant to track the round trip (request/response) between the karmada and the WebHook, not the user request.
	// It is suitable for correlating log entries between the webhook and karmada, for either auditing or debugging.
	// +required
	UID types.UID `json:"uid"`

	// Kind is the fully-qualified type of object being submitted (for example, v1.Pod or autoscaling.v1.Scale)
	// +required
	Kind metav1.GroupVersionKind `json:"kind"`

	// Name is the name of the object as presented in the request.
	// +required
	Name string `json:"name"`

	// Namespace is the namespace associated with the request (if any).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Operation is the operation being performed.
	// +required
	Operation OperationType `json:"operation"`

	// Object is the object from the incoming request.
	// +optional
	Object runtime.RawExtension `json:"object,omitempty"`

	// ObservedObject is the object observed from the kube-apiserver of member clusters.
	// Not nil only when OperationType is ExploreRetaining.
	// +optional
	ObservedObject *runtime.RawExtension `json:"observedObject,omitempty"`

	// DesiredReplicas represents the desired pods number which webhook should revise with.
	// It'll be set only if OperationType is ExploreReplicaRevising.
	// +optional
	DesiredReplicas *int32 `json:"replicas,omitempty"`

	// AggregatedStatus represents status list of the resource running in each member cluster.
	// +optional
	AggregatedStatus []workv1alpha1.AggregatedStatusItem `json:"aggregatedStatus,omitempty"`
}

// ExploreResponse describes an explore response.
type ExploreResponse struct {
	// UID is an identifier for the individual request/response.
	// This must be copied over from the corresponding ExploreRequest.
	// +required
	UID types.UID `json:"uid"`

	// Successful indicates whether the request be processed successfully.
	// +required
	Successful bool `json:"successful"`

	// Status contains extra details information about why the request not successful.
	// This filed is not consulted in any way if "Successful" is "true".
	// +optional
	Status *RequestStatus `json:"status,omitempty"`

	// The patch body. We only support "JSONPatch" currently which implements RFC 6902.
	// +optional
	Patch []byte `json:"patch,omitempty"`

	// The type of Patch. We only allow "JSONPatch" currently.
	// +optional
	PatchType *PatchType `json:"patchType,omitempty" protobuf:"bytes,5,opt,name=patchType"`

	// ReplicaRequirements represents the requirements required by each replica.
	// Required if OperationType is ExploreReplica.
	// +optional
	ReplicaRequirements *workv1alpha2.ReplicaRequirements `json:"replicaRequirements,omitempty"`

	// Replicas represents the number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified.
	// Required if OperationType is ExploreReplica.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Dependencies represents the reference of dependencies object.
	// Required if OperationType is ExploreDependencies.
	// +optional
	Dependencies []DependentObjectReference `json:"dependencies,omitempty"`

	// RawStatus represents the referencing object's status.
	// +optional
	RawStatus *runtime.RawExtension `json:"rawStatus,omitempty"`

	// Healthy represents the referencing object's healthy status.
	// +optional
	Healthy *bool `json:"healthy,omitempty"`
}

// RequestStatus holds the status of a request.
type RequestStatus struct {
	// Message is human-readable description of the status of this operation.
	// +optional
	Message string `json:"message,omitempty"`

	// Code is the HTTP return code of this status.
	// +optional
	Code int32 `json:"code,omitempty"`
}

// PatchType is the type of patch being used to represent the mutated object
type PatchType string

const (
	// PatchTypeJSONPatch represents the JSONType.
	PatchTypeJSONPatch PatchType = "JSONPatch"
)

// DependentObjectReference contains enough information to locate the referenced object inside current cluster.
type DependentObjectReference struct {
	// APIVersion represents the API version of the referent.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the referent.
	// +required
	Kind string `json:"kind"`

	// Namespace represents the namespace for the referent.
	// For non-namespace scoped resources(e.g. 'ClusterRole')，do not need specify Namespace,
	// and for namespace scoped resources, Namespace is required.
	// If Namespace is not specified, means the resource is non-namespace scoped.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name represents the name of the referent.
	// +required
	Name string `json:"name"`
}
