/*
Copyright The Karmada Authors.

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
func (se *SchedulerEstimator) MaxAvailableReplicas(parentCtx context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error) {
	return getClusterReplicasConcurrently(parentCtx, clusters, se.timeout, func(ctx context.Context, cluster string) (int32, error) {
		return se.maxAvailableReplicas(ctx, cluster, replicaRequirements.DeepCopy())
	})
}

func (se *SchedulerEstimator) maxAvailableReplicas(ctx context.Context, cluster string, replicaRequirements *workv1alpha2.ReplicaRequirements) (int32, error) {
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

func getClusterReplicasConcurrently(parentCtx context.Context, clusters []*clusterv1alpha1.Cluster,
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
		}(i, clusters[i].Name)
	}
	wg.Wait()

	return availableTargetClusters, util.AggregateErrors(errChan)
}
