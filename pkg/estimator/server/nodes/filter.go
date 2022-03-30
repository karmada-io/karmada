package nodes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listv1 "k8s.io/client-go/listers/core/v1"
	schedcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var (
	tolerationFilterPredicate = func(t *corev1.Taint) bool {
		// PodToleratesNodeTaints is only interested in NoSchedule and NoExecute taints.
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}
)

// NodeClaimWrapper is a wrapper that wraps the node claim.
type NodeClaimWrapper struct {
	nodeSelector         labels.Selector
	tolerations          []corev1.Toleration
	nodeAffinitySelector *nodeaffinity.NodeSelector
}

// NewNodeClaimWrapper returns a new NodeClaimWrapper.
func NewNodeClaimWrapper(claim *pb.NodeClaim) (*NodeClaimWrapper, error) {
	wrapper := &NodeClaimWrapper{}
	if claim == nil {
		wrapper.nodeSelector = labels.Everything()
		return wrapper, nil
	}
	if claim.NodeAffinity != nil {
		selector, err := nodeaffinity.NewNodeSelector(claim.NodeAffinity)
		if err != nil {
			return nil, err
		}
		wrapper.nodeAffinitySelector = selector
	}
	wrapper.nodeSelector = labels.SelectorFromSet(claim.NodeSelector)
	wrapper.tolerations = claim.Tolerations
	return wrapper, nil
}

// ListNodesByLabelSelector returns nodes that match the node selector.
func (w *NodeClaimWrapper) ListNodesByLabelSelector(nodeLister listv1.NodeLister) ([]*corev1.Node, error) {
	nodes, err := nodeLister.List(w.nodeSelector)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// IsNodeMatched returns whether the node matches all conditions.
func (w *NodeClaimWrapper) IsNodeMatched(node *corev1.Node) bool {
	return w.IsNodeAffinityMatched(node) && w.IsNodeSchedulable(node)
}

// IsNodeAffinityMatched returns whether the node matches the node affinity.
func (w *NodeClaimWrapper) IsNodeAffinityMatched(node *corev1.Node) bool {
	if w.nodeAffinitySelector == nil {
		return true
	}
	return w.nodeAffinitySelector.Match(node)
}

// IsNodeSchedulable returns whether the node matches the tolerations.
func (w *NodeClaimWrapper) IsNodeSchedulable(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		return false
	}
	if !helper.NodeReady(node) {
		return false
	}
	if _, isUntolerated := schedcorev1.FindMatchingUntoleratedTaint(node.Spec.Taints, w.tolerations, tolerationFilterPredicate); isUntolerated {
		return false
	}
	return true
}
