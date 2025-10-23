/*
Copyright 2022 The Karmada Authors.

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

package spreadconstraint

import (
	"fmt"

	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SelectBestClusters selects the cluster set based the GroupClustersInfo and placement
func SelectBestClusters(placement *policyv1alpha1.Placement, groupClustersInfo *GroupClustersInfo, needReplicas int32) ([]ClusterDetailInfo, error) {
	if len(placement.SpreadConstraints) == 0 || shouldIgnoreSpreadConstraint(placement) {
		klog.V(4).Infof("Select all clusters")
		return groupClustersInfo.Clusters, nil
	}

	if shouldIgnoreAvailableResource(placement) {
		needReplicas = InvalidReplicas
	}

	return selectBestClustersBySpreadConstraints(placement.SpreadConstraints, groupClustersInfo, needReplicas)
}

func selectBestClustersBySpreadConstraints(spreadConstraints []policyv1alpha1.SpreadConstraint,
	groupClustersInfo *GroupClustersInfo, needReplicas int32) ([]ClusterDetailInfo, error) {
	spreadConstraintMap := make(map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint)
	for i := range spreadConstraints {
		spreadConstraintMap[spreadConstraints[i].SpreadByField] = spreadConstraints[i]
	}

	if _, exist := spreadConstraintMap[policyv1alpha1.SpreadByFieldRegion]; exist {
		return selectBestClustersByRegion(spreadConstraintMap, groupClustersInfo)
	} else if _, exist := spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster]; exist {
		return selectBestClustersByCluster(spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster], groupClustersInfo, needReplicas)
	}

	return nil, fmt.Errorf("just support cluster and region spread constraint")
}

func shouldIgnoreSpreadConstraint(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'static weighted', ignore the declaration specified by spread constraints.
	return isStaticWeightedStrategy(strategy)
}

func shouldIgnoreAvailableResource(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'Duplicated', ignore the information about cluster available resource.
	if strategy == nil || strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return true
	}

	return false
}

// when replica assignment strategy is "Duplicated" or "static weighted", no need to calculate the available resource
func shouldIgnoreCalculateAvailableResource(placement *policyv1alpha1.Placement) bool {
	strategyType := placement.ReplicaSchedulingType()
	if strategyType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return true
	}

	return isStaticWeightedStrategy(placement.ReplicaScheduling)
}

// isStaticWeightedStrategy checks if the strategy is static weighted.
// A strategy is considered static weighted if the scheduling type is 'Divided',
// the division preference is 'Weighted', and either no weight preference is
// specified (implying equal weights) or static weights are defined without
// a dynamic weight factor.
func isStaticWeightedStrategy(strategy *policyv1alpha1.ReplicaSchedulingStrategy) bool {
	return strategy != nil && strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		strategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(strategy.WeightPreference == nil || len(strategy.WeightPreference.StaticWeightList) != 0 && strategy.WeightPreference.DynamicWeight == "")
}
