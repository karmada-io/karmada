/*
Copyright 2021 The Karmada Authors.

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

package nodes

import (
	corev1 "k8s.io/api/core/v1"
	schedcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"

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
	// Note: Kubernetes v1.35 extended the toleration operators by introducing Lt and Gt,
	// and extended the ToleratesTaint method to include logger and enableComparisonOperators parameters.
	// PR: https://github.com/kubernetes/kubernetes/pull/134665
	//
	// TODO(@RainbowMango): Karmada requires more detailed design to support these new operators.
	// For now, we disable the comparison operators (enableComparisonOperators=false) to maintain
	// backward-compatible behavior.
	// With this flag set to false, the logger parameter is not actually used.
	podToleratesUnschedulable := schedcorev1.TolerationsTolerateTaint(klog.Background(), tolerations, &unschedulableTaint, false)
	if node.Spec.Unschedulable && !podToleratesUnschedulable {
		return false
	}
	// Note: Kubernetes v1.35 extended the toleration operators by introducing Lt and Gt,
	// and extended the ToleratesTaint method to include logger and enableComparisonOperators parameters.
	// PR: https://github.com/kubernetes/kubernetes/pull/134665
	//
	// TODO(@RainbowMango): Karmada requires more detailed design to support these new operators.
	// For now, we disable the comparison operators (enableComparisonOperators=false) to maintain
	// backward-compatible behavior.
	// With this flag set to false, the logger parameter is not actually used.
	if _, isUntolerated := schedcorev1.FindMatchingUntoleratedTaint(klog.Background(), node.Spec.Taints, tolerations, DoNotScheduleTaintsFilterFunc(), false); isUntolerated {
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
