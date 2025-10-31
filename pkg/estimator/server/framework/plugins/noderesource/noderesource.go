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
		return math.MaxInt32, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
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

// EstimateComponents the sets allowed by the node resources for a given pb.Component.
func (pl *nodeResourceEstimator) EstimateComponents(_ context.Context, snapshot *schedcache.Snapshot, components []pb.Component) (int32, *framework.Result) {
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return math.MaxInt32, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}

	if len(components) == 0 {
		return 0, framework.AsResult(fmt.Errorf("no components specified"))
	}

	nodes, err := getNodeRestResource(snapshot)
	if err != nil {
		return 0, framework.AsResult(err)
	}

	var sets int32
	for canAssignOneComponentSets(newTasks(components), nodes) {
		sets++
	}

	if sets == 0 {
		return 0, framework.NewResult(framework.Unschedulable, "no enough resources")
	}
	return sets, framework.NewResult(framework.Success)
}

// getNodeRestResource calculates the remaining available resources for each node in the cluster.
// It clones each node and subtracts the already requested resources and existing pod count
// to determine how much capacity is left for new workloads.
// Returns a slice of NodeInfo with updated allocatable resources representing available capacity.
func getNodeRestResource(snapshot *schedcache.Snapshot) ([]*schedulerframework.NodeInfo, error) {
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

// canAssignOneComponentSets attempts to schedule one complete set of components across the available nodes.
// It returns true if all components in the set can be successfully scheduled, false otherwise.
// The function modifies the node resources as it assigns replicas to simulate actual scheduling.
func canAssignOneComponentSets(ts *tasks, allNodes []*schedulerframework.NodeInfo) bool {
	for !ts.done() {
		i, t := ts.getTask()
		if i == -1 {
			// No more tasks to schedule, but done() returned false - this shouldn't happen
			return false
		}

		scheduled := false
		for _, node := range allNodes {
			if !matchNode(t, node) {
				continue
			}
			needResource := util.NewResource(t.ResourceRequest)
			needResource.AllowedPodNumber = 1
			if node.Allocatable.Allocatable(needResource) {
				// Assign one replica to this node.
				node.Allocatable.SubResource(needResource)
				ts.scheduleOne(i)
				scheduled = true
				break
			}
		}

		if !scheduled {
			// No node can fit this task, cannot complete the component set
			return false
		}
	}

	return ts.done()
}

// task represents a single component type with its scheduling requirements and remaining replicas.
type task struct {
	// replicaRequirements defines the resource and scheduling constraints for each replica
	replicaRequirements pb.ReplicaRequirements
	// toBeScheduled tracks how many replicas of this component still need to be scheduled
	toBeScheduled int32
}

// tasks manages a collection of component tasks for scheduling estimation.
// It tracks the remaining replicas for each component type that need to be scheduled.
type tasks struct {
	// items contains all component tasks to be scheduled
	items []task
}

// newTasks creates a new task collection from the given components.
// Each component is converted to a task with its replica requirements and count.
func newTasks(components []pb.Component) *tasks {
	ts := make([]task, 0, len(components))
	for _, component := range components {
		ts = append(ts, task{
			replicaRequirements: component.ReplicaRequirements,
			toBeScheduled:       component.Replicas,
		})
	}

	return &tasks{
		items: ts,
	}
}

// getTask returns the index and replica requirements of the first task that still needs to be scheduled.
// It scans through all tasks to find one with remaining replicas to schedule.
// Returns (-1, empty ReplicaRequirements) if no unfinished tasks are found.
func (t *tasks) getTask() (int, pb.ReplicaRequirements) {
	for i := 0; i < len(t.items); i++ {
		if t.items[i].toBeScheduled > 0 {
			return i, t.items[i].replicaRequirements
		}
	}

	return -1, pb.ReplicaRequirements{}
}

// done returns true if all tasks have been completely scheduled (no replicas remaining).
// This indicates that a complete component set has been successfully allocated.
func (t *tasks) done() bool {
	for _, tk := range t.items {
		if tk.toBeScheduled > 0 {
			return false
		}
	}
	return true
}

// scheduleOne decrements the replica count for the task at the specified index.
// This should be called when a replica has been successfully scheduled on a node.
func (t *tasks) scheduleOne(index int) {
	if index < 0 || index >= len(t.items) {
		// Invalid index - defensive programming
		return
	}
	if t.items[index].toBeScheduled > 0 {
		t.items[index].toBeScheduled--
	}
}

// matchNode checks whether the node matches the replicaRequirements' node affinity and tolerations.
func matchNode(replicaRequirements pb.ReplicaRequirements, node *schedulerframework.NodeInfo) bool {
	affinity := nodeutil.GetRequiredNodeAffinity(replicaRequirements)
	var tolerations []corev1.Toleration

	if replicaRequirements.NodeClaim != nil {
		tolerations = replicaRequirements.NodeClaim.Tolerations
	}

	if !nodeutil.IsNodeAffinityMatched(node.Node(), affinity) || !nodeutil.IsTolerationMatched(node.Node(), tolerations) {
		return false
	}

	return true
}
