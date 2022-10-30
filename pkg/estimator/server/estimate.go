package server

import (
	"context"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	utiltrace "k8s.io/utils/trace"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
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

	// TODO(Garrybest): design a framework and make filter and score plugins
	var res int32
	processNode := func(i int) {
		node := allNodes[i]
		if !nodeutil.IsNodeAffinityMatched(node.Node(), affinity) || !nodeutil.IsTolerationMatched(node.Node(), tolerations) {
			return
		}
		maxReplica := es.nodeMaxAvailableReplica(node, requirements.ResourceRequest)
		atomic.AddInt32(&res, maxReplica)
	}
	es.parallelizer.Until(ctx, len(allNodes), processNode)
	return res, nil
}

func (es *AccurateSchedulerEstimatorServer) nodeMaxAvailableReplica(node *framework.NodeInfo, rl corev1.ResourceList) int32 {
	rest := node.Allocatable.Clone().SubResource(node.Requested)
	if rest.AllowedPodNumber > int64(len(node.Pods)) {
		rest.AllowedPodNumber -= int64(len(node.Pods))
	}
	return int32(rest.MaxDivided(rl))
}
