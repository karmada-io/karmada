package replica

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/karmada-io/karmada/pkg/estimator/server/nodes"
)

// NodeMaxAvailableReplica calculates max available replicas of a node, based on
// the pods assigned to the node and the request resource of the replica.
func NodeMaxAvailableReplica(node *corev1.Node, pods []*corev1.Pod, request corev1.ResourceList) (int32, error) {
	ni := nodes.NewNodeInfo(node)
	if err := ni.AssignedPodRequest(pods); err != nil {
		return 0, err
	}
	return int32(ni.MaxReplicaDivided(request)), nil
}
