/*
Copyright 2025 The Karmada Authors.

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

package estimator

import (
	"sort"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/util"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

// SchedulingSimulator simulates a scheduling process to estimate workload capacity.
// It uses the First Fit Decreasing (FFD) algorithm to pack components into available nodes efficiently.
type SchedulingSimulator struct {
	// nodes represent the available cluster nodes with their resource capacity
	nodes []*schedulerframework.NodeInfo
	// matchNode is a function to check if a component can be scheduled on a specific node
	// based on node affinity, tolerations, and other scheduling constraints
	matchNode func(nodeClaim *pb.NodeClaim, node *schedulerframework.NodeInfo) bool
}

// NewSchedulingSimulator creates a new scheduling simulator instance.
func NewSchedulingSimulator(nodes []*schedulerframework.NodeInfo, matchNode func(nodeClaim *pb.NodeClaim, node *schedulerframework.NodeInfo) bool) *SchedulingSimulator {
	return &SchedulingSimulator{
		nodes:     nodes,
		matchNode: matchNode,
	}
}

// SimulateSchedulingFFD implements the First Fit Decreasing (FFD) algorithm to estimate
// the maximum number of complete component sets that can be scheduled on the cluster.
//
// FFD Algorithm Steps:
// 1. Sort components by their "difficulty to schedule" (decreasing order of resource constraints)
// 2. For each complete set, try to schedule all components using first-fit strategy
// 3. Continue until no more complete sets can be scheduled or upper limit is reached
func (s *SchedulingSimulator) SimulateSchedulingFFD(components []pb.Component, upperSets int32) int32 {
	// Calculate total cluster resources for component sorting
	totalResource := util.EmptyResource()
	for _, node := range s.nodes {
		totalResource.Add(node.Allocatable.ResourceList())
	}

	componentsCopy := make([]pb.Component, len(components))
	copy(componentsCopy, components)
	// Sort components in decreasing order of scheduling difficulty
	// Components that are harder to place (fewer possible placements) are scheduled first
	sort.Slice(componentsCopy, func(i, j int) bool {
		iMaxSets := totalResource.MaxDivided(componentsCopy[i].ReplicaRequirements.ResourceRequest)
		jMaxSets := totalResource.MaxDivided(componentsCopy[j].ReplicaRequirements.ResourceRequest)
		if iMaxSets == jMaxSets {
			// Use tie-breaker when components have similar scheduling difficulty
			return tieBreaker(componentsCopy[i], componentsCopy[j])
		}
		// Schedule components with fewer possible placements first (harder to place)
		return iMaxSets < jMaxSets
	})

	var completeSets int32
	// Try to schedule complete component sets until we can no longer do so or reach the upper limit.
	for {
		if completeSets < upperSets && s.scheduleComponentSet(componentsCopy) {
			completeSets++
		} else {
			break
		}
	}

	return completeSets
}

// tieBreaker resolves ties when two components have the same scheduling difficulty.
// It compares components by their total resource consumption to prioritize larger components first.
func tieBreaker(componentA, componentB pb.Component) bool {
	resourceA := util.NewResource(componentA.ReplicaRequirements.ResourceRequest)
	resourceB := util.NewResource(componentB.ReplicaRequirements.ResourceRequest)

	// Calculate total resource score for a component
	getScore := func(res *util.Resource) int64 {
		score := res.MilliCPU + res.Memory + res.EphemeralStorage
		for _, v := range res.ScalarResources {
			score += v
		}
		return score
	}

	return getScore(resourceA) > getScore(resourceB)
}

// scheduleComponentSet attempts to schedule one complete set of all components.
func (s *SchedulingSimulator) scheduleComponentSet(components []pb.Component) bool {
	for _, component := range components {
		if !s.scheduleComponent(component) {
			return false
		}
	}

	return true
}

func (s *SchedulingSimulator) scheduleComponent(component pb.Component) bool {
	task := newSchedulingTask(component)
	for _, node := range s.nodes {
		if !s.matchNode(task.nodeClaim, node) {
			continue
		}

		for node.Allocatable.Allocatable(task.requiredResourcePerReplica) {
			// Assign one replica to this node.
			task.scheduleOnePod(node)
			if task.done() {
				// short path
				return true
			}
		}
	}

	return task.done()
}

// componentSchedulingTask represents a single component scheduling task with its requirements and state.
type componentSchedulingTask struct {
	// nodeClaim represents the NodeAffinity, NodeSelector and Tolerations required by this component.
	nodeClaim *pb.NodeClaim
	// requiredResourcePerReplica represents the resources required by a single replica of this component.
	requiredResourcePerReplica *util.Resource
	// toBeScheduled tracks how many replicas of this component still need to be scheduled
	toBeScheduled int32
}

// done returns true if the task has been completely scheduled (no replicas remaining).
// This indicates that a complete component has been successfully allocated.
func (t *componentSchedulingTask) done() bool {
	return t.toBeScheduled == 0
}

// scheduleOnePod schedules one replica of this component on the specified node.
// It decrements the remaining replica count and subtracts the required resources from the node.
// This should be called when a replica has been successfully scheduled on a node.
func (t *componentSchedulingTask) scheduleOnePod(node *schedulerframework.NodeInfo) {
	if t.toBeScheduled <= 0 {
		// No more replicas to schedule
		return
	}

	node.Allocatable.SubResource(t.requiredResourcePerReplica)
	t.toBeScheduled--
}

// newSchedulingTask creates a new component scheduling task from the given component.
// It initializes the task with the component's node claim, required resources per replica, and total replicas to be scheduled.
func newSchedulingTask(component pb.Component) componentSchedulingTask {
	needResource := util.NewResource(component.ReplicaRequirements.ResourceRequest)
	needResource.AllowedPodNumber = 1
	return componentSchedulingTask{
		nodeClaim:                  component.ReplicaRequirements.NodeClaim,
		requiredResourcePerReplica: needResource,
		toBeScheduled:              component.Replicas,
	}
}
