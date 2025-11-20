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

package noderesource

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
	"github.com/karmada-io/karmada/pkg/util"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
)

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "NodeResourceEstimator"
	// nodeResourceEstimator is enabled by default.
	enabled = true

	// noNodeConstraint represents the value when there is no node resource constraint.
	noNodeConstraint = math.MaxInt32
)

// nodeResourceEstimator is to estimate how many replicas/sets allowed by the node resources for a given pb.ReplicaRequirements.
type nodeResourceEstimator struct {
	enabled      bool
	parallelizer parallelize.Parallelizer
}

var _ framework.EstimateReplicasPlugin = &nodeResourceEstimator{}
var _ framework.EstimateComponentsPlugin = &nodeResourceEstimator{}

// New initializes a new plugin and returns it.
func New(fh framework.Handle) (framework.Plugin, error) {
	return &nodeResourceEstimator{
		enabled:      enabled,
		parallelizer: parallelize.NewParallelizer(fh.Parallelism()),
	}, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *nodeResourceEstimator) Name() string {
	return Name
}

// Estimate the replica allowed by the node resources for a given pb.ReplicaRequirements.
func (pl *nodeResourceEstimator) Estimate(ctx context.Context, snapshot *schedcache.Snapshot, requirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return noNodeConstraint, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}

	allNodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return 0, framework.AsResult(err)
	}
	var (
		affinity    = nodeutil.GetRequiredNodeAffinity(*requirements)
		tolerations []corev1.Toleration
	)

	if requirements.NodeClaim != nil {
		tolerations = requirements.NodeClaim.Tolerations
	}

	var res int32
	processNode := func(i int) {
		node := allNodes[i]
		if !nodeutil.IsNodeAffinityMatched(node.Node(), affinity) || !nodeutil.IsTolerationMatched(node.Node(), tolerations) {
			return
		}
		maxReplica := pl.nodeMaxAvailableReplica(node, requirements.ResourceRequest)
		atomic.AddInt32(&res, maxReplica)
	}
	pl.parallelizer.Until(ctx, len(allNodes), processNode)

	return res, framework.NewResult(framework.Success)
}

func (pl *nodeResourceEstimator) nodeMaxAvailableReplica(node *schedulerframework.NodeInfo, rl corev1.ResourceList) int32 {
	rest := node.Allocatable.Clone().SubResource(node.Requested)
	// The number of pods in a node is a kind of resource in node allocatable resources.
	// However, total requested resources of all pods on this node, i.e. `node.Requested`,
	// do not contain pod resources. So after subtraction, we should cope with allowed pod
	// number manually which is the upper bound of this node available replicas.
	rest.AllowedPodNumber = util.MaxInt64(rest.AllowedPodNumber-int64(len(node.Pods)), 0)
	return int32(rest.MaxDivided(rl)) // #nosec G115: integer overflow conversion int64 -> int32
}

// EstimateComponents estimates the maximum number of complete component sets that can be scheduled.
// It returns the number of sets that can fit on the available node resources.
func (pl *nodeResourceEstimator) EstimateComponents(_ context.Context, snapshot *schedcache.Snapshot, components []pb.Component, _ string) (int32, *framework.Result) {
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return noNodeConstraint, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}

	if len(components) == 0 {
		klog.V(5).Infof("%s: received empty components list", pl.Name())
		return noNodeConstraint, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s received empty components list", pl.Name()))
	}

	nodes, err := getNodesAvailableResources(snapshot)
	if err != nil {
		return 0, framework.AsResult(err)
	}

	var sets int32
	// Keep scheduling full component sets until one fails to fit.
	for scheduleComponentSet(components, nodes) {
		sets++
	}

	if sets == 0 {
		return 0, framework.NewResult(framework.Unschedulable, "no enough resources")
	}
	return sets, framework.NewResult(framework.Success)
}

// getNodesAvailableResources retrieves and prepares the list of node information from the snapshot.
// It clones each node's info and adjusts the allocatable resources by subtracting the requested resources.
// So that the returned node infos reflect the actual available resources for scheduling.
func getNodesAvailableResources(snapshot *schedcache.Snapshot) ([]*schedulerframework.NodeInfo, error) {
	allNodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return nil, err
	}

	rest := make([]*schedulerframework.NodeInfo, 0, len(allNodes))
	for _, node := range allNodes {
		n := node.Clone()
		n.Allocatable.SubResource(n.Requested)
		n.Allocatable.AllowedPodNumber = util.MaxInt64(n.Allocatable.AllowedPodNumber-int64(len(node.Pods)), 0)
		rest = append(rest, n)
	}

	return rest, nil
}

// scheduleComponentSet attempts to schedule one complete set of components across the available nodes.
// It returns true if all components in the set can be successfully scheduled, false otherwise.
// The function modifies the node resources as it assigns replicas to simulate actual scheduling.
func scheduleComponentSet(components []pb.Component, allNodes []*schedulerframework.NodeInfo) bool {
	for _, component := range components {
		if !scheduleComponent(component, allNodes) {
			return false
		}
	}

	return true
}

// scheduleComponent attempts to schedule all replicas of a single component across the available nodes.
// It iterates through nodes to find suitable ones and schedules as many replicas as possible on each node.
// Returns true if all replicas of the component can be successfully scheduled, false otherwise.
func scheduleComponent(component pb.Component, allNodes []*schedulerframework.NodeInfo) bool {
	t := newSchedulingTask(component)

	for _, node := range allNodes {
		if !matchNode(t.nodeClaim, node) {
			continue
		}

		for node.Allocatable.Allocatable(t.requiredResourcePerReplica) {
			// Assign one replica to this node.
			t.scheduleOnePod(node)
			if t.done() {
				// short path
				return true
			}
		}
	}

	return t.done()
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

// matchNode checks whether the node matches the scheduling constraints defined in the replica requirements.
func matchNode(nodeClaim *pb.NodeClaim, node *schedulerframework.NodeInfo) bool {
	affinity := nodeutil.GetRequiredNodeAffinity(pb.ReplicaRequirements{NodeClaim: nodeClaim})
	var tolerations []corev1.Toleration

	if nodeClaim != nil {
		tolerations = nodeClaim.Tolerations
	}

	if !nodeutil.IsNodeAffinityMatched(node.Node(), affinity) || !nodeutil.IsTolerationMatched(node.Node(), tolerations) {
		return false
	}

	return true
}
