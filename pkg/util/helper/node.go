package helper

import corev1 "k8s.io/api/core/v1"

// NodeReady checks whether the node condition is ready.
func NodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
