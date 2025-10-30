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
	"time"

	utiltrace "k8s.io/utils/trace"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
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

func (es *AccurateSchedulerEstimatorServer) estimateReplicas(ctx context.Context, snapshot *schedcache.Snapshot, requirements pb.ReplicaRequirements) (int32, error) {
	replicas, ret := es.estimateFramework.RunEstimateReplicasPlugins(ctx, snapshot, &requirements)

	// No replicas can be scheduled on the cluster, skip further checks and return 0
	if ret.IsUnschedulable() {
		return 0, nil
	}

	if ret.IsFailed() {
		return 0, fmt.Errorf("estimate replica plugins fails with %s", ret.Reasons())
	}

	return replicas, nil
}

// EstimateComponents returns max available component sets in terms of request and cluster status.
func (es *AccurateSchedulerEstimatorServer) EstimateComponents(ctx context.Context, object string, request *pb.MaxAvailableComponentSetsRequest) (int32, error) {
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

	maxAvailableComponentSets, err := es.estimateComponents(ctx, snapShot, request.Components)
	if err != nil {
		return 0, err
	}
	trace.Step("Computing estimation done")

	return maxAvailableComponentSets, nil
}

func (es *AccurateSchedulerEstimatorServer) estimateComponents(
	ctx context.Context,
	snapshot *schedcache.Snapshot,
	components []pb.Component,
) (int32, error) {
	maxSets, ret := es.estimateFramework.RunEstimateComponentsPlugins(ctx, snapshot, components)

	// No replicas can be scheduled on the cluster, skip further checks and return 0
	if ret.IsUnschedulable() {
		return 0, nil
	}

	if !ret.IsSuccess() && !ret.IsNoOperation() {
		return maxSets, fmt.Errorf("estimate components plugins fails with %s", ret.Reasons())
	}

	// TODO - Node info has not be included in this implementation. Node Resource Estimation will be moved to a separate plugin.

	// If no plugins were run (NoOperation), return maxSets value to prevent scheduling failure
	if ret.IsSuccess() || ret.IsNoOperation() {
		return maxSets, nil
	}
	return 0, nil
}
