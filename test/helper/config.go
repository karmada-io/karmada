package helper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// NewResourceInterpreterCustomization will build a ResourceInterpreterCustomization object.
func NewResourceInterpreterCustomization(
	name string,
	target configv1alpha1.CustomizationTarget,
	rules configv1alpha1.CustomizationRules) *configv1alpha1.ResourceInterpreterCustomization {
	return &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target:         target,
			Customizations: rules,
		},
	}
}
