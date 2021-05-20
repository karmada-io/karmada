package helper

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// DenyReasonResourceSelectorsModify constructs a reason indicating that modify ResourceSelectors is not allowed.
const DenyReasonResourceSelectorsModify = "modify ResourceSelectors is forbidden"

// SetDefaultSpreadConstraints set default spread constraints if both 'SpreadByField' and 'SpreadByLabel' not set.
func SetDefaultSpreadConstraints(spreadConstraints []policyv1alpha1.SpreadConstraint) {
	for i := range spreadConstraints {
		if len(spreadConstraints[i].SpreadByLabel) == 0 && len(spreadConstraints[i].SpreadByField) == 0 {
			klog.Infof("Setting default SpreadByField with %s", policyv1alpha1.SpreadByFieldCluster)
			spreadConstraints[i].SpreadByField = policyv1alpha1.SpreadByFieldCluster
		}

		if spreadConstraints[i].MinGroups == 0 {
			klog.Infof("Setting default MinGroups to 1")
			spreadConstraints[i].MinGroups = 1
		}
	}
}

// ValidateSpreadConstraint tests if the constraints is valid.
func ValidateSpreadConstraint(spreadConstraints []policyv1alpha1.SpreadConstraint) error {
	// SpreadByField and SpreadByLabel should not co-exist
	for _, constraint := range spreadConstraints {
		if len(constraint.SpreadByField) > 0 && len(constraint.SpreadByLabel) > 0 {
			return fmt.Errorf("invalid constraints: SpreadByLabel(%s) should not co-exist with spreadByField(%s)", constraint.SpreadByLabel, constraint.SpreadByField)
		}

		// If MaxGroups provided, it should greater or equal than MinGroups.
		if constraint.MaxGroups > 0 && constraint.MaxGroups < constraint.MinGroups {
			return fmt.Errorf("maxGroups(%d) lower than minGroups(%d) is not allowed", constraint.MaxGroups, constraint.MinGroups)
		}
	}
	return nil
}

// IsDependentOverridesPresent checks if a PropagationPolicy's dependent OverridePolicy all exist.
func IsDependentOverridesPresent(c client.Client, policy *policyv1alpha1.PropagationPolicy) (bool, error) {
	ns := policy.Namespace
	for _, override := range policy.Spec.DependentOverrides {
		exist, err := IsOverridePolicyExist(c, ns, override)
		if err != nil {
			return false, err
		}
		if !exist {
			return false, nil
		}
	}

	return true, nil
}

// IsDependentClusterOverridesPresent checks if a ClusterPropagationPolicy's dependent ClusterOverridePolicy all exist.
func IsDependentClusterOverridesPresent(c client.Client, policy *policyv1alpha1.ClusterPropagationPolicy) (bool, error) {
	for _, override := range policy.Spec.DependentOverrides {
		exist, err := IsClusterOverridePolicyExist(c, override)
		if err != nil {
			return false, err
		}
		if !exist {
			return false, nil
		}
	}

	return true, nil
}

// ValidateFieldSelector tests if the fieldSelector is valid.
func ValidateFieldSelector(fieldSelector *policyv1alpha1.FieldSelector) error {
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
		case v1.NodeSelectorOpIn, v1.NodeSelectorOpNotIn:
		default:
			return fmt.Errorf("unsupported operator %q, must be In or NotIn", matchExpression.Operator)
		}
	}

	return nil
}
