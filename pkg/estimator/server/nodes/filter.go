package nodes

import (
	corev1 "k8s.io/api/core/v1"
	schedcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

var (
	unschedulableTaint = corev1.Taint{
		Key:    corev1.TaintNodeUnschedulable,
		Effect: corev1.TaintEffectNoSchedule,
	}
)

// GetRequiredNodeAffinity returns the parsing result of pod's nodeSelector and nodeAffinity.
func GetRequiredNodeAffinity(requirements pb.ReplicaRequirements) nodeaffinity.RequiredNodeAffinity {
	if requirements.NodeClaim == nil {
		return nodeaffinity.RequiredNodeAffinity{}
	}
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeSelector: requirements.NodeClaim.NodeSelector,
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: requirements.NodeClaim.NodeAffinity,
				},
			},
		},
	}
	return nodeaffinity.GetRequiredNodeAffinity(pod)
}

// IsNodeAffinityMatched returns whether the node matches the node affinity.
func IsNodeAffinityMatched(node *corev1.Node, affinity nodeaffinity.RequiredNodeAffinity) bool {
	// Ignore parsing errors for backwards compatibility.
	match, _ := affinity.Match(node)
	return match
}

// IsTolerationMatched returns whether the node matches the tolerations.
func IsTolerationMatched(node *corev1.Node, tolerations []corev1.Toleration) bool {
	// If pod tolerate unschedulable taint, it's also tolerate `node.Spec.Unschedulable`.
	podToleratesUnschedulable := schedcorev1.TolerationsTolerateTaint(tolerations, &unschedulableTaint)
	if node.Spec.Unschedulable && !podToleratesUnschedulable {
		return false
	}
	if _, isUntolerated := schedcorev1.FindMatchingUntoleratedTaint(node.Spec.Taints, tolerations, DoNotScheduleTaintsFilterFunc()); isUntolerated {
		return false
	}
	return true
}

// DoNotScheduleTaintsFilterFunc returns the filter function that can
// filter out the node taints that reject scheduling Pod on a Node.
func DoNotScheduleTaintsFilterFunc() func(t *corev1.Taint) bool {
	return func(t *corev1.Taint) bool {
		// PodToleratesNodeTaints is only interested in NoSchedule and NoExecute taints.
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}
}
