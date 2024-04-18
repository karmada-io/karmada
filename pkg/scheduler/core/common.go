/*
Copyright 2021 The Karmada Authors.

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

package core

import (
	"fmt"
	"time"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
)

// SelectClusters selects clusters based on the placement and resource binding spec.
func SelectClusters(clustersScore framework.ClusterScoreList,
	placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec) ([]*clusterv1alpha1.Cluster, error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepSelect, startTime)

	groupClustersInfo := spreadconstraint.GroupClustersWithScore(clustersScore, placement, spec, calAvailableReplicas)
	return spreadconstraint.SelectBestClusters(placement, groupClustersInfo, spec.Replicas)
}

// AssignReplicas assigns replicas to clusters based on the placement and resource binding spec.
func AssignReplicas(
	clusters []*clusterv1alpha1.Cluster,
	spec *workv1alpha2.ResourceBindingSpec,
	status *workv1alpha2.ResourceBindingStatus,
) ([]workv1alpha2.TargetCluster, error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepAssignReplicas, startTime)

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters available to schedule")
	}

	if spec.Replicas > 0 {
		state := newAssignState(clusters, spec, status)
		assignFunc, ok := assignFuncMap[state.strategyType]
		if !ok {
			// should never happen at present
			return nil, fmt.Errorf("unsupported replica scheduling strategy, replicaSchedulingType: %s, replicaDivisionPreference: %s, "+
				"please try another scheduling strategy", spec.Placement.ReplicaSchedulingType(), spec.Placement.ReplicaScheduling.ReplicaDivisionPreference)
		}
		assignResults, err := assignFunc(state)
		if err != nil {
			return nil, err
		}
		return removeZeroReplicasCluster(assignResults), nil
	}

	// If not workload, assign all clusters without considering replicas.
	targetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name}
	}
	return targetClusters, nil
}
