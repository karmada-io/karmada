/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	// Namespace represents the namespaces belonged to a ResourceRequest
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	// PriorityClassName represents the priority class name for a given ResourceRequest.
	// It is used by the resource quota estimator to check quota constraints, as ResourceQuota supports priority class as a scope.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty" protobuf:"bytes,4,opt,name=priorityClassName"`
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
	// UnschedulableThreshold represents the period threshold of pod unschedulable condition.
	// This value is considered as a classification standard of unschedulable replicas.
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
	UnschedulableReplicas int32 `json:"unschedulableReplicas" protobuf:"varint,1,opt,name=unschedulableReplicas"`
}

// MaxAvailableComponentSetsRequest is the gRPC request message used to estimate
// how many complete sets of components can be scheduled on a cluster.
type MaxAvailableComponentSetsRequest struct {
	// Cluster is the target cluster where the scheduling estimation is performed.
	// +required
	Cluster string `json:"cluster" protobuf:"bytes,1,opt,name=cluster"`

	// Components lists the component types that form one full workload set,
	// along with their resource and replica requirements.
	// +required
	Components []Component `json:"components" protobuf:"bytes,2,rep,name=components"`

	// Namespace is the namespace of the workload being estimated.
	// It is used by the accurate estimator to check the quota configurations
	// in the target member cluster.
	// +required
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`
}

// Component defines the scheduling and resource requirements for a single
// workload component (e.g., JobManager, TaskManager).
type Component struct {
	// Name is the identifier of the component within the workload set.
	// +required
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// ReplicaRequirements specifies the per-replica resource requirements
	// (CPU, memory, etc.) and scheduling constraints.
	// +required
	ReplicaRequirements ComponentReplicaRequirements `json:"replicaRequirements" protobuf:"bytes,2,opt,name=replicaRequirements"`

	// Replicas is the number of replicas of this component required in a single workload set.
	// +required
	Replicas int32 `json:"replicas" protobuf:"varint,3,opt,name=replicas"`
}

// ComponentReplicaRequirements represents the resource and scheduling requirements for each replica.
type ComponentReplicaRequirements struct {
	// NodeClaim represents the NodeAffinity, NodeSelector and Tolerations required by each replica.
	// +optional
	NodeClaim *NodeClaim `json:"nodeClaim,omitempty" protobuf:"bytes,1,opt,name=nodeClaim"`

	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty" protobuf:"bytes,2,rep,name=resourceRequest,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`

	// PriorityClassName represents the priority class name for a given ResourceRequest.
	// It is used by the resource quota estimator to check quota constraints, as ResourceQuota supports priority class as a scope.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty" protobuf:"bytes,3,opt,name=priorityClassName"`
}

// MaxAvailableComponentSetsResponse is the gRPC response message containing the
// maximum number of complete component sets that can be scheduled.
type MaxAvailableComponentSetsResponse struct {
	// MaxSets is the maximum number of workload sets (i.e., all required components together)
	// that the target cluster can accommodate given the resource constraints.
	// +required
	MaxSets int32 `json:"maxSets" protobuf:"varint,1,opt,name=maxSets"`
}
