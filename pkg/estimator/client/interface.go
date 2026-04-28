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
	// MaxAvailableComponentSets returns the maximum number of complete multi-component sets (in terms of replicas) that each cluster can host.
	MaxAvailableComponentSets(ctx context.Context, req ComponentSetEstimationRequest) ([]ComponentSetEstimationResponse, error)
}

// ComponentSetEstimationRequest carries input parameters for estimating multi-component set availability per cluster.
// Fields can be extended over time without changing the method signature.
type ComponentSetEstimationRequest struct {
	// Clusters represents a list of feasible clusters to estimate against.
	Clusters []*clusterv1alpha1.Cluster
	// Components are the components that form a multi-component workload.
	Components []workv1alpha2.Component
	// Namespace is the namespace of the workload being estimated.
	// It is used by the accurate estimator to check the quota configurations
	// in the target member cluster. This field is required for quota-aware estimation.
	Namespace string
}

// ComponentSetEstimationResponse represents how many complete component sets a cluster can accommodate.
type ComponentSetEstimationResponse struct {
	// Name is the cluster name.
	Name string
	// Sets is the maximum number of complete component sets that can be scheduled on the cluster.
	Sets int32
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
