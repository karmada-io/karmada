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

// UnauthenticReplica is a special replica number returned by an estimator for a single cluster when the estimator
// cannot calculate the value for that cluster. For example, the scheduler ignores only that cluster's value while
// keeping the values of the remaining clusters.
// An estimator may also return usable per-cluster results together with an aggregate error. Consumers that choose to
// tolerate partial results must use this sentinel to skip the affected cluster. Consumers that treat an error as fatal,
// such as the descheduler, keep discarding the whole batch.
const UnauthenticReplica = -1

var (
	replicaEstimators              = map[string]ReplicaEstimator{}
	unschedulableReplicaEstimators = map[string]UnschedulableReplicaEstimator{}
)

// ReplicaEstimator estimates how many replicas a workload can run on each target cluster,
// based on resource availability and node constraints.
type ReplicaEstimator interface {
	// MaxAvailableReplicas returns the maximum number of replicas of a single-component workload that each cluster can host.
	// When an error is returned, implementations must set UnauthenticReplica for every cluster
	// that could not be estimated; successful clusters may still carry their real value.
	MaxAvailableReplicas(ctx context.Context, req ReplicaEstimationRequest) ([]workv1alpha2.TargetCluster, error)
	// MaxAvailableComponentSets returns the maximum number of complete multi-component sets (in terms of replicas) that each cluster can host.
	// The same per-cluster UnauthenticReplica rule applies when an error is returned.
	MaxAvailableComponentSets(ctx context.Context, req ComponentSetEstimationRequest) ([]ComponentSetEstimationResponse, error)
}

// AssumedWorkload represents the resource footprint of an in-flight workload set that has
// been assigned to a cluster but whose pods have not yet been bound to nodes.
// Callers populate this from the scheduler's assumption cache; estimator implementations
// are responsible for converting it to the wire format (e.g., pb.AssumedWorkload) internally.
type AssumedWorkload struct {
	// Namespace is the namespace of the assumed workload.
	Namespace string
	// Components lists the component types and their per-replica resource requirements
	// that are assumed on the target cluster.
	Components []workv1alpha2.Component
}

// ReplicaEstimationRequest carries input parameters for estimating single-template workload
// replica availability per cluster.
type ReplicaEstimationRequest struct {
	// Clusters represents a list of feasible clusters to estimate against.
	Clusters []*clusterv1alpha1.Cluster
	// ReplicaRequirements describes the resource requirements of a single-template workload replica.
	ReplicaRequirements *workv1alpha2.ReplicaRequirements
	// AssumedWorkloads maps cluster name to the in-flight workloads that have already
	// been assigned to that cluster but whose pods have not yet been bound to nodes.
	// The estimator deducts each cluster's assumed footprint from available capacity
	// to avoid over-commitment during back-to-back scheduling cycles.
	// +optional
	AssumedWorkloads map[string][]AssumedWorkload
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
	// AssumedWorkloads maps cluster name to the in-flight workloads that have already
	// been assigned to that cluster but whose pods have not yet been bound to nodes.
	// The estimator deducts each cluster's assumed footprint from available capacity
	// to avoid over-commitment during back-to-back scheduling cycles.
	// +optional
	AssumedWorkloads map[string][]AssumedWorkload
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
