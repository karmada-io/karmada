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

package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var (
	assignFuncMap = map[string]func(*assignState) ([]workv1alpha2.TargetCluster, error){
		DuplicatedStrategy:    assignByDuplicatedStrategy,
		AggregatedStrategy:    assignByDynamicStrategy,
		StaticWeightStrategy:  assignByStaticWeightStrategy,
		DynamicWeightStrategy: assignByDynamicStrategy,
	}
)

const (
	// DuplicatedStrategy indicates each candidate member cluster will directly apply the original replicas.
	DuplicatedStrategy = "Duplicated"
	// AggregatedStrategy indicates dividing replicas among clusters as few as possible and
	// taking clusters' available replicas into consideration as well.
	AggregatedStrategy = "Aggregated"
	// StaticWeightStrategy indicates dividing replicas by static weight according to WeightPreference.
	StaticWeightStrategy = "StaticWeight"
	// DynamicWeightStrategy indicates dividing replicas by dynamic weight according to WeightPreference.
	DynamicWeightStrategy = "DynamicWeight"
)

// assignmentMode indicates how to assign replicas, especially in case of re-assignment.
type assignmentMode string

const (
	// Steady represents a steady, incremental approach to re-assign replicas
	// across clusters. It aims to maintain the exist replicas distribution as
	// closely as possible, only making minimal adjustments when necessary.
	// It minimizes disruptions and preserves the balance across clusters.
	Steady assignmentMode = "Steady"

	// Fresh means that disregards the previous assignment entirely and
	// seeks to establish an entirely new replica distribution across clusters.
	// It is willing to accept significant changes even if it involves disruption.
	Fresh assignmentMode = "Fresh"
)

// assignState is a wrapper of the input for assigning function.
type assignState struct {
	candidates []*clusterv1alpha1.Cluster
	strategy   *policyv1alpha1.ReplicaSchedulingStrategy
	spec       *workv1alpha2.ResourceBindingSpec

	// fields below are indirect results
	strategyType string

	// assignmentMode represents the mode how to assign replicas
	assignmentMode assignmentMode

	scheduledClusters []workv1alpha2.TargetCluster
	assignedReplicas  int32
	availableClusters []workv1alpha2.TargetCluster
	availableReplicas int32

	// targetReplicas is the replicas that we need to schedule in this round
	targetReplicas int32
}

func newAssignState(candidates []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec,
	status *workv1alpha2.ResourceBindingStatus) *assignState {
	var strategyType string

	switch spec.Placement.ReplicaSchedulingType() {
	case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
		strategyType = DuplicatedStrategy
	case policyv1alpha1.ReplicaSchedulingTypeDivided:
		switch spec.Placement.ReplicaScheduling.ReplicaDivisionPreference {
		case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
			strategyType = AggregatedStrategy
		case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
			if spec.Placement.ReplicaScheduling.WeightPreference != nil && len(spec.Placement.ReplicaScheduling.WeightPreference.DynamicWeight) != 0 {
				strategyType = DynamicWeightStrategy
			} else {
				strategyType = StaticWeightStrategy
			}
		}
	}

	// the assignment mode is defaults to Steady to minimizes disruptions and preserves the balance across clusters.
	expectAssignmentMode := Steady
	// when spec.rescheduleTriggeredAt is updated, it represents a rescheduling is manually triggered by user, and the
	// expected behavior of this action is to do a complete recalculation without referring to last scheduling results.
	if util.RescheduleRequired(spec.RescheduleTriggeredAt, status.LastScheduledTime) {
		expectAssignmentMode = Fresh
	}

	return &assignState{candidates: candidates, strategy: spec.Placement.ReplicaScheduling, spec: spec, strategyType: strategyType, assignmentMode: expectAssignmentMode}
}

func (as *assignState) buildScheduledClusters() {
	candidateClusterSet := sets.Set[string]{}
	for _, c := range as.candidates {
		candidateClusterSet.Insert(c.Name)
	}
	as.scheduledClusters = []workv1alpha2.TargetCluster{}
	for _, c := range as.spec.Clusters {
		// Ignore clusters that are no longer candidates, to ensure we can get real
		// 'assigned' replicas from the previous schedule result. The ignored replicas
		// will be treated as scaled-up replicas that will be assigned to other
		// candidate clusters.
		if !candidateClusterSet.Has(c.Name) {
			continue
		}
		as.scheduledClusters = append(as.scheduledClusters, c)
	}
	as.assignedReplicas = util.GetSumOfReplicas(as.scheduledClusters)
}

func (as *assignState) buildAvailableClusters(c calculator) {
	as.availableClusters = c(as.candidates, as.spec)
	as.availableReplicas = util.GetSumOfReplicas(as.availableClusters)
}

// resortAvailableClusters is used to make sure scheduledClusters are at the front of availableClusters
// list so that we can assign new replicas to them preferentially when scale up.
func (as *assignState) resortAvailableClusters() []workv1alpha2.TargetCluster {
	// get the previous scheduled clusters
	prior := sets.NewString()
	for _, cluster := range as.scheduledClusters {
		if cluster.Replicas > 0 {
			prior.Insert(cluster.Name)
		}
	}

	if len(prior) == 0 {
		return as.availableClusters
	}

	var (
		prev = make([]workv1alpha2.TargetCluster, 0, len(prior))
		left = make([]workv1alpha2.TargetCluster, 0, len(as.availableClusters)-len(prior))
	)

	for _, cluster := range as.availableClusters {
		if prior.Has(cluster.Name) {
			prev = append(prev, cluster)
		} else {
			left = append(left, cluster)
		}
	}
	as.availableClusters = append(prev, left...)
	return as.availableClusters
}

// assignByDuplicatedStrategy assigns replicas by DuplicatedStrategy.
func assignByDuplicatedStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	targetClusters := make([]workv1alpha2.TargetCluster, len(state.candidates))
	for i, cluster := range state.candidates {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: state.spec.Replicas}
	}
	return targetClusters, nil
}

/*
* assignByStaticWeightStrategy assigns a total number of replicas to the selected clusters by the weight list.
* For example, we want to assign replicas to two clusters named A and B.
* | Total | Weight(A:B) | Assignment(A:B) |
* |   9   |   1:2       |     3:6         |
* |   9   |   1:3       |     2:7         | Approximate assignment
* Note:
* 1. If any selected cluster which not present on the weight list will be ignored(different with '0' replica).
* 2. In case of not enough replica for specific cluster which will get '0' replica.
 */
func assignByStaticWeightStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	// If ReplicaDivisionPreference is set to "Weighted" and WeightPreference is not set,
	// scheduler will weight all clusters averagely.
	if state.strategy.WeightPreference == nil {
		state.strategy.WeightPreference = getDefaultWeightPreference(state.candidates)
	}
	weightList := getStaticWeightInfoList(state.candidates, state.strategy.WeightPreference.StaticWeightList, state.spec.Clusters)

	disp := helper.NewDispenser(state.spec.Replicas, nil)
	disp.TakeByWeight(weightList)

	return disp.Result, nil
}

func assignByDynamicStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	state.buildScheduledClusters()

	// 1. when Fresh mode expected, do a complete recalculation without referring to the last scheduling results.
	if state.assignmentMode == Fresh {
		result, err := dynamicFreshScale(state)
		if err != nil {
			return nil, fmt.Errorf("failed to do fresh scale: %v", err)
		}
		return result, nil
	}

	// 2. when Steady mode expected, try minimizing large changes in scheduling results.
	if state.assignedReplicas > state.spec.Replicas {
		// We need to reduce the replicas in terms of the previous result.
		result, err := dynamicScaleDown(state)
		if err != nil {
			return nil, fmt.Errorf("failed to scale down: %v", err)
		}
		return result, nil
	} else if state.assignedReplicas < state.spec.Replicas {
		// We need to enlarge the replicas in terms of the previous result (if exists).
		// First scheduling is considered as a special kind of scaling up.
		result, err := dynamicScaleUp(state)
		if err != nil {
			return nil, fmt.Errorf("failed to scale up: %v", err)
		}
		return result, nil
	}

	return state.scheduledClusters, nil
}
