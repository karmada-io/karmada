package helper

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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

// TolerationExists checks if the given toleration exists in list of tolerations. Returns true if exists false otherwise.
func TolerationExists(tolerations []corev1.Toleration, tolerationToFind *corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if toleration.MatchToleration(tolerationToFind) {
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

// AddTolerations add some tolerations if not existed.
func AddTolerations(placement *policyv1alpha1.Placement, tolerationsToAdd ...*corev1.Toleration) {
	for _, tolerationToAdd := range tolerationsToAdd {
		if !TolerationExists(placement.ClusterTolerations, tolerationToAdd) {
			placement.ClusterTolerations = append(placement.ClusterTolerations, *tolerationToAdd)
		}
	}
}

// HasNoExecuteTaints check if NoExecute taints exist.
func HasNoExecuteTaints(taints []corev1.Taint) bool {
	for i := range taints {
		if taints[i].Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}

// GetNoExecuteTaints will get all NoExecute taints.
func GetNoExecuteTaints(taints []corev1.Taint) []corev1.Taint {
	var result []corev1.Taint
	for i := range taints {
		if taints[i].Effect == corev1.TaintEffectNoExecute {
			result = append(result, taints[i])
		}
	}
	return result
}

// GetMinTolerationTime returns minimal toleration time from the given slice, or -1 if it's infinite.
func GetMinTolerationTime(noExecuteTaints []corev1.Taint, usedTolerations []corev1.Toleration) time.Duration {
	if len(noExecuteTaints) == 0 {
		return -1
	}
	if len(usedTolerations) == 0 {
		return 0
	}

	// Get all trigger time.
	var triggerTimes []time.Time
	for i := range usedTolerations {
		if usedTolerations[i].TolerationSeconds == nil {
			continue
		}
		tolerationSeconds := *(usedTolerations[i].TolerationSeconds)
		for j := range noExecuteTaints {
			// should not happen
			if noExecuteTaints[j].TimeAdded == nil {
				continue
			}
			timeAdded := *noExecuteTaints[j].TimeAdded
			if usedTolerations[i].ToleratesTaint(&noExecuteTaints[j]) {
				triggerTimes = append(triggerTimes, timeAdded.Add(time.Duration(tolerationSeconds)*time.Second))
			}
		}
	}

	// If triggerTimes is empty, it means all taints would be tolerated forever.
	if len(triggerTimes) == 0 {
		return -1
	}

	// Find the latest trigger time.
	t := triggerTimes[0]
	for i := 1; i < len(triggerTimes); i++ {
		if triggerTimes[i].Before(t) {
			t = triggerTimes[i]
		}
	}

	// If trigger time is up, don't tolerate.
	if t.Before(time.Now()) {
		return 0
	}

	return time.Until(t)
}

// GetMatchingTolerations returns true and list of Tolerations matching all Taints if all are tolerated, or false otherwise.
func GetMatchingTolerations(taints []corev1.Taint, tolerations []corev1.Toleration) (bool, []corev1.Toleration) {
	if len(taints) == 0 {
		return true, []corev1.Toleration{}
	}
	if len(tolerations) == 0 && len(taints) > 0 {
		return false, []corev1.Toleration{}
	}
	var result []corev1.Toleration
	for i := range taints {
		tolerated := false
		for j := range tolerations {
			if tolerations[j].ToleratesTaint(&taints[i]) {
				result = append(result, tolerations[j])
				tolerated = true
				break
			}
		}
		if !tolerated {
			return false, []corev1.Toleration{}
		}
	}
	return true, result
}

// NewNotReadyToleration returns a default not ready toleration.
func NewNotReadyToleration(tolerationSeconds int64) *corev1.Toleration {
	return &corev1.Toleration{
		Key:               clusterv1alpha1.TaintClusterNotReady,
		Operator:          corev1.TolerationOpExists,
		Effect:            corev1.TaintEffectNoExecute,
		TolerationSeconds: &tolerationSeconds,
	}
}

// NewUnreachableToleration returns a default unreachable toleration.
func NewUnreachableToleration(tolerationSeconds int64) *corev1.Toleration {
	return &corev1.Toleration{
		Key:               clusterv1alpha1.TaintClusterUnreachable,
		Operator:          corev1.TolerationOpExists,
		Effect:            corev1.TaintEffectNoExecute,
		TolerationSeconds: &tolerationSeconds,
	}
}
