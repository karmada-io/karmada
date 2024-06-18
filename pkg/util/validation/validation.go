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

package validation

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// LabelValueMaxLength is a label's max length
const LabelValueMaxLength int = 63

// ValidatePropagationSpec validates a PropagationSpec before creation or update.
func ValidatePropagationSpec(spec policyv1alpha1.PropagationSpec) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, ValidatePlacement(spec.Placement, field.NewPath("spec").Child("placement"))...)
	if spec.Failover != nil && spec.Failover.Application != nil && !spec.PropagateDeps {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("propagateDeps"), spec.PropagateDeps, "application failover is set, propagateDeps must be true"))
	}
	allErrs = append(allErrs, ValidateFailover(spec.Failover, field.NewPath("spec").Child("failover"))...)
	allErrs = append(allErrs, validateResourceSelectorsIfPreemptionEnabled(spec, field.NewPath("spec").Child("resourceSelectors"))...)
	return allErrs
}

// validateResourceSelectorsIfPreemptionEnabled validates ResourceSelectors if Preemption is Always.
func validateResourceSelectorsIfPreemptionEnabled(spec policyv1alpha1.PropagationSpec, fldPath *field.Path) field.ErrorList {
	if spec.Preemption != policyv1alpha1.PreemptAlways {
		return nil
	}

	var allErrs field.ErrorList
	for index, resourceSelector := range spec.ResourceSelectors {
		if len(resourceSelector.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index).Child("name"), resourceSelector.Name, "name can not be empty if preemption is Always, the empty name may cause unexpected resources preemption"))
		}
	}
	return allErrs
}

// ValidatePlacement validates a placement before creation or update.
func ValidatePlacement(placement policyv1alpha1.Placement, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if placement.ClusterAffinity != nil && placement.ClusterAffinities != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, placement, "clusterAffinities can not co-exist with clusterAffinity"))
	}

	allErrs = append(allErrs, ValidateClusterAffinity(placement.ClusterAffinity, fldPath.Child("clusterAffinity"))...)
	allErrs = append(allErrs, ValidateClusterAffinities(placement.ClusterAffinities, fldPath.Child("clusterAffinities"))...)
	allErrs = append(allErrs, ValidateSpreadConstraint(placement.SpreadConstraints, fldPath.Child("spreadConstraints"))...)
	return allErrs
}

// ValidateClusterAffinity validates a clusterAffinity before creation or update.
func ValidateClusterAffinity(affinity *policyv1alpha1.ClusterAffinity, fldPath *field.Path) field.ErrorList {
	if affinity == nil {
		return nil
	}

	var allErrs field.ErrorList
	err := ValidatePolicyFieldSelector(affinity.FieldSelector)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("fieldSelector"), affinity.FieldSelector, err.Error()))
	}
	return allErrs
}

// ValidateClusterAffinities validates clusterAffinities before creation or update.
func ValidateClusterAffinities(affinities []policyv1alpha1.ClusterAffinityTerm, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	affinityNames := make(map[string]bool)
	for index := range affinities {
		for _, err := range validation.IsQualifiedName(affinities[index].AffinityName) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), affinities[index].AffinityName, err))
		}
		if _, exist := affinityNames[affinities[index].AffinityName]; exist {
			allErrs = append(allErrs, field.Invalid(fldPath, affinities, "each affinity term in a policy must have a unique name"))
		} else {
			affinityNames[affinities[index].AffinityName] = true
		}

		allErrs = append(allErrs, ValidateClusterAffinity(&affinities[index].ClusterAffinity, fldPath.Index(index))...)
	}
	return allErrs
}

// ValidatePolicyFieldSelector tests if the fieldSelector of propagation policy or override policy is valid.
func ValidatePolicyFieldSelector(fieldSelector *policyv1alpha1.FieldSelector) error {
	if fieldSelector == nil {
		return nil
	}

	for _, matchExpression := range fieldSelector.MatchExpressions {
		switch matchExpression.Key {
		case util.ProviderField, util.RegionField, util.ZoneField:
		default:
			return fmt.Errorf("unsupported key %q, must be provider, region, or zone", matchExpression.Key)
		}

		switch matchExpression.Operator {
		case corev1.NodeSelectorOpIn, corev1.NodeSelectorOpNotIn:
		default:
			return fmt.Errorf("unsupported operator %q, must be In or NotIn", matchExpression.Operator)
		}
	}

	return nil
}

// ValidateSpreadConstraint tests if the constraints is valid.
func ValidateSpreadConstraint(spreadConstraints []policyv1alpha1.SpreadConstraint, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	spreadByFieldsWithErrorMark := make(map[policyv1alpha1.SpreadFieldValue]*bool)
	for index, constraint := range spreadConstraints {
		// SpreadByField and SpreadByLabel should not co-exist
		if len(constraint.SpreadByField) > 0 && len(constraint.SpreadByLabel) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), constraint, "spreadByLabel should not co-exist with spreadByField"))
		}

		// If MinGroups provided, it should not be lower than 0.
		if constraint.MinGroups < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), constraint, "minGroups lower than 0 is not allowed"))
		}

		// If MaxGroups provided, it should not be lower than 0.
		if constraint.MaxGroups < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), constraint, "maxGroups lower than 0 is not allowed"))
		}

		// If MaxGroups provided, it should greater or equal than MinGroups.
		if constraint.MaxGroups > 0 && constraint.MaxGroups < constraint.MinGroups {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), constraint, "maxGroups lower than minGroups is not allowed"))
		}

		if len(constraint.SpreadByField) > 0 {
			marked := spreadByFieldsWithErrorMark[constraint.SpreadByField]
			if !ptr.Deref[bool](marked, true) {
				allErrs = append(allErrs, field.Invalid(fldPath, spreadConstraints, fmt.Sprintf("multiple %s spread constraints are not allowed", constraint.SpreadByField)))
				*marked = true
			}
			if marked == nil {
				spreadByFieldsWithErrorMark[constraint.SpreadByField] = ptr.To[bool](false)
			}
		}
	}

	if len(spreadByFieldsWithErrorMark) > 0 {
		// If one of spread constraints are using 'SpreadByField', the 'SpreadByFieldCluster' must be included.
		// For example, when using 'SpreadByFieldRegion' to specify region groups, at the meantime, you must use
		// 'SpreadByFieldCluster' to specify how many clusters should be selected.
		if _, ok := spreadByFieldsWithErrorMark[policyv1alpha1.SpreadByFieldCluster]; !ok {
			allErrs = append(allErrs, field.Invalid(fldPath, spreadConstraints, "the cluster spread constraint must be enabled in one of the constraints in case of SpreadByField is enabled"))
		}
	}

	return allErrs
}

// ValidateFailover validates that the failoverBehavior is correctly defined.
func ValidateFailover(failoverBehavior *policyv1alpha1.FailoverBehavior, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if failoverBehavior == nil {
		return nil
	}

	allErrs = append(allErrs, ValidateApplicationFailover(failoverBehavior.Application, fldPath.Child("application"))...)
	return allErrs
}

// ValidateApplicationFailover validates that the application failover is correctly defined.
func ValidateApplicationFailover(applicationFailoverBehavior *policyv1alpha1.ApplicationFailoverBehavior, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if applicationFailoverBehavior == nil {
		return nil
	}

	if *applicationFailoverBehavior.DecisionConditions.TolerationSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("decisionConditions").Child("tolerationSeconds"), *applicationFailoverBehavior.DecisionConditions.TolerationSeconds, "must be greater than or equal to 0"))
	}

	if applicationFailoverBehavior.PurgeMode != policyv1alpha1.Graciously && applicationFailoverBehavior.GracePeriodSeconds != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gracePeriodSeconds"), *applicationFailoverBehavior.GracePeriodSeconds, "only takes effect when purgeMode is graciously"))
	}

	if applicationFailoverBehavior.PurgeMode == policyv1alpha1.Graciously && applicationFailoverBehavior.GracePeriodSeconds == nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gracePeriodSeconds"), applicationFailoverBehavior.GracePeriodSeconds, "should not be empty when purgeMode is graciously"))
	}

	if applicationFailoverBehavior.GracePeriodSeconds != nil && *applicationFailoverBehavior.GracePeriodSeconds <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gracePeriodSeconds"), *applicationFailoverBehavior.GracePeriodSeconds, "must be greater than 0"))
	}

	return allErrs
}

// ValidateOverrideSpec validates that the overrider specification is correctly defined.
func ValidateOverrideSpec(overrideSpec *policyv1alpha1.OverrideSpec) field.ErrorList {
	var allErrs field.ErrorList
	if overrideSpec == nil {
		return nil
	}
	specPath := field.NewPath("spec")
	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	if overrideSpec.TargetCluster != nil {
		allErrs = append(allErrs, ValidateClusterAffinity(overrideSpec.TargetCluster, specPath.Child("targetCluster"))...)
	}
	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	if overrideSpec.TargetCluster != nil && overrideSpec.OverrideRules != nil {
		allErrs = append(allErrs, field.Invalid(specPath.Child("targetCluster"), overrideSpec.TargetCluster, "overrideRules and targetCluster can't co-exist"))
	}
	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	if !emptyOverrides(overrideSpec.Overriders) && overrideSpec.OverrideRules != nil {
		allErrs = append(allErrs, field.Invalid(specPath.Child("overriders"), overrideSpec.Overriders, "overrideRules and overriders can't co-exist"))
	}
	allErrs = append(allErrs, ValidateOverrideRules(overrideSpec.OverrideRules, specPath)...)
	return allErrs
}

// emptyOverrides check if the overriders of override policy is empty.
func emptyOverrides(overriders policyv1alpha1.Overriders) bool {
	if len(overriders.Plaintext) != 0 {
		return false
	}
	if len(overriders.ImageOverrider) != 0 {
		return false
	}
	if len(overriders.CommandOverrider) != 0 {
		return false
	}
	if len(overriders.ArgsOverrider) != 0 {
		return false
	}
	if len(overriders.LabelsOverrider) != 0 {
		return false
	}
	if len(overriders.AnnotationsOverrider) != 0 {
		return false
	}
	return true
}

// ValidateOverrideRules validates the overrideRules of override policy.
func ValidateOverrideRules(overrideRules []policyv1alpha1.RuleWithCluster, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for overrideRuleIndex, rule := range overrideRules {
		rulePath := fldPath.Child("overrideRules").Index(overrideRuleIndex)

		// validates provided annotations.
		for annotationIndex, annotation := range rule.Overriders.AnnotationsOverrider {
			annotationPath := rulePath.Child("overriders").Child("annotationsOverrider").Index(annotationIndex)
			allErrs = append(allErrs, apivalidation.ValidateAnnotations(annotation.Value, annotationPath.Child("value"))...)
		}

		// validates provided labels.
		for labelIndex, label := range rule.Overriders.LabelsOverrider {
			labelPath := rulePath.Child("overriders").Child("labelsOverrider").Index(labelIndex)
			allErrs = append(allErrs, metav1validation.ValidateLabels(label.Value, labelPath.Child("value"))...)
		}

		// validates predicate path.
		for imageIndex, image := range rule.Overriders.ImageOverrider {
			imagePath := rulePath.Child("overriders").Child("imageOverrider").Index(imageIndex)
			if image.Predicate != nil && !strings.HasPrefix(image.Predicate.Path, "/") {
				allErrs = append(allErrs, field.Invalid(imagePath.Child("predicate").Child("path"), image.Predicate.Path, "path should be start with / character"))
			}
		}

		// validates the targetCluster.
		allErrs = append(allErrs, ValidateClusterAffinity(rule.TargetCluster, rulePath.Child("targetCluster"))...)
	}
	return allErrs
}
