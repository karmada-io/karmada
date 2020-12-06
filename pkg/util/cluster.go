package util

import (
	"github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsMemberClusterReady tells whether the cluster status in 'Ready' condition.
func IsMemberClusterReady(clusterStatus *v1alpha1.MemberClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		// TODO(RainbowMango): Condition type should be defined in API, and after that update this hard code accordingly.
		if condition.Type == v1alpha1.ClusterConditionReady {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}
