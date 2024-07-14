/*
Copyright 2024 The Karmada Authors.

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
	"errors"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// VerifyDependencies verifies dependencies.
func VerifyDependencies(dependencies []configv1alpha1.DependentObjectReference) error {
	var errs []error
	for _, dependency := range dependencies {
		if len(dependency.APIVersion) == 0 || len(dependency.Kind) == 0 {
			errs = append(errs, errors.New("dependency missing required apiVersion or kind"))
			continue
		}
		if len(dependency.Name) == 0 && dependency.LabelSelector == nil {
			errs = append(errs, errors.New("dependency can not leave name and labelSelector all empty"))
		}
	}
	return utilerrors.NewAggregate(errs)
}
