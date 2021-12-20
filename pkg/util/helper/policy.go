package helper

import (
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
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

// GetFollowedResourceSelectorsWhenMatchServiceImport get followed derived-service and endpointSlices resource selectors
// when policy's ResourceSelectors contains ResourceSelector, whose kind is ServiceImport.
func GetFollowedResourceSelectorsWhenMatchServiceImport(resourceSelectors []policyv1alpha1.ResourceSelector) []policyv1alpha1.ResourceSelector {
	var addedResourceSelectors []policyv1alpha1.ResourceSelector

	for _, resourceSelector := range resourceSelectors {
		if resourceSelector.Kind != util.ServiceImportKind {
			continue
		}

		if resourceSelector.Namespace == "" || resourceSelector.Name == "" {
			continue
		}

		addedResourceSelectors = append(addedResourceSelectors, GenerateResourceSelectorForServiceImport(resourceSelector)...)
	}

	return addedResourceSelectors
}

// GenerateResourceSelectorForServiceImport generates resource selectors for ServiceImport.
func GenerateResourceSelectorForServiceImport(svcImport policyv1alpha1.ResourceSelector) []policyv1alpha1.ResourceSelector {
	derivedServiceName := names.GenerateDerivedServiceName(svcImport.Name)
	return []policyv1alpha1.ResourceSelector{
		{
			APIVersion: "v1",
			Kind:       util.ServiceKind,
			Namespace:  svcImport.Namespace,
			Name:       derivedServiceName,
		},
		{
			APIVersion: "discovery.k8s.io/v1",
			Kind:       util.EndpointSliceKind,
			Namespace:  svcImport.Namespace,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					discoveryv1.LabelServiceName: derivedServiceName,
				},
			},
		},
	}
}
