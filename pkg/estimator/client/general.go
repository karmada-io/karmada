package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// GeneralEstimator is the default replica estimator.
func init() {
	replicaEstimators["general-estimator"] = NewGeneralEstimator()
}

// GeneralEstimator is a normal estimator in terms of cluster ResourceSummary.
type GeneralEstimator struct{}

// NewGeneralEstimator builds a new GeneralEstimator.
func NewGeneralEstimator() *GeneralEstimator {
	return &GeneralEstimator{}
}

// MaxAvailableReplicas estimates the maximum replicas that can be applied to the target cluster by cluster ResourceSummary.
func (ge *GeneralEstimator) MaxAvailableReplicas(ctx context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error) {
	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		maxReplicas := ge.maxAvailableReplicas(cluster, replicaRequirements)
		availableTargetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: maxReplicas}
	}
	return availableTargetClusters, nil
}

func (ge *GeneralEstimator) maxAvailableReplicas(cluster *clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) int32 {
	resourceSummary := cluster.Status.ResourceSummary
	if resourceSummary == nil {
		return 0
	}

	maximumReplicas := getAllowedPodNumber(resourceSummary)
	if maximumReplicas <= 0 {
		return 0
	}

	if replicaRequirements == nil {
		return int32(maximumReplicas)
	}

	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		// calculates available resource quantity
		// available = allocatable - allocated - allocating
		allocatable, ok := resourceSummary.Allocatable[key]
		if !ok {
			return 0
		}
		allocated, ok := resourceSummary.Allocated[key]
		if ok {
			allocatable.Sub(allocated)
		}
		allocating, ok := resourceSummary.Allocating[key]
		if ok {
			allocatable.Sub(allocating)
		}
		availableQuantity := allocatable.Value()
		// short path: no more resource left.
		if availableQuantity <= 0 {
			return 0
		}

		if key == corev1.ResourceCPU {
			requestedQuantity = value.MilliValue()
			availableQuantity = allocatable.MilliValue()
		}

		maximumReplicasForResource := availableQuantity / requestedQuantity
		if maximumReplicasForResource < maximumReplicas {
			maximumReplicas = maximumReplicasForResource
		}
	}

	return int32(maximumReplicas)
}

func getAllowedPodNumber(resourceSummary *clusterv1alpha1.ResourceSummary) int64 {
	var allocatable, allocated, allocating int64
	if resourceSummary.Allocatable != nil {
		allocatable = resourceSummary.Allocatable.Pods().Value()
	}
	if resourceSummary.Allocated != nil {
		allocated = resourceSummary.Allocated.Pods().Value()
	}
	if resourceSummary.Allocating != nil {
		allocating = resourceSummary.Allocating.Pods().Value()
	}
	allowedPodNumber := allocatable - allocated - allocating
	// When too many pods have been created, scheduling will fail so that the allocating pods number may be huge.
	// If allowedPodNumber is less than or equal to 0, we don't allow more pods to be created.
	if allowedPodNumber <= 0 {
		return 0
	}
	return allowedPodNumber
}
