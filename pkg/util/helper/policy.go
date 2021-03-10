package helper

import (
	"fmt"

	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
