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
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	"github.com/karmada-io/karmada/pkg/features"
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

	estCtx := framework.ReplicaEstimationContext{
		Snapshot:            snapShot,
		ReplicaRequirements: request.GetReplicaRequirements(),
	}
	if features.FeatureGate.Enabled(features.SchedulingOvercommitProtection) {
		estCtx.AssumedWorkloads = request.GetAssumedWorkloads()
	}
	maxAvailableReplicas, err := es.estimateReplicas(ctx, estCtx)
	if err != nil {
		return 0, err
	}
	trace.Step("Computing estimation done")

	return maxAvailableReplicas, nil
}

func (es *AccurateSchedulerEstimatorServer) estimateReplicas(ctx context.Context, estCtx framework.ReplicaEstimationContext) (int32, error) {
	replicas, ret := es.estimateFramework.RunEstimateReplicasPlugins(ctx, estCtx)

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

	estCtx := framework.ComponentEstimationContext{
		Snapshot:   snapShot,
		Components: request.Components,
		Namespace:  request.Namespace,
	}
	if features.FeatureGate.Enabled(features.SchedulingOvercommitProtection) {
		estCtx.AssumedWorkloads = request.GetAssumedWorkloads()
	}
	maxAvailableComponentSets, err := es.estimateComponents(ctx, estCtx)
	if err != nil {
		return 0, err
	}
	trace.Step("Computing estimation done")

	return maxAvailableComponentSets, nil
}

func (es *AccurateSchedulerEstimatorServer) estimateComponents(ctx context.Context, estCtx framework.ComponentEstimationContext) (int32, error) {
	maxSets, ret := es.estimateFramework.RunEstimateComponentsPlugins(ctx, estCtx)

	// No replicas can be scheduled on the cluster, skip further checks and return 0
	if ret.IsUnschedulable() {
		return 0, nil
	}

	if !ret.IsSuccess() && !ret.IsNoOperation() {
		return maxSets, fmt.Errorf("estimate components plugins fails with %s", ret.Reasons())
	}

	// If no plugins were run (NoOperation), return maxSets value to prevent scheduling failure
	if ret.IsSuccess() || ret.IsNoOperation() {
		return maxSets, nil
	}
	return 0, nil
}
