/*
Copyright 2022 The Karmada Authors.

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
