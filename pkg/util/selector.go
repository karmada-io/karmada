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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ImplicitPriority describes the extent to which a ResourceSelector or a set of
// ResourceSelectors match resources.
type ImplicitPriority int

const (
	// PriorityMisMatch means the ResourceSelector does not match the resource.
	PriorityMisMatch ImplicitPriority = iota
	// PriorityMatchAll means the ResourceSelector whose Name and LabelSelector is empty
	// matches the resource.
	PriorityMatchAll
	// PriorityMatchLabelSelector means the LabelSelector of ResourceSelector matches the resource.
	PriorityMatchLabelSelector
	// PriorityMatchName means the Name of ResourceSelector matches the resource.
	PriorityMatchName
)

// ResourceMatches tells if the specific resource matches the selector.
func ResourceMatches(resource *unstructured.Unstructured, rs policyv1alpha1.ResourceSelector) bool {
	return ResourceSelectorPriority(resource, rs) > PriorityMisMatch
}

// ResourceSelectorPriority tells the priority between the specific resource and the selector.
func ResourceSelectorPriority(resource *unstructured.Unstructured, rs policyv1alpha1.ResourceSelector) ImplicitPriority {
	if resource.GetAPIVersion() != rs.APIVersion ||
		resource.GetKind() != rs.Kind ||
		(len(rs.Namespace) > 0 && resource.GetNamespace() != rs.Namespace) {
		return PriorityMisMatch
	}

	// match rules:
	// case ResourceSelector.name   ResourceSelector.labelSelector   Rule
	// 1    not-empty               not-empty                        match name only and ignore selector
	// 2    not-empty               empty                            match name only
	// 3    empty                   not-empty                        match selector only
	// 4    empty                   empty                            match all

	// case 1, 2: name not empty, don't need to consult selector.
	if len(rs.Name) > 0 {
		if rs.Name == resource.GetName() {
			return PriorityMatchName
		}
		return PriorityMisMatch
	}

	// case 4: short path, both name and selector empty, matches all
	if rs.LabelSelector == nil {
		return PriorityMatchAll
	}

	// case 3: matches with selector
	var s labels.Selector
	var err error
	if s, err = metav1.LabelSelectorAsSelector(rs.LabelSelector); err != nil {
		// should not happen because all resource selector should be fully validated by webhook.
		return PriorityMisMatch
	}

	if s.Matches(labels.Set(resource.GetLabels())) {
		return PriorityMatchLabelSelector
	}
	return PriorityMisMatch
}

// ClusterMatches tells if specific cluster matches the affinity.
func ClusterMatches(cluster *clusterv1alpha1.Cluster, affinity policyv1alpha1.ClusterAffinity) bool {
	for _, clusterName := range affinity.ExcludeClusters {
		if clusterName == cluster.Name {
			return false
		}
	}

	// match rules:
	// case LabelSelector   ClusterNames   	FieldSelector       Rule
	// 1    not-empty        not-empty       not-empty          match selector, name and field
	// 2    not-empty        empty           empty              match selector only
	// 3    not-empty        not-empty       empty              match selector, name
	// 4    not-empty        empty       	 not-empty          match selector, filed
	// 5    empty            not-empty       not-empty          match name, filed
	// 6    empty            not-empty       empty              match name only
	// 7    empty            empty       	 not-empty          match field only
	// 8    empty            empty           empty              match all

	if affinity.LabelSelector != nil {
		var s labels.Selector
		var err error
		if s, err = metav1.LabelSelectorAsSelector(affinity.LabelSelector); err != nil {
			return false
		}

		if !s.Matches(labels.Set(cluster.GetLabels())) {
			return false
		}
	}

	if affinity.FieldSelector != nil {
		var clusterFieldsMatchExpressions []corev1.NodeSelectorRequirement
		for i := range affinity.FieldSelector.MatchExpressions {
			matchExpression := &affinity.FieldSelector.MatchExpressions[i]
			if matchExpression.Key != ZoneField {
				clusterFieldsMatchExpressions = append(clusterFieldsMatchExpressions, *matchExpression)
				continue
			}

			// First, match zones field.
			if !matchZones(matchExpression, cluster.Spec.Zones) {
				return false
			}
		}

		if len(clusterFieldsMatchExpressions) > 0 {
			// Second, match other fields.
			var matchFields labels.Selector
			var errs []error
			if matchFields, errs = lifted.NodeSelectorRequirementsAsSelector(clusterFieldsMatchExpressions); errs != nil {
				return false
			}
			clusterFields := extractClusterFields(cluster)
			if matchFields != nil && !matchFields.Matches(clusterFields) {
				return false
			}
		}
	}

	return ClusterNamesMatches(cluster, affinity.ClusterNames)
}

// ClusterNamesMatches tells if specific cluster matches the clusterNames affinity.
func ClusterNamesMatches(cluster *clusterv1alpha1.Cluster, clusterNames []string) bool {
	if len(clusterNames) == 0 {
		return true
	}

	for _, clusterName := range clusterNames {
		if clusterName == cluster.Name {
			return true
		}
	}
	return false
}

// ResourceMatchSelectors tells if the specific resource matches the selectors.
func ResourceMatchSelectors(resource *unstructured.Unstructured, selectors ...policyv1alpha1.ResourceSelector) bool {
	for _, rs := range selectors {
		if ResourceMatches(resource, rs) {
			return true
		}
	}
	return false
}

// ResourceMatchSelectorsPriority returns the highest priority between specific resource and the selectors.
func ResourceMatchSelectorsPriority(resource *unstructured.Unstructured, selectors ...policyv1alpha1.ResourceSelector) ImplicitPriority {
	var priority ImplicitPriority
	for _, rs := range selectors {
		if p := ResourceSelectorPriority(resource, rs); p > priority {
			priority = p
		}
	}
	return priority
}

func extractClusterFields(cluster *clusterv1alpha1.Cluster) labels.Set {
	clusterFieldsMap := make(labels.Set)

	if cluster.Spec.Provider != "" {
		clusterFieldsMap[ProviderField] = cluster.Spec.Provider
	}

	if cluster.Spec.Region != "" {
		clusterFieldsMap[RegionField] = cluster.Spec.Region
	}

	return clusterFieldsMap
}

// matchZones checks if zoneMatchExpression can match zones and returns true if it matches.
// For unknown operators, matchZones always returns false.
// The matching rules are as follows:
// 1. When the operator is "In", zoneMatchExpression must contain all zones, otherwise it doesn't match.
// 2. When the operator is "NotIn", zoneMatchExpression mustn't contain any one of zones, otherwise it doesn't match.
// 3. When the operator is "Exists", zones mustn't be empty, otherwise it doesn't match.
// 4. When the operator is "DoesNotExist", zones must be empty, otherwise it doesn't match.
func matchZones(zoneMatchExpression *corev1.NodeSelectorRequirement, zones []string) bool {
	switch zoneMatchExpression.Operator {
	case corev1.NodeSelectorOpIn:
		if len(zones) == 0 {
			return false
		}
		for _, zone := range zones {
			if !slices.Contains(zoneMatchExpression.Values, zone) {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpNotIn:
		for _, zone := range zones {
			if slices.Contains(zoneMatchExpression.Values, zone) {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpExists:
		return len(zones) > 0
	case corev1.NodeSelectorOpDoesNotExist:
		return len(zones) == 0
	default:
		klog.V(5).Infof("Unsupported %q operator for zones requirement", zoneMatchExpression.Operator)
		return false
	}
}
