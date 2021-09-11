package nodes

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/karmada-io/karmada/pkg/util"
)

// NodeInfo is the wrapper of a node and its resource.
type NodeInfo struct {
	Node *corev1.Node

	AllocatableResource *util.Resource
	IdleResource        *util.Resource
}

// NewNodeInfo returns a instance of NodeInfo. The initial IdleResource equals with AllocatableResource.
func NewNodeInfo(node *corev1.Node) *NodeInfo {
	allocatableResource := util.NewResource(node.Status.Allocatable)
	return &NodeInfo{
		Node:                node,
		AllocatableResource: allocatableResource,
		IdleResource:        allocatableResource,
	}
}

// AssignedPodRequest counts the effective request resource of pods for the node.
// IdleResource will be subtracted from the all pod request resources when this function is called.
func (ni *NodeInfo) AssignedPodRequest(pods []*corev1.Pod) error {
	occupiedResource := util.EmptyResource()
	for _, pod := range pods {
		occupiedResource.AddPodRequest(&pod.Spec)
	}
	// The pod request does not contain a pod resource, so we add it manually.
	occupiedResource.AddResourcePods(int64(len(pods)))

	// The occupied resource must be less than or equal with the node idle resource.
	if !occupiedResource.LessEqual(ni.IdleResource) {
		return fmt.Errorf("node %s does have enough idle resource to accommodate %d pods", ni.Node.Name, len(pods))
	}

	// subtract
	if err := ni.IdleResource.Sub(occupiedResource.ResourceList()); err != nil {
		return err
	}
	return nil
}

// MaxReplicaDivided returns how many replicas that the node can produce.
func (ni *NodeInfo) MaxReplicaDivided(rl corev1.ResourceList) int64 {
	return ni.IdleResource.MaxDivided(rl)
}
