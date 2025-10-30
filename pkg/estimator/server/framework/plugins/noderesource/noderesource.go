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
