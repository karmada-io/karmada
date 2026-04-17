/*
Copyright 2026 The Karmada Authors.

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

package nodeautoscaler

import (
	"context"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
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
	Name = "NodeAutoscalerEstimator"
)

// nodeAutoscalerEstimator extends NodeResourceEstimator by adding potential
// capacity from node autoscalers (e.g. Karpenter).
type nodeAutoscalerEstimator struct {
	parallelizer parallelize.Parallelizer
	providers    []CapacityProvider
}

var _ framework.EstimateReplicasPlugin = &nodeAutoscalerEstimator{}

// New initializes a new plugin and returns it.
func New(fh framework.Handle) (framework.Plugin, error) {
	pl := &nodeAutoscalerEstimator{
		parallelizer: parallelize.NewParallelizer(fh.Parallelism()),
	}
	if dc := fh.DynamicClient(); dc != nil {
		pl.providers = []CapacityProvider{NewKarpenterProvider(dc)}
	}
	return pl, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *nodeAutoscalerEstimator) Name() string {
	return Name
}

// Estimate returns currentAvailable (from existing nodes) + potentialReplicas (from autoscaler).
func (pl *nodeAutoscalerEstimator) Estimate(ctx context.Context, snapshot *schedcache.Snapshot, requirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	current := pl.estimateFromExistingNodes(ctx, snapshot, requirements)
	hasPending := pl.hasPendingPods(snapshot)
	potential := pl.estimateFromProviders(ctx, requirements, hasPending)
	return current + potential, framework.NewResult(framework.Success)
}

// estimateFromExistingNodes replicates the NodeResourceEstimator logic.
func (pl *nodeAutoscalerEstimator) estimateFromExistingNodes(ctx context.Context, snapshot *schedcache.Snapshot, requirements *pb.ReplicaRequirements) int32 {
	allNodes, err := snapshot.NodeInfos().List()
	if err != nil || len(allNodes) == 0 {
		return 0
	}

	affinity, tolerations := estimator.GetAffinityAndTolerations(requirements.NodeClaim)

	var res int32
	processNode := func(i int) {
		node := allNodes[i].Clone()
		if !estimator.MatchNode(node, affinity, tolerations) {
			return
		}
		rest := nodeAvailableResource(node)
		maxReplica := int32(rest.MaxDivided(requirements.ResourceRequest)) // #nosec G115
		atomic.AddInt32(&res, maxReplica)
	}
	pl.parallelizer.Until(ctx, len(allNodes), processNode)
	return res
}

func nodeAvailableResource(node *schedulerframework.NodeInfo) *util.Resource {
	rest := node.Allocatable
	rest = rest.SubResource(node.Requested)
	rest.AllowedPodNumber = util.MaxInt64(rest.AllowedPodNumber-int64(len(node.Pods)), 0)
	return rest
}

// estimateFromProviders sums potential replicas from all available providers.
func (pl *nodeAutoscalerEstimator) estimateFromProviders(ctx context.Context, requirements *pb.ReplicaRequirements, hasPendingPods bool) int32 {
	var total int32
	for _, provider := range pl.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}
		potential, err := provider.GetPotentialReplicas(ctx, requirements, hasPendingPods)
		if err != nil {
			klog.Warningf("CapacityProvider error: %v", err)
			continue
		}
		total += potential
	}
	return total
}

// hasPendingPods checks if there are any Pending pods in the snapshot.
func (pl *nodeAutoscalerEstimator) hasPendingPods(snapshot *schedcache.Snapshot) bool {
	nodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return false
	}
	for _, node := range nodes {
		for _, pod := range node.Pods {
			if pod.Pod.Status.Phase == corev1.PodPending {
				return true
			}
		}
	}
	return false
}
