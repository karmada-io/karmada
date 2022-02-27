package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/util"
)

// RegisterSchedulerEstimator will register a SchedulerEstimator.
func RegisterSchedulerEstimator(se *SchedulerEstimator) {
	replicaEstimators["scheduler-estimator"] = se
	unschedulableReplicaEstimators["scheduler-estimator"] = se
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
func (se *SchedulerEstimator) MaxAvailableReplicas(
	parentCtx context.Context,
	clusters []*clusterv1alpha1.Cluster,
	replicaRequirements *workv1alpha2.ReplicaRequirements,
) ([]workv1alpha2.TargetCluster, error) {
	clusterNames := make([]string, len(clusters))
	for i, cluster := range clusters {
		clusterNames[i] = cluster.Name
	}
	return getClusterReplicasConcurrently(parentCtx, clusterNames, se.timeout, func(ctx context.Context, cluster string) (int32, error) {
		return se.maxAvailableReplicas(ctx, cluster, replicaRequirements.DeepCopy())
	})
}

// GetUnschedulableReplicas gets the unschedulable replicas which belong to a specified workload by calling karmada-scheduler-estimator.
func (se *SchedulerEstimator) GetUnschedulableReplicas(
	parentCtx context.Context,
	clusters []string,
	reference *workv1alpha2.ObjectReference,
	unscheduableThreshold time.Duration,
) ([]workv1alpha2.TargetCluster, error) {
	return getClusterReplicasConcurrently(parentCtx, clusters, se.timeout, func(ctx context.Context, cluster string) (int32, error) {
		return se.maxUnscheduableReplicas(ctx, cluster, reference.DeepCopy(), unscheduableThreshold)
	})
}

func (se *SchedulerEstimator) maxAvailableReplicas(ctx context.Context, cluster string, replicaRequirements *workv1alpha2.ReplicaRequirements) (int32, error) {
	client, err := se.cache.GetClient(cluster)
	if err != nil {
		return UnauthenticReplica, err
	}

	req := &pb.MaxAvailableReplicasRequest{
		Cluster:             cluster,
		ReplicaRequirements: pb.ReplicaRequirements{},
	}
	if replicaRequirements != nil {
		req.ReplicaRequirements.ResourceRequest = replicaRequirements.ResourceRequest
		if replicaRequirements.NodeClaim != nil {
			req.ReplicaRequirements.NodeClaim = &pb.NodeClaim{
				NodeAffinity: replicaRequirements.NodeClaim.HardNodeAffinity,
				NodeSelector: replicaRequirements.NodeClaim.NodeSelector,
				Tolerations:  replicaRequirements.NodeClaim.Tolerations,
			}
		}
	}
	res, err := client.MaxAvailableReplicas(ctx, req)
	if err != nil {
		return UnauthenticReplica, fmt.Errorf("gRPC request cluster(%s) estimator error when calling MaxAvailableReplicas: %v", cluster, err)
	}
	return res.MaxReplicas, nil
}

func (se *SchedulerEstimator) maxUnscheduableReplicas(
	ctx context.Context,
	cluster string,
	reference *workv1alpha2.ObjectReference,
	threshold time.Duration,
) (int32, error) {
	client, err := se.cache.GetClient(cluster)
	if err != nil {
		return UnauthenticReplica, err
	}

	req := &pb.UnschedulableReplicasRequest{
		Cluster: cluster,
		Resource: pb.ObjectReference{
			APIVersion: reference.APIVersion,
			Kind:       reference.Kind,
			Namespace:  reference.Namespace,
			Name:       reference.Name,
		},
		UnschedulableThreshold: threshold,
	}
	res, err := client.GetUnschedulableReplicas(ctx, req)
	if err != nil {
		return UnauthenticReplica, fmt.Errorf("gRPC request cluster(%s) estimator error when calling UnschedulableReplicas: %v", cluster, err)
	}
	return res.UnschedulableReplicas, nil
}

func getClusterReplicasConcurrently(parentCtx context.Context, clusters []string,
	timeout time.Duration, getClusterReplicas getClusterReplicasFunc) ([]workv1alpha2.TargetCluster, error) {
	// add object information into gRPC metadata
	if u, ok := parentCtx.Value(util.ContextKeyObject).(string); ok {
		parentCtx = metadata.AppendToOutgoingContext(parentCtx, string(util.ContextKeyObject), u)
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))

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
			availableTargetClusters[idx] = workv1alpha2.TargetCluster{Name: cluster, Replicas: replicas}
		}(i, clusters[i])
	}
	wg.Wait()

	return availableTargetClusters, util.AggregateErrors(errChan)
}
