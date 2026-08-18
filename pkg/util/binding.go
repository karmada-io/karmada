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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
)

const componentRequirementsHashPrefix = "v1:sha256:"

type componentRequirementsSnapshot struct {
	Name                string                                     `json:"name"`
	ReplicaRequirements *workv1alpha2.ComponentReplicaRequirements `json:"replicaRequirements,omitempty"`
}

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

	// For component-based workloads, trigger rescheduling when clusters are empty (e.g., after eviction).
	// This is a temporary fix to ensure cluster failover works correctly.
	// Limitation: This only handles the failover scenario where clusters are cleared.
	// It does not detect component replica changes (e.g., scale up/down) or replica swaps between components.
	// A complete solution requires changing how scheduling results are stored to support multi-template workloads,
	// likely by extending TargetCluster to include per-component replica information.
	// The comprehensive solution is tracked by: https://github.com/karmada-io/karmada/issues/6998
	if features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) && len(bindingSpec.Components) > 0 {
		if len(bindingSpec.Clusters) == 0 {
			return true
		}
	}

	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		for _, targetCluster := range bindingSpec.Clusters {
			if targetCluster.Replicas != bindingSpec.Replicas {
				return true
			}
		}
		return false
	}
	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		replicasSum := GetSumOfReplicas(bindingSpec.Clusters)
		return replicasSum != bindingSpec.Replicas
	}
	return false
}

// ComponentScaleDirection classifies a name-keyed desired/accepted replica transition.
type ComponentScaleDirection int

const (
	// ComponentScaleUnknown means the desired and accepted snapshots cannot be compared safely.
	ComponentScaleUnknown ComponentScaleDirection = iota
	// ComponentScaleEqual means every desired component matches its accepted replica count.
	ComponentScaleEqual
	// ComponentScaleUp means at least one component grows and none shrink.
	ComponentScaleUp
	// ComponentScaleDown means at least one component shrinks and none grow.
	ComponentScaleDown
	// ComponentScaleMixed means some components grow while others shrink.
	ComponentScaleMixed
)

// IsMultiTemplateSchedulingApplicable reports whether component scheduling can
// currently produce one complete assignment on exactly one target cluster.
func IsMultiTemplateSchedulingApplicable(bindingSpec *workv1alpha2.ResourceBindingSpec) bool {
	if bindingSpec == nil || len(bindingSpec.Components) <= 1 || bindingSpec.Placement == nil ||
		bindingSpec.Placement.ClusterAffinities != nil {
		return false
	}
	for i := range bindingSpec.Placement.SpreadConstraints {
		constraint := bindingSpec.Placement.SpreadConstraints[i]
		if constraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster && constraint.MinGroups == 1 && constraint.MaxGroups == 1 {
			return true
		}
	}
	return false
}

// ClassifyComponentReplicaTransition compares complete desired and accepted
// component snapshots. Invalid, incomplete, or differently named snapshots are unknown.
func ClassifyComponentReplicaTransition(desired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) ComponentScaleDirection {
	if len(desired) == 0 || len(desired) != len(accepted) {
		return ComponentScaleUnknown
	}

	acceptedReplicas, valid := acceptedComponentReplicasByName(accepted)
	if !valid {
		return ComponentScaleUnknown
	}

	seen := make(map[string]struct{}, len(desired))
	var scaleUp, scaleDown bool
	for i := range desired {
		component := desired[i]
		if component.Name == "" {
			return ComponentScaleUnknown
		}
		if _, exists := seen[component.Name]; exists {
			return ComponentScaleUnknown
		}
		seen[component.Name] = struct{}{}
		replicas, exists := acceptedReplicas[component.Name]
		if !exists {
			return ComponentScaleUnknown
		}
		switch {
		case component.Replicas > replicas:
			scaleUp = true
		case component.Replicas < replicas:
			scaleDown = true
		}
	}

	switch {
	case scaleUp && scaleDown:
		return ComponentScaleMixed
	case scaleUp:
		return ComponentScaleUp
	case scaleDown:
		return ComponentScaleDown
	default:
		return ComponentScaleEqual
	}
}

func acceptedComponentReplicasByName(accepted []workv1alpha2.TargetComponent) (map[string]int32, bool) {
	replicasByName := make(map[string]int32, len(accepted))
	for i := range accepted {
		if accepted[i].Name == "" {
			return nil, false
		}
		if _, exists := replicasByName[accepted[i].Name]; exists {
			return nil, false
		}
		replicasByName[accepted[i].Name] = accepted[i].Replicas
	}
	return replicasByName, true
}

// IsBindingComponentScaleSupported reports whether the persisted result can be
// updated in place using the component scale planner.
func IsBindingComponentScaleSupported(bindingSpec *workv1alpha2.ResourceBindingSpec) bool {
	if bindingSpec == nil || !features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) ||
		!IsMultiTemplateSchedulingApplicable(bindingSpec) || len(bindingSpec.Clusters) != 1 ||
		bindingSpec.Placement.ClusterAffinities != nil {
		return false
	}
	direction := ClassifyComponentReplicaTransition(bindingSpec.Components, bindingSpec.Clusters[0].Components)
	return direction == ComponentScaleUp || direction == ComponentScaleDown
}

// HasBindingComponentResult reports whether any owned target carries a component assignment.
func HasBindingComponentResult(bindingSpec *workv1alpha2.ResourceBindingSpec) bool {
	if bindingSpec == nil {
		return false
	}
	for i := range bindingSpec.Clusters {
		if len(bindingSpec.Clusters[i].Components) > 0 {
			return true
		}
	}
	return false
}

// IsBindingComponentResultChanged reports whether the persisted component result
// differs from the desired replica snapshot.
func IsBindingComponentResultChanged(bindingSpec *workv1alpha2.ResourceBindingSpec) bool {
	if bindingSpec == nil || !HasBindingComponentResult(bindingSpec) || len(bindingSpec.Clusters) != 1 {
		return false
	}
	return ClassifyComponentReplicaTransition(bindingSpec.Components, bindingSpec.Clusters[0].Components) != ComponentScaleEqual
}

// IsBindingComponentsAccepted reports whether the persisted single-cluster
// component result is a complete snapshot of the current desired replicas.
func IsBindingComponentsAccepted(bindingSpec *workv1alpha2.ResourceBindingSpec) bool {
	return bindingSpec != nil && features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) &&
		IsMultiTemplateSchedulingApplicable(bindingSpec) && len(bindingSpec.Clusters) == 1 &&
		ClassifyComponentReplicaTransition(bindingSpec.Components, bindingSpec.Clusters[0].Components) == ComponentScaleEqual
}

// GenerateComponentRequirementsHash returns a stable identity for the name-keyed
// replica requirements used by component scheduling. Replica counts are excluded
// because they are persisted separately in TargetComponent.
func GenerateComponentRequirementsHash(components []workv1alpha2.Component) (string, error) {
	snapshot := make([]componentRequirementsSnapshot, len(components))
	for i := range components {
		snapshot[i] = componentRequirementsSnapshot{
			Name:                components[i].Name,
			ReplicaRequirements: components[i].ReplicaRequirements,
		}
	}
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].Name < snapshot[j].Name
	})

	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return componentRequirementsHashPrefix + hex.EncodeToString(sum[:]), nil
}

// IsBindingComponentRequirementsHashMissing reports whether no accepted
// component requirements identity has been persisted yet.
func IsBindingComponentRequirementsHashMissing(annotations map[string]string) bool {
	return annotations == nil || annotations[AcceptedComponentRequirementsHashAnnotation] == ""
}

// IsBindingComponentRequirementsHashMatched reports whether the current component
// requirements are the same requirements accepted with the scheduling result.
func IsBindingComponentRequirementsHashMatched(components []workv1alpha2.Component, annotations map[string]string) bool {
	if IsBindingComponentRequirementsHashMissing(annotations) {
		return false
	}
	hash, err := GenerateComponentRequirementsHash(components)
	return err == nil && annotations[AcceptedComponentRequirementsHashAnnotation] == hash
}

// IsBindingComponentRequirementsHashMismatch reports whether a persisted accepted
// requirements identity differs from the current component requirements.
func IsBindingComponentRequirementsHashMismatch(components []workv1alpha2.Component, annotations map[string]string) bool {
	return !IsBindingComponentRequirementsHashMissing(annotations) &&
		!IsBindingComponentRequirementsHashMatched(components, annotations)
}

// IsBindingComponentResultPending reports whether Work delivery must wait for a
// complete result that matches the desired replicas and accepted requirements.
func IsBindingComponentResultPending(bindingSpec *workv1alpha2.ResourceBindingSpec, annotations map[string]string) bool {
	if bindingSpec == nil || !features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) {
		return false
	}

	hasResult := HasBindingComponentResult(bindingSpec)
	if !hasResult {
		// Ordinary multi-component placements do not produce a component result.
		return IsMultiTemplateSchedulingApplicable(bindingSpec)
	}
	// Once an accepted component result exists, transitions out of the supported
	// shape stay frozen until the scheduler commits a replacement result.
	if !IsMultiTemplateSchedulingApplicable(bindingSpec) || len(bindingSpec.Clusters) != 1 {
		return true
	}
	if ClassifyComponentReplicaTransition(bindingSpec.Components, bindingSpec.Clusters[0].Components) != ComponentScaleEqual {
		return true
	}
	return !IsBindingComponentRequirementsHashMatched(bindingSpec.Components, annotations)
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
