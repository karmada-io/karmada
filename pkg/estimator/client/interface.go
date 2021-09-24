package client

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// UnauthenticReplica is special replica number returned by estimator in case of estimator can't calculate the available
// replicas.
// The scheduler should discard the estimator's result and back-off to rely on other estimator's result.
const UnauthenticReplica = -1

var (
	replicaEstimators = map[string]ReplicaEstimator{}
)

// ReplicaEstimator is an estimator which estimates the maximum replicas that can be applied to the target cluster.
type ReplicaEstimator interface {
	MaxAvailableReplicas(clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha1.ReplicaRequirements) ([]workv1alpha1.TargetCluster, error)
}

// GetReplicaEstimators returns all replica estimators.
func GetReplicaEstimators() map[string]ReplicaEstimator {
	return replicaEstimators
}
