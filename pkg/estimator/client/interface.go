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

// ReplicaEstimator is an estimator which estimates the maximum replicas that can be applied to the target cluster.
type ReplicaEstimator interface {
	MaxAvailableReplicas(ctx context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error)
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
