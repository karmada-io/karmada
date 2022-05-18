package helper

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// TaintExists checks if the given taint exists in list of taints. Returns true if exists false otherwise.
func TaintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

// UpdateClusterControllerTaint add and remove some taints.
func UpdateClusterControllerTaint(ctx context.Context, client client.Client, taintsToAdd, taintsToRemove []*corev1.Taint, cluster *clusterv1alpha1.Cluster) error {
	var clusterTaintsToAdd, clusterTaintsToRemove []corev1.Taint
	// Find which taints need to be added.
	for _, taintToAdd := range taintsToAdd {
		if !TaintExists(cluster.Spec.Taints, taintToAdd) {
			clusterTaintsToAdd = append(clusterTaintsToAdd, *taintToAdd)
		}
	}
	// Find which taints need to be removed.
	for _, taintToRemove := range taintsToRemove {
		if TaintExists(cluster.Spec.Taints, taintToRemove) {
			clusterTaintsToRemove = append(clusterTaintsToRemove, *taintToRemove)
		}
	}
	// If no taints need to be added and removed, just return.
	if len(clusterTaintsToAdd) == 0 && len(clusterTaintsToRemove) == 0 {
		return nil
	}

	taints := make([]corev1.Taint, 0, len(cluster.Spec.Taints)+len(clusterTaintsToAdd)-len(clusterTaintsToRemove))
	// Remove taints which need to be removed.
	for i := range cluster.Spec.Taints {
		if !TaintExists(clusterTaintsToRemove, &cluster.Spec.Taints[i]) {
			taints = append(taints, cluster.Spec.Taints[i])
		}
	}

	// Add taints.
	for _, taintToAdd := range clusterTaintsToAdd {
		now := metav1.Now()
		taintToAdd.TimeAdded = &now
		taints = append(taints, taintToAdd)
	}

	cluster = cluster.DeepCopy()
	cluster.Spec.Taints = taints

	return client.Update(ctx, cluster)
}
