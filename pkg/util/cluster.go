package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
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

// GetMemberCluster returns the given MemberCluster resource
func GetMemberCluster(hostClient client.Client, clusterName string) (*v1alpha1.MemberCluster, error) {
	memberCluster := &v1alpha1.MemberCluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, memberCluster); err != nil {
		return nil, err
	}
	return memberCluster, nil
}
