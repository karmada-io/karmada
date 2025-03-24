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

package server

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	utiltrace "k8s.io/utils/trace"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
	"github.com/karmada-io/karmada/pkg/util"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

// EstimateReplicas returns max available replicas in terms of request and cluster status.
func (es *AccurateSchedulerEstimatorServer) EstimateReplicas(ctx context.Context, object string, request *pb.MaxAvailableReplicasRequest) (int32, error) {
	trace := utiltrace.New("Estimating", utiltrace.Field{Key: "namespacedName", Value: object})
	defer trace.LogIfLong(100 * time.Millisecond)

	snapShot := schedcache.NewEmptySnapshot()
	if err := es.Cache.UpdateSnapshot(snapShot); err != nil {
		return 0, err
	}
	trace.Step("Snapshotting estimator cache and node infos done")

	if snapShot.NumNodes() == 0 {
		return 0, nil
	}

	maxAvailableReplicas, err := es.estimateReplicas(ctx, snapShot, request.ReplicaRequirements)
	if err != nil {
		return 0, err
	}
	trace.Step("Computing estimation done")

	return maxAvailableReplicas, nil
}

func (es *AccurateSchedulerEstimatorServer) estimateReplicas(
	ctx context.Context,
	snapshot *schedcache.Snapshot,
	requirements pb.ReplicaRequirements,
) (int32, error) {
	allNodes, err := snapshot.NodeInfos().List()
	if err != nil {
		return 0, err
	}
	var (
		affinity    = nodeutil.GetRequiredNodeAffinity(requirements)
		tolerations []corev1.Toleration
	)

	if requirements.NodeClaim != nil {
		tolerations = requirements.NodeClaim.Tolerations
	}

	var res int32
	replicas, ret := es.estimateFramework.RunEstimateReplicasPlugins(ctx, snapshot, &requirements)

	// No replicas can be scheduled on the cluster, skip further checks and return 0
	if ret.IsUnschedulable() {
		return 0, nil
	}

	if !ret.IsSuccess() && !ret.IsNoOperation() {
		return replicas, fmt.Errorf("estimate replica plugins fails with %s", ret.Reasons())
	}
	processNode := func(i int) {
		node := allNodes[i]
		if !nodeutil.IsNodeAffinityMatched(node.Node(), affinity) || !nodeutil.IsTolerationMatched(node.Node(), tolerations) {
			return
		}
		maxReplica := es.nodeMaxAvailableReplica(node, requirements.ResourceRequest)
		atomic.AddInt32(&res, maxReplica)
	}
	es.parallelizer.Until(ctx, len(allNodes), processNode)

	if ret.IsSuccess() && replicas < res {
		res = replicas
	}
	return res, nil
}

func (es *AccurateSchedulerEstimatorServer) nodeMaxAvailableReplica(node *framework.NodeInfo, rl corev1.ResourceList) int32 {
	rest := node.Allocatable.Clone().SubResource(node.Requested)
	// The number of pods in a node is a kind of resource in node allocatable resources.
	// However, total requested resources of all pods on this node, i.e. `node.Requested`,
	// do not contain pod resources. So after subtraction, we should cope with allowed pod
	// number manually which is the upper bound of this node available replicas.
	rest.AllowedPodNumber = util.MaxInt64(rest.AllowedPodNumber-int64(len(node.Pods)), 0)
	return int32(rest.MaxDivided(rl)) // #nosec G115: integer overflow conversion int64 -> int32
}
