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
	corev1 "k8s.io/api/core/v1"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
	"github.com/karmada-io/karmada/pkg/util"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

// SchedulingSimulator simulates a scheduling process to estimate workload capacity.
// It uses the First Fit(FF) algorithm to pack components into available nodes efficiently.
// During the simulation, it will consume node resources as components are scheduled, so make
// sure the nodes passed in are cloned copies or not used elsewhere.
type SchedulingSimulator struct {
	// nodes represent the available cluster nodes with their resource capacity
	nodes []*schedulerframework.NodeInfo
}

// NewSchedulingSimulator creates a new scheduling simulator instance.
func NewSchedulingSimulator(nodes []*schedulerframework.NodeInfo) *SchedulingSimulator {
	return &SchedulingSimulator{
		nodes: nodes,
	}
}

// SimulateScheduling implements the First Fit(FF) algorithm to estimate
// the maximum number of complete component sets that can be scheduled on the cluster.
//
// FF Algorithm Steps:
// 1. For each complete set, try to schedule all components using first-fit strategy
// 2. Continue until no more complete sets can be scheduled or upper limit is reached
func (s *SchedulingSimulator) SimulateScheduling(components []pb.Component, upperBound int32) int32 {
	var completeSets int32
	// Try to schedule complete component sets until we can no longer do so or reach the upper limit.
	for {
		if completeSets < upperBound && s.scheduleComponentSet(components) {
			completeSets++
		} else {
			break
		}
	}

	return completeSets
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
	requiredPerReplica := util.NewResource(component.ReplicaRequirements.ResourceRequest)
	requiredPerReplica.AllowedPodNumber = 1
	remaining := component.Replicas
	nodeClaim := component.ReplicaRequirements.NodeClaim

	for _, node := range s.nodes {
		if !matchNode(nodeClaim, node) {
			continue
		}

		allocatable := node.Allocatable.MaxDivided(requiredPerReplica.ResourceList())
		if allocatable == 0 {
			continue
		}

		if int64(remaining) < allocatable {
			allocatable = int64(remaining)
		}

		node.Allocatable.SubResource(requiredPerReplica.Clone().Multiply(allocatable))
		remaining -= int32(allocatable) // #nosec G115: integer overflow conversion int64 -> int32
		if remaining == 0 {
			return true
		}
	}

	return remaining == 0
}

// matchNode checks whether the node matches the scheduling constraints defined in the replica requirements.
func matchNode(nodeClaim *pb.NodeClaim, node *schedulerframework.NodeInfo) bool {
	if node.Node() == nil {
		// Always match since we lack node affinity/toleration info, so we skip these checks.
		return true
	}

	affinity := nodeutil.GetRequiredNodeAffinity(pb.ReplicaRequirements{NodeClaim: nodeClaim})
	var tolerations []corev1.Toleration

	if nodeClaim != nil {
		tolerations = nodeClaim.Tolerations
	}

	return nodeutil.IsNodeAffinityMatched(node.Node(), affinity) && nodeutil.IsTolerationMatched(node.Node(), tolerations)
}
