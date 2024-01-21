/*
Copyright 2021 The Karmada Authors.

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
	"time"

	"github.com/emirpasic/gods/maps/treemap"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// UnauthenticReplica is special replica number returned by estimator in case of estimator can't calculate the available
// replicas.
// The scheduler should discard the estimator's result and back-off to rely on other estimator's result.
const UnauthenticReplica = -1

var (
	// replicaEstimators are organized in a TreeMap, sorted by descending priority.
	// The key is of type EstimatorPriority, indicating the estimator's priority level.
	// The value is a map with string keys and ReplicaEstimator values, grouping estimators by their respective priorities.
	replicaEstimators = treemap.NewWith(estimatorPriorityComparator)

	// unschedulableReplicaEstimators are organized in a TreeMap, sorted by descending priority.
	// The key is of type EstimatorPriority, indicating the estimator's priority level.
	// The value is a map with string keys and UnschedulableReplicaEstimator values, grouping estimators by their respective priorities.
	unschedulableReplicaEstimators = treemap.NewWith(estimatorPriorityComparator)
)

// registerReplicaEstimator add a estimator to replicaEstimators
func registerReplicaEstimator(estimatorName string, estimator ReplicaEstimator) {
	if val, ok := replicaEstimators.Get(estimator.Priority()); !ok {
		replicaEstimators.Put(estimator.Priority(), map[string]ReplicaEstimator{estimatorName: estimator})
	} else {
		estimatorsWithSamePriority := val.(map[string]ReplicaEstimator)
		estimatorsWithSamePriority[estimatorName] = estimator
	}
}

// registerUnschedulableReplicaEstimator add a estimator to unschedulableReplicaEstimators
func registerUnschedulableReplicaEstimator(estimatorName string, estimator UnschedulableReplicaEstimator) {
	if val, ok := unschedulableReplicaEstimators.Get(estimator.Priority()); !ok {
		unschedulableReplicaEstimators.Put(estimator.Priority(), map[string]UnschedulableReplicaEstimator{estimatorName: estimator})
	} else {
		estimatorsWithSamePriority := val.(map[string]UnschedulableReplicaEstimator)
		estimatorsWithSamePriority[estimatorName] = estimator
	}
}

// ReplicaEstimator is an estimator which estimates the maximum replicas that can be applied to the target cluster.
type ReplicaEstimator interface {
	MaxAvailableReplicas(ctx context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error)
	Priority() EstimatorPriority
}

// UnschedulableReplicaEstimator is an estimator which estimates the unschedulable replicas which belong to a specified workload.
type UnschedulableReplicaEstimator interface {
	GetUnschedulableReplicas(ctx context.Context, clusters []string, reference *workv1alpha2.ObjectReference, unschedulableThreshold time.Duration) ([]workv1alpha2.TargetCluster, error)
	Priority() EstimatorPriority
}

// GetReplicaEstimators returns all replica estimators.
func GetReplicaEstimators() *treemap.Map {
	return replicaEstimators
}

// GetUnschedulableReplicaEstimators returns all unschedulable replica estimators.
func GetUnschedulableReplicaEstimators() *treemap.Map {
	return unschedulableReplicaEstimators
}

// EstimatorPriority the priority of estimator
//  1. If two estimators are of the same priority, call both and choose the minimum value of each estimated result.
//  2. If higher-priority estimators have formed a full result of member clusters, no longer to call lower-priority estimator.
//  3. If higher-priority estimators haven't given the result for certain member clusters, lower-priority estimator will
//     continue to estimate for such clusters haven't got a result.
type EstimatorPriority int32

const (
	// General general priority, e.g: ResourceModel
	General EstimatorPriority = 10
	// Accurate accurate priority, e.g: SchedulerEstimator
	Accurate EstimatorPriority = 20
)

// estimatorPriorityComparator provides a basic comparison on EstimatorPriority.
func estimatorPriorityComparator(a, b interface{}) int {
	aAsserted := a.(EstimatorPriority)
	bAsserted := b.(EstimatorPriority)
	switch {
	case aAsserted > bAsserted:
		return -1
	case aAsserted < bAsserted:
		return 1
	default:
		return 0
	}
}
