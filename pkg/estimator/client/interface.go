package client

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

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
