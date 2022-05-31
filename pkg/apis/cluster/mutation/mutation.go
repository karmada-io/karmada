package mutation

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

// MutateCluster mutates required fields of the Cluster.
func MutateCluster(cluster *clusterapis.Cluster) {
	MutateClusterTaints(cluster.Spec.Taints)
}

// MutateClusterTaints add TimeAdded field for cluster NoExecute taints only if TimeAdded not set.
func MutateClusterTaints(taints []corev1.Taint) {
	for i := range taints {
		if taints[i].Effect == corev1.TaintEffectNoExecute && taints[i].TimeAdded == nil {
			now := metav1.Now()
			taints[i].TimeAdded = &now
		}
	}
}
