package scheduler

import (
	"encoding/json"
	"reflect"

	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func placementChanged(
	placement policyv1alpha1.Placement,
	appliedPlacementStr string,
	schedulerObservingAffinityName string,
) bool {
	if appliedPlacementStr == "" {
		return true
	}

	appliedPlacement := policyv1alpha1.Placement{}
	err := json.Unmarshal([]byte(appliedPlacementStr), &appliedPlacement)
	if err != nil {
		klog.Errorf("Failed to unmarshal applied placement string: %v", err)
		return false
	}

	// first check: entire placement does not change
	if reflect.DeepEqual(placement, appliedPlacement) {
		return false
	}

	// second check: except for ClusterAffinities, the placement has changed
	if !reflect.DeepEqual(placement.ClusterAffinity, appliedPlacement.ClusterAffinity) ||
		!reflect.DeepEqual(placement.ClusterTolerations, appliedPlacement.ClusterTolerations) ||
		!reflect.DeepEqual(placement.SpreadConstraints, appliedPlacement.SpreadConstraints) ||
		!reflect.DeepEqual(placement.ReplicaScheduling, appliedPlacement.ReplicaScheduling) {
		return true
	}

	// third check: check weather ClusterAffinities has changed
	return clusterAffinitiesChanged(placement.ClusterAffinities, appliedPlacement.ClusterAffinities, schedulerObservingAffinityName)
}

func clusterAffinitiesChanged(
	clusterAffinities, appliedClusterAffinities []policyv1alpha1.ClusterAffinityTerm,
	schedulerObservingAffinityName string,
) bool {
	if schedulerObservingAffinityName == "" {
		return true
	}

	var clusterAffinityTerm, appliedClusterAffinityTerm *policyv1alpha1.ClusterAffinityTerm
	for index := range clusterAffinities {
		if clusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			clusterAffinityTerm = &clusterAffinities[index]
			break
		}
	}
	for index := range appliedClusterAffinities {
		if appliedClusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			appliedClusterAffinityTerm = &appliedClusterAffinities[index]
			break
		}
	}
	if clusterAffinityTerm == nil || appliedClusterAffinityTerm == nil {
		return true
	}
	if !reflect.DeepEqual(&clusterAffinityTerm, &appliedClusterAffinityTerm) {
		return true
	}
	return false
}
