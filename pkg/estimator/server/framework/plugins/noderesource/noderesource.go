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
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/estimator"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	"github.com/karmada-io/karmada/pkg/util"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
)

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "NodeResourceEstimator"

	// noNodeConstraint represents the value when there is no node resource constraint.
	noNodeConstraint = math.MaxInt32
)

// nodeResourceEstimator estimates replicas/sets allowed by node resources.
// It always calculates capacity from existing nodes, and optionally adds
// potential capacity from additional providers (e.g., karpenter).
type nodeResourceEstimator struct {
	parallelizer parallelize.Parallelizer
	providers    []CapacityProvider // additional providers (potential capacity)
}

var _ framework.EstimateReplicasPlugin = &nodeResourceEstimator{}
var _ framework.EstimateComponentsPlugin = &nodeResourceEstimator{}

// New initializes a new plugin and returns it.
// --node-capacity-providers specifies additional providers beyond the built-in
// existing-node calculation. If empty, only existing nodes are used.
func New(fh framework.Handle) (framework.Plugin, error) {
	pl := &nodeResourceEstimator{
		parallelizer: parallelize.NewParallelizer(fh.Parallelism()),
	}
	for _, name := range fh.NodeCapacityProviders() {
		factory, ok := LookupProvider(name)
		if !ok {
			return nil, fmt.Errorf("unknown node capacity provider %q (registered: %v)", name, RegisteredProviders())
		}
		p, err := factory(fh)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize node capacity provider %q: %w", name, err)
		}
		pl.providers = append(pl.providers, p)
	}
	return pl, nil
}

// Name returns name of the plugin.
func (pl *nodeResourceEstimator) Name() string {
	return Name
}

// Estimate calculates available replicas from existing nodes (always) plus
// any additional capacity from configured providers (summed).
func (pl *nodeResourceEstimator) Estimate(ctx context.Context, snapshot *schedcache.Snapshot, requirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	current, ret := pl.estimateFromExistingNodes(ctx, snapshot, requirements)
	if ret.IsFailed() {
		return 0, ret
	}

	var additional int32
	for _, p := range pl.providers {
		replicas, ret := p.Estimate(ctx, snapshot, requirements)
		if ret.IsFailed() {
			return 0, ret
		}
		if !ret.IsNoOperation() {
			additional += replicas
		}
	}

	return current + additional, framework.NewResult(framework.Success)
}

// estimateFromExistingNodes calculates available replicas from current nodes.
func (pl *nodeResourceEstimator) estimateFromExistingNodes(ctx context.Context, snapshot *schedcache.Snapshot, requirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	allNodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return 0, framework.AsResult(err)
	}
	if len(allNodes) == 0 {
		return 0, framework.NewResult(framework.Success)
	}

	var affinity nodeaffinity.RequiredNodeAffinity
	var tolerations []corev1.Toleration
	var resourceRequest corev1.ResourceList
	if requirements != nil {
		affinity, tolerations, err = estimator.GetAffinityAndTolerations(requirements.NodeClaim)
		if err != nil {
			return 0, framework.AsResult(err)
		}

		resourceRequest, err = requirements.UnmarshalResourceRequest()
		if err != nil {
			return 0, framework.AsResult(err)
		}
	}

	var res int32
	processNode := func(i int) {
		node := allNodes[i].Clone()
		if !estimator.MatchNode(node, affinity, tolerations) {
			return
		}
		rest := getNodeAvailableResource(node)
		maxReplica := int32(rest.MaxDivided(resourceRequest)) // #nosec G115
		atomic.AddInt32(&res, maxReplica)
	}
	pl.parallelizer.Until(ctx, len(allNodes), processNode)
	return res, framework.NewResult(framework.Success)
}

// EstimateComponents estimates the maximum number of complete component sets.
// This uses only existing node resources (additional providers cannot simulate
// bin-packing across nodes that don't yet exist).
func (pl *nodeResourceEstimator) EstimateComponents(_ context.Context, estCtx framework.ComponentEstimationContext) (int32, *framework.Result) {
	components := estCtx.Components
	if len(components) == 0 {
		klog.V(5).Infof("%s: received empty components list", Name)
		return noNodeConstraint, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s received empty components list", Name))
	}

	nodes, err := getNodesAvailableResources(estCtx.Snapshot)
	if err != nil {
		return 0, framework.AsResult(err)
	}

	sets, err := estimator.NewSchedulingSimulator(nodes).SimulateScheduling(components, math.MaxInt32)
	if err != nil {
		return 0, framework.AsResult(err)
	}
	if sets == 0 {
		return 0, framework.NewResult(framework.Unschedulable, "no enough resources")
	}
	return sets, framework.NewResult(framework.Success)
}

// getNodesAvailableResources retrieves and prepares the list of node information from the snapshot.
func getNodesAvailableResources(snapshot *schedcache.Snapshot) ([]*schedulerframework.NodeInfo, error) {
	allNodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return nil, err
	}

	rest := make([]*schedulerframework.NodeInfo, 0, len(allNodes))
	for _, node := range allNodes {
		n := node.Clone()
		n.Allocatable = getNodeAvailableResource(n)
		rest = append(rest, n)
	}

	return rest, nil
}

// getNodeAvailableResource calculates a node's available resources after deducting requested resources.
func getNodeAvailableResource(node *schedulerframework.NodeInfo) *util.Resource {
	rest := node.Allocatable
	rest = rest.SubResource(node.Requested)
	rest.AllowedPodNumber = util.MaxInt64(rest.AllowedPodNumber-int64(len(node.Pods)), 0)
	return rest
}
