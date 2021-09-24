package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/util"
)

// RegisterSchedulerEstimator will register a SchedulerEstimator.
func RegisterSchedulerEstimator(se *SchedulerEstimator) {
	replicaEstimators["scheduler-estimator"] = se
}

type getClusterReplicasFunc func(ctx context.Context, cluster string) (int32, error)

// SchedulerEstimator is an estimator that calls karmada-scheduler-estimator for estimation.
type SchedulerEstimator struct {
	cache   *SchedulerEstimatorCache
	timeout time.Duration
}

// NewSchedulerEstimator builds a new SchedulerEstimator.
func NewSchedulerEstimator(cache *SchedulerEstimatorCache, timeout time.Duration) *SchedulerEstimator {
	return &SchedulerEstimator{
		cache:   cache,
		timeout: timeout,
	}
}

// MaxAvailableReplicas estimates the maximum replicas that can be applied to the target cluster by calling karmada-scheduler-estimator.
func (se *SchedulerEstimator) MaxAvailableReplicas(clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha1.ReplicaRequirements) ([]workv1alpha1.TargetCluster, error) {
	return getClusterReplicasConcurrently(clusters, se.timeout, func(ctx context.Context, cluster string) (int32, error) {
		return se.maxAvailableReplicas(ctx, cluster, replicaRequirements.DeepCopy())
	})
}

func (se *SchedulerEstimator) maxAvailableReplicas(ctx context.Context, cluster string, replicaRequirements *workv1alpha1.ReplicaRequirements) (int32, error) {
	client, err := se.cache.GetClient(cluster)
	if err != nil {
		return UnauthenticReplica, err
	}

	req := &pb.MaxAvailableReplicasRequest{
		Cluster: cluster,
		ReplicaRequirements: pb.ReplicaRequirements{
			ResourceRequest: replicaRequirements.ResourceRequest,
		},
	}
	if replicaRequirements.NodeClaim != nil {
		req.ReplicaRequirements.NodeClaim = &pb.NodeClaim{
			NodeAffinity: replicaRequirements.NodeClaim.HardNodeAffinity,
			NodeSelector: replicaRequirements.NodeClaim.NodeSelector,
			Tolerations:  replicaRequirements.NodeClaim.Tolerations,
		}
	}
	res, err := client.MaxAvailableReplicas(ctx, req)
	if err != nil {
		return UnauthenticReplica, fmt.Errorf("gRPC request cluster(%s) estimator error: %v", cluster, err)
	}
	return res.MaxReplicas, nil
}

func getClusterReplicasConcurrently(clusters []*clusterv1alpha1.Cluster, timeout time.Duration, getClusterReplicas getClusterReplicasFunc) ([]workv1alpha1.TargetCluster, error) {
	availableTargetClusters := make([]workv1alpha1.TargetCluster, len(clusters))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(clusters))
	for i := range clusters {
		wg.Add(1)
		go func(idx int, cluster string) {
			defer wg.Done()
			replicas, err := getClusterReplicas(ctx, cluster)
			if err != nil {
				errChan <- err
			}
			availableTargetClusters[idx] = workv1alpha1.TargetCluster{Name: cluster, Replicas: replicas}
		}(i, clusters[i].Name)
	}
	wg.Wait()

	return availableTargetClusters, util.AggregateErrors(errChan)
}
