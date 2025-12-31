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

package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(spec *workv1alpha2.ResourceBindingSpec) []string {
	var clusterNames []string
	for _, targetCluster := range spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// IsBindingReplicasChanged will check if the sum of replicas is different from the replicas of object
func IsBindingReplicasChanged(bindingSpec *workv1alpha2.ResourceBindingSpec, strategy *policyv1alpha1.ReplicaSchedulingStrategy) bool {
	if strategy == nil {
		return false
	}

	bindingReplicas := GetTotalBindingReplicas(bindingSpec)

	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		for _, targetCluster := range bindingSpec.Clusters {
			if targetCluster.Replicas != bindingReplicas {
				return true
			}
		}
		return false
	}
	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		replicasSum := GetSumOfReplicas(bindingSpec.Clusters)
		return replicasSum != bindingReplicas
	}
	return false
}

// GetTotalBindingReplicas will get the total replicas for a given resourcebinding
func GetTotalBindingReplicas(bindingSpec *workv1alpha2.ResourceBindingSpec) int32 {
	if len(bindingSpec.Components) > 0 {
		return GetSumOfReplicasForComponents(bindingSpec.Components)
	}
	return bindingSpec.Replicas
}

// GetSumofReplicasForComponents will get the sum of replicas for multi-component resources
func GetSumOfReplicasForComponents(components []workv1alpha2.Component) int32 {
	replicasSum := int32(0)
	for _, component := range components {
		replicasSum += component.Replicas
	}
	return replicasSum
}

// GetSumOfReplicas will get the sum of replicas in target clusters
func GetSumOfReplicas(clusters []workv1alpha2.TargetCluster) int32 {
	replicasSum := int32(0)
	for i := range clusters {
		replicasSum += clusters[i].Replicas
	}
	return replicasSum
}

// ConvertToClusterNames will convert a cluster slice to clusterName's sets.String
func ConvertToClusterNames(clusters []workv1alpha2.TargetCluster) sets.Set[string] {
	clusterNames := sets.New[string]()
	for _, cluster := range clusters {
		clusterNames.Insert(cluster.Name)
	}

	return clusterNames
}

// MergeTargetClusters will merge the replicas in two TargetCluster
func MergeTargetClusters(oldCluster, newCluster []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	switch {
	case len(oldCluster) == 0:
		return newCluster
	case len(newCluster) == 0:
		return oldCluster
	}
	// oldMap is a map of the result for the old replicas so that it can be merged with the new result easily
	oldMap := make(map[string]int32)
	for _, cluster := range oldCluster {
		oldMap[cluster.Name] = cluster.Replicas
	}
	// merge the new replicas and the data of old replicas
	for i, cluster := range newCluster {
		value, ok := oldMap[cluster.Name]
		if ok {
			newCluster[i].Replicas = cluster.Replicas + value
			delete(oldMap, cluster.Name)
		}
	}
	for key, value := range oldMap {
		newCluster = append(newCluster, workv1alpha2.TargetCluster{Name: key, Replicas: value})
	}
	return newCluster
}

// RescheduleRequired judges whether reschedule is required.
func RescheduleRequired(rescheduleTriggeredAt, lastScheduledTime *metav1.Time) bool {
	if rescheduleTriggeredAt == nil {
		return false
	}
	// lastScheduledTime is nil means first schedule haven't finished or yet keep failing, just wait for this schedule.
	if lastScheduledTime == nil {
		return false
	}
	return rescheduleTriggeredAt.After(lastScheduledTime.Time)
}

// MergePolicySuspension merges the suspension configuration from policy to binding suspension.
func MergePolicySuspension(bindingSuspension *workv1alpha2.Suspension, policySuspension *policyv1alpha1.Suspension) *workv1alpha2.Suspension {
	if policySuspension != nil {
		if bindingSuspension == nil {
			bindingSuspension = &workv1alpha2.Suspension{}
		}
		bindingSuspension.Suspension = *policySuspension
		return bindingSuspension
	}
	// policySuspension is nil, clean up binding's suspension part.
	if bindingSuspension == nil {
		return nil
	}
	bindingSuspension.Suspension = policyv1alpha1.Suspension{}
	if bindingSuspension.Scheduling == nil {
		return nil
	}
	return bindingSuspension
}
