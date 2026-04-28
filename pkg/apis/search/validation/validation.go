/*
Copyright 2026 The Karmada Authors.

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
	"net/url"

	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	utilvalidation "github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidateResourceRegistry validates a ResourceRegistry object.
func ValidateResourceRegistry(resourceRegistry *searchapis.ResourceRegistry) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(
		&resourceRegistry.ObjectMeta,
		false,
		apimachineryvalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	allErrs = append(allErrs, ValidateResourceRegistrySpec(&resourceRegistry.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateResourceRegistryUpdate validates a ResourceRegistry update.
func ValidateResourceRegistryUpdate(newResourceRegistry, oldResourceRegistry *searchapis.ResourceRegistry) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(
		&newResourceRegistry.ObjectMeta,
		&oldResourceRegistry.ObjectMeta,
		field.NewPath("metadata"),
	)
	allErrs = append(allErrs, ValidateResourceRegistry(newResourceRegistry)...)
	return allErrs
}

// ValidateResourceRegistrySpec validates a ResourceRegistrySpec.
func ValidateResourceRegistrySpec(spec *searchapis.ResourceRegistrySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, utilvalidation.ValidateClusterAffinity(&spec.TargetCluster, fldPath.Child("targetCluster"))...)

	for i := range spec.ResourceSelectors {
		allErrs = append(allErrs, validateResourceSelector(&spec.ResourceSelectors[i], fldPath.Child("resourceSelectors").Index(i))...)
	}

	allErrs = append(allErrs, validateBackendStore(spec.BackendStore, fldPath.Child("backendStore"))...)

	return allErrs
}

// validateResourceSelector validates semantic constraints of a ResourceSelector.
func validateResourceSelector(selector *searchapis.ResourceSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if selector.APIVersion != "" {
		if _, err := schema.ParseGroupVersion(selector.APIVersion); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("apiVersion"), selector.APIVersion, err.Error()))
		}
	}

	if selector.Namespace != "" {
		if errs := apimachineryvalidation.ValidateNamespaceName(selector.Namespace, false); len(errs) > 0 {
			for _, err := range errs {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), selector.Namespace, err))
			}
		}
	}

	return allErrs
}

// validateBackendStore validates semantic constraints of a BackendStoreConfig.
func validateBackendStore(backendStore *searchapis.BackendStoreConfig, fldPath *field.Path) field.ErrorList {
	if backendStore == nil || backendStore.OpenSearch == nil {
		return nil
	}

	allErrs := field.ErrorList{}
	openSearchPath := fldPath.Child("openSearch")
	for i, address := range backendStore.OpenSearch.Addresses {
		u, err := url.Parse(address)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(openSearchPath.Child("addresses").Index(i), address, err.Error()))
			continue
		}

		if u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			allErrs = append(allErrs, field.Invalid(
				openSearchPath.Child("addresses").Index(i),
				address,
				"must be a valid http or https URL with non-empty scheme and host",
			))
		}
	}

	return allErrs
}
