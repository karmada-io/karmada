package nodes

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listv1 "k8s.io/client-go/listers/core/v1"
	schedcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

// ListNodesByNodeClaim returns all nodes that match the node claim.
func ListNodesByNodeClaim(nodeLister listv1.NodeLister, claim *pb.NodeClaim) ([]*corev1.Node, error) {
	nodeClaim := claim
	if nodeClaim == nil {
		nodeClaim = &pb.NodeClaim{}
	}
	nodes, err := ListNodesByLabelSelector(nodeLister, labels.SelectorFromSet(nodeClaim.NodeSelector))
	if err != nil {
		return nil, fmt.Errorf("cannot list nodes by label selector, %v", err)
	}
	nodes, err = FilterNodesByNodeAffinity(nodes, nodeClaim.NodeAffinity)
	if err != nil {
		return nil, fmt.Errorf("cannot filter nodes by node affinity, %v", err)
	}
	nodes, err = FilterSchedulableNodes(nodes, nodeClaim.Tolerations)
	if err != nil {
		return nil, fmt.Errorf("cannot filter nodes by tolerations, %v", err)
	}
	return nodes, err
}

// ListAllNodes returns all nodes.
func ListAllNodes(nodeLister listv1.NodeLister) ([]*corev1.Node, error) {
	return ListNodesByLabelSelector(nodeLister, labels.Everything())
}

// ListNodesByLabelSelector returns nodes that match the node selector.
func ListNodesByLabelSelector(nodeLister listv1.NodeLister, selector labels.Selector) ([]*corev1.Node, error) {
	nodes, err := nodeLister.List(selector)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// FilterNodesByNodeAffinity returns nodes that match the node affinity.
func FilterNodesByNodeAffinity(nodes []*corev1.Node, affinity *corev1.NodeSelector) ([]*corev1.Node, error) {
	if affinity == nil {
		return nodes, nil
	}
	matchedNodes := make([]*corev1.Node, 0)
	selector, err := nodeaffinity.NewNodeSelector(affinity)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if selector.Match(node) {
			matchedNodes = append(matchedNodes, node)
		}
	}
	return matchedNodes, nil
}

// FilterSchedulableNodes filters schedulable nodes that match the given tolerations.
func FilterSchedulableNodes(nodes []*corev1.Node, tolerations []corev1.Toleration) ([]*corev1.Node, error) {
	filterPredicate := func(t *corev1.Taint) bool {
		// PodToleratesNodeTaints is only interested in NoSchedule and NoExecute taints.
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}
	matchedNodes := make([]*corev1.Node, 0)
	for _, node := range nodes {
		if node.Spec.Unschedulable {
			continue
		}
		if !IsNodeReady(node.Status.Conditions) {
			continue
		}
		if _, isUntolerated := schedcorev1.FindMatchingUntoleratedTaint(node.Spec.Taints, tolerations, filterPredicate); isUntolerated {
			continue
		}
		matchedNodes = append(matchedNodes, node)
	}
	return matchedNodes, nil
}

// IsNodeReady checks whether the node condition is ready.
func IsNodeReady(nodeStatus []corev1.NodeCondition) bool {
	for i := range nodeStatus {
		if nodeStatus[i].Type == corev1.NodeReady {
			return nodeStatus[i].Status == corev1.ConditionTrue
		}
	}
	return false
}
