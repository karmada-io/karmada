package pb

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// MaxAvailableReplicasRequest represents the request that sent by gRPC client to calculate max available replicas.
type MaxAvailableReplicasRequest struct {
	// Cluster represents the cluster name.
	// +required
	Cluster string `json:"cluster" protobuf:"bytes,1,opt,name=cluster"`
	// ReplicaRequirements represents the requirements required by each replica.
	// +required
	ReplicaRequirements ReplicaRequirements `json:"replicaRequirements" protobuf:"bytes,2,opt,name=replicaRequirements"`
}

// NodeClaim represents the NodeAffinity, NodeSelector and Tolerations required by each replica.
type NodeClaim struct {
	// A node selector represents the union of the results of one or more label queries over a set of
	// nodes; that is, it represents the OR of the selectors represented by the node selector terms.
	// Note that only PodSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	// is included here because it has a hard limit on pod scheduling.
	// +optional
	NodeAffinity *corev1.NodeSelector `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,2,rep,name=nodeSelector"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,3,rep,name=tolerations"`
}

// ReplicaRequirements represents the requirements required by each replica.
type ReplicaRequirements struct {
	// NodeClaim represents the NodeAffinity, NodeSelector and Tolerations required by each replica.
	// +optional
	NodeClaim *NodeClaim `json:"nodeClaim,omitempty" protobuf:"bytes,1,opt,name=nodeClaim"`
	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty" protobuf:"bytes,2,rep,name=resourceRequest,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`
}

// MaxAvailableReplicasResponse represents the response that sent by gRPC server to calculate max available replicas.
type MaxAvailableReplicasResponse struct {
	// MaxReplicas represents the max replica that the cluster can produce.
	// +required
	MaxReplicas int32 `json:"maxReplicas" protobuf:"varint,1,opt,name=maxReplicas"`
}

// UnschedulableReplicasRequest represents the request that sent by gRPC client to calculate unschedulable replicas.
type UnschedulableReplicasRequest struct {
	// Cluster represents the cluster name.
	// +required
	Cluster string `json:"cluster" protobuf:"bytes,1,opt,name=cluster"`
	// Resource represents the Kubernetes resource to be propagated.
	// +required
	Resource ObjectReference `json:"resource" protobuf:"bytes,2,opt,name=resource"`
	// UnschedulableThreshold represents the period threshold of pod unscheduable condition.
	// This value is considered as a classification standard of unscheduable replicas.
	// +optional
	UnschedulableThreshold time.Duration `json:"unschedulableThreshold,omitempty" protobuf:"varint,3,opt,name=unschedulableThreshold,casttype=time.Duration"`
}

// ObjectReference contains enough information to locate the referenced object inside current cluster.
type ObjectReference struct {
	// APIVersion represents the API version of the referent.
	// +required
	APIVersion string `json:"apiVersion" protobuf:"bytes,1,opt,name=apiVersion"`

	// Kind represents the Kind of the referent.
	// +required
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`

	// Namespace represents the namespace for the referent.
	// For non-namespace scoped resources(e.g. 'ClusterRole')，do not need specify Namespace,
	// and for namespace scoped resources, Namespace is required.
	// If Namespace is not specified, means the resource is non-namespace scoped.
	// +required
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`

	// Name represents the name of the referent.
	// +required
	Name string `json:"name" protobuf:"bytes,4,opt,name=name"`
}

// UnschedulableReplicasResponse represents the response that sent by gRPC server to calculate unschedulable replicas.
type UnschedulableReplicasResponse struct {
	// UnschedulableReplicas represents the unschedulable replicas that the object contains.
	// +required
	UnschedulableReplicas int32 `json:"maxUnschedulableReplicas" protobuf:"varint,1,opt,name=maxUnschedulableReplicas"`
}
