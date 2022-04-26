package spreadconstraint

import (
	"fmt"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SelectBestClusters selects the cluster set based the GroupClustersInfo and placement
func SelectBestClusters(placement *policyv1alpha1.Placement, groupClustersInfo *GroupClustersInfo, needReplicas int32) ([]*clusterv1alpha1.Cluster, error) {
	if len(placement.SpreadConstraints) == 0 || shouldIgnoreSpreadConstraint(placement) {
		var clusters []*clusterv1alpha1.Cluster
		for _, cluster := range groupClustersInfo.Clusters {
			clusters = append(clusters, cluster.Cluster)
		}
		klog.V(4).Infof("select all clusters")
		return clusters, nil
	}

	if shouldIgnoreAvailableResource(placement) {
		needReplicas = InvalidReplicas
	}

	return selectBestClustersBySpreadConstraints(placement.SpreadConstraints, groupClustersInfo, needReplicas)
}

func selectBestClustersBySpreadConstraints(spreadConstraints []policyv1alpha1.SpreadConstraint,
	groupClustersInfo *GroupClustersInfo, needReplicas int32) ([]*clusterv1alpha1.Cluster, error) {
	if len(spreadConstraints) > 1 {
		return nil, fmt.Errorf("just support single spread constraint")
	}

	spreadConstraint := spreadConstraints[0]
	if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster {
		return selectBestClustersByCluster(spreadConstraint, groupClustersInfo, needReplicas)
	}

	return nil, fmt.Errorf("just support cluster spread constraint")
}

func shouldIgnoreSpreadConstraint(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'static weighted', ignore the declaration specified by spread constraints.
	if strategy != nil && strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		strategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(strategy.WeightPreference != nil && len(strategy.WeightPreference.StaticWeightList) != 0 && strategy.WeightPreference.DynamicWeight == "") {
		return true
	}

	return false
}

func shouldIgnoreAvailableResource(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'Duplicated', ignore the information about cluster available resource.
	if strategy == nil || strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return true
	}

	return false
}
