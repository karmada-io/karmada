package util

import (
	"github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsMemberClusterReady tells whether the cluster in ready state via checking its conditions.
func IsMemberClusterReady(cluster *v1alpha1.MemberCluster) bool {
	for _, condition := range cluster.Status.Conditions {
		// TODO(RainbowMango): Condition type should be defined in API, and after that update this hard code accordingly.
		if condition.Type == "ClusterReady" {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}
