package spreadconstraint

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// selectBestClustersByClusterLabels support for SpreadByLabel constraints.
func selectBestClustersByClusterLabels(spreadConstraint []policyv1alpha1.SpreadConstraint, groupClustersInfo *GroupClustersInfo,
	needReplicas int32) ([]*clusterv1alpha1.Cluster, error) {
	var clusterInfos = groupClustersInfo.Clusters
	for i := range spreadConstraint {
		if spreadConstraint[i].SpreadByLabel == "" {
			continue
		}
		// firstly, match cluster labels
		requirement, err := labels.NewRequirement(spreadConstraint[i].SpreadByLabel, selection.Exists, []string{})
		if err != nil {
			return nil, err
		}
		selector := labels.NewSelector().Add(*requirement)
		var selectedClusters []ClusterDetailInfo
		for j := range clusterInfos {
			if selector.Matches(labels.Set(clusterInfos[j].Cluster.GetLabels())) {
				selectedClusters = append(selectedClusters, clusterInfos[j])
			}
		}

		// secondly, select the clusters with the highest score in the selected clusters
		totalClusterCnt := len(selectedClusters)
		if totalClusterCnt < spreadConstraint[i].MinGroups {
			return nil, fmt.Errorf("the number of feasible clusters is less than spreadConstraint[%d].MinGroups", i)
		}

		needCnt := spreadConstraint[i].MaxGroups
		if totalClusterCnt < spreadConstraint[i].MaxGroups {
			needCnt = totalClusterCnt
		}

		if needReplicas == InvalidReplicas {
			selectedClusters = selectedClusters[:needCnt]
		} else {
			selectedClusters = selectClustersByAvailableResource(selectedClusters, int32(needCnt), needReplicas)
		}
		if len(selectedClusters) == 0 {
			return nil, fmt.Errorf("no enough resource when selecting %d clusters", needCnt)
		}

		// thirdly, save the selected clusters to entry the next spread constraints.
		clusterInfos = selectedClusters
	}

	var clusters []*clusterv1alpha1.Cluster
	for i := range clusterInfos {
		clusters = append(clusters, clusterInfos[i].Cluster)
	}
	return clusters, nil
}
