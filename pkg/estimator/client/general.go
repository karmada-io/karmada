package client

import (
	"context"
	"fmt"
	"math"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
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
func (ge *GeneralEstimator) MaxAvailableReplicas(_ context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error) {
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

	// if the allocatableModelings from the cluster status are empty possibly due to
	// users have not set the models or the state has not been collected,
	// we consider to use another way to calculate the max replicas.
	if features.FeatureGate.Enabled(features.CustomizedClusterResourceModeling) && len(cluster.Status.ResourceSummary.AllocatableModelings) > 0 {
		num, err := getMaximumReplicasBasedOnResourceModels(cluster, replicaRequirements)
		if err == nil {
			klog.Infof("cluster %s has max available replicas: %d according to cluster resource models", cluster.GetName(), num)
			if num < maximumReplicas {
				maximumReplicas = num
			}

			return int32(maximumReplicas)
		}
		klog.Info(err.Error())
	}

	num := getMaximumReplicasBasedOnClusterSummary(resourceSummary, replicaRequirements)
	if num < maximumReplicas {
		maximumReplicas = num
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

func convertToResourceModelsMinMap(models []clusterv1alpha1.ResourceModel) map[clusterv1alpha1.ResourceName][]resource.Quantity {
	resourceModelsMinMap := make(map[clusterv1alpha1.ResourceName][]resource.Quantity)
	for _, model := range models {
		for _, resourceModelRange := range model.Ranges {
			resourceModelsMinMap[resourceModelRange.Name] = append(resourceModelsMinMap[resourceModelRange.Name], resourceModelRange.Min)
		}
	}

	return resourceModelsMinMap
}

func getNodeAvailableReplicas(modelIndex int, replicaRequirements *workv1alpha2.ReplicaRequirements, resourceModelsMinMap map[clusterv1alpha1.ResourceName][]resource.Quantity) int64 {
	var maximumReplicasOneNode int64 = math.MaxInt64
	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		availableMinBoundary := resourceModelsMinMap[clusterv1alpha1.ResourceName(key)][modelIndex]

		availableQuantity := availableMinBoundary.Value()
		if key == corev1.ResourceCPU {
			requestedQuantity = value.MilliValue()
			availableQuantity = availableMinBoundary.MilliValue()
		}

		maximumReplicasForResource := availableQuantity / requestedQuantity
		if maximumReplicasForResource < maximumReplicasOneNode {
			maximumReplicasOneNode = maximumReplicasForResource
		}
	}

	// if it is the first suitable model, we consider this case to be able to deploy a Pod.
	if maximumReplicasOneNode == 0 {
		return 1
	}
	return maximumReplicasOneNode
}

func getMaximumReplicasBasedOnClusterSummary(resourceSummary *clusterv1alpha1.ResourceSummary, replicaRequirements *workv1alpha2.ReplicaRequirements) int64 {
	var maximumReplicas int64 = math.MaxInt64
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

	return maximumReplicas
}

func getMaximumReplicasBasedOnResourceModels(cluster *clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) (int64, error) {
	resourceModelsMinMap := convertToResourceModelsMinMap(cluster.Spec.ResourceModels)

	minCompliantModelIndex := 0
	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		quantityArray, ok := resourceModelsMinMap[clusterv1alpha1.ResourceName(key)]
		if !ok {
			return -1, fmt.Errorf("resource model is inapplicable as missing resource: %s", string(key))
		}

		for index, minValue := range quantityArray {
			// Suppose there is the following resource model:
			// Model1: cpu [1C,2C)
			// Model2: cpu [2C,3C)
			// if pod cpu request is 1.5C, we regard the nodes in model1 as meeting the requirements of the Pod.
			// Suppose there is the following resource model:
			// Model1: cpu [1C,2C), memory [1Gi,2Gi)
			// Model2: cpu [2C,3C), memory [2Gi,3Gi)
			// if pod cpu request is 1.5C and memory request is 2.5Gi
			// We regard the node of model1 as not meeting the requirements, and the nodes of model2 and later as meeting the requirements.
			if minValue.Cmp(value) > 0 {
				// Since the 'min' value of the first model is always 0, hit here
				// the index should be >=1, so it's safe to use 'index-1' here.
				if index-1 > minCompliantModelIndex {
					minCompliantModelIndex = index - 1
				}
				break
			}

			if index == len(quantityArray)-1 {
				minCompliantModelIndex = index
			}
		}
	}

	var maximumReplicasForResource int64
	for i := minCompliantModelIndex; i < len(cluster.Spec.ResourceModels); i++ {
		if cluster.Status.ResourceSummary.AllocatableModelings[i].Count == 0 {
			continue
		}
		maximumReplicasForResource += int64(cluster.Status.ResourceSummary.AllocatableModelings[i].Count) * getNodeAvailableReplicas(i, replicaRequirements, resourceModelsMinMap)
	}

	return maximumReplicasForResource, nil
}
