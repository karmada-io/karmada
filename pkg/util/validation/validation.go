package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// LabelValueMaxLength is a label's max length
const LabelValueMaxLength int = 63

// ValidatePolicyFieldSelector tests if the fieldSelector of propagation policy is valid.
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
func ValidateSpreadConstraint(spreadConstraints []policyv1alpha1.SpreadConstraint) error {
	spreadByFields := sets.NewString()

	for _, constraint := range spreadConstraints {
		// SpreadByField and SpreadByLabel should not co-exist
		if len(constraint.SpreadByField) > 0 && len(constraint.SpreadByLabel) > 0 {
			return fmt.Errorf("invalid constraints: SpreadByLabel(%s) should not co-exist with spreadByField(%s)", constraint.SpreadByLabel, constraint.SpreadByField)
		}

		// If MaxGroups provided, it should greater or equal than MinGroups.
		if constraint.MaxGroups > 0 && constraint.MaxGroups < constraint.MinGroups {
			return fmt.Errorf("maxGroups(%d) lower than minGroups(%d) is not allowed", constraint.MaxGroups, constraint.MinGroups)
		}

		if len(constraint.SpreadByField) > 0 {
			spreadByFields.Insert(string(constraint.SpreadByField))
		}
	}

	if spreadByFields.Len() > 0 {
		// If one of spread constraints are using 'SpreadByField', the 'SpreadByFieldCluster' must be included.
		// For example, when using 'SpreadByFieldRegion' to specify region groups, at the meantime, you must use
		// 'SpreadByFieldCluster' to specify how many clusters should be selected.
		if !spreadByFields.Has(string(policyv1alpha1.SpreadByFieldCluster)) {
			return fmt.Errorf("the cluster spread constraint must be enabled in one of the constraints in case of SpreadByField is enabled")
		}
	}

	return nil
}

// ValidateOverrideSpec tests if the overrideRules and (overriders or targetCluster) co-exist
func ValidateOverrideSpec(overrideSpec *policyv1alpha1.OverrideSpec) error {
	if overrideSpec == nil {
		return nil
	}

	if overrideSpec.OverrideRules == nil {
		return nil
	}

	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	if overrideSpec.TargetCluster != nil || !EmptyOverrides(overrideSpec.Overriders) {
		return fmt.Errorf("overrideRules and (overriders or targetCluster) can't co-exist")
	}

	return nil
}

// EmptyOverrides check if the overriders of override policy is empty
func EmptyOverrides(overriders policyv1alpha1.Overriders) bool {
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
