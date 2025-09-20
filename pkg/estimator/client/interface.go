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

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// UnauthenticReplica is special replica number returned by estimator in case of estimator can't calculate the available
// replicas.
// The scheduler should discard the estimator's result and back-off to rely on other estimator's result.
const UnauthenticReplica = -1

var (
	replicaEstimators              = map[string]ReplicaEstimator{}
	unschedulableReplicaEstimators = map[string]UnschedulableReplicaEstimator{}
)

// ReplicaEstimator estimates how many replicas a workload can run on each target cluster,
// based on resource availability and node constraints.
type ReplicaEstimator interface {
	// MaxAvailableReplicas returns the maximum number of replicas of a single-component workload that each cluster can host.
	MaxAvailableReplicas(ctx context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error)
}

// ComponentSetEstimator extends ReplicaEstimator to handle multi-component workloads,
// where all components in a set must be scheduled together.
type ComponentSetEstimator interface {
	ReplicaEstimator
	// MaxAvailableComponentSets returns the maximum number of complete multi-component sets (in terms of replicas) that each cluster can host.
	MaxAvailableComponentSets(ctx context.Context, clusters []*clusterv1alpha1.Cluster, components []*workv1alpha2.Component) ([]workv1alpha2.TargetCluster, error)
}

// UnschedulableReplicaEstimator is an estimator which estimates the unschedulable replicas which belong to a specified workload.
type UnschedulableReplicaEstimator interface {
	GetUnschedulableReplicas(ctx context.Context, clusters []string, reference *workv1alpha2.ObjectReference, unschedulableThreshold time.Duration) ([]workv1alpha2.TargetCluster, error)
}

// GetReplicaEstimators returns all replica estimators.
func GetReplicaEstimators() map[string]ReplicaEstimator {
	return replicaEstimators
}

// GetUnschedulableReplicaEstimators returns all unschedulable replica estimators.
func GetUnschedulableReplicaEstimators() map[string]UnschedulableReplicaEstimator {
	return unschedulableReplicaEstimators
}
