package configuration

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func hasWildcard(slice []string) bool {
	for _, s := range slice {
		if s == "*" {
			return true
		}
	}
	return false
}

func validateRule(rule *configv1alpha1.Rule, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	if len(rule.APIGroups) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("apiGroups"), ""))
	}
	if len(rule.APIGroups) > 1 && hasWildcard(rule.APIGroups) {
		allErrors = append(allErrors, field.Invalid(fldPath.Child("apiGroups"), rule.APIGroups, "if '*' is present, must not specify other API groups"))
	}
	// Note: group could be empty, e.g., the legacy "v1" API. So don't check empty items here.

	if len(rule.APIVersions) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("apiVersions"), ""))
	}
	if len(rule.APIVersions) > 1 && hasWildcard(rule.APIVersions) {
		allErrors = append(allErrors, field.Invalid(fldPath.Child("apiVersions"), rule.APIVersions, "if '*' is present, must not specify other API versions"))
	}
	for i, version := range rule.APIVersions {
		if version == "" {
			allErrors = append(allErrors, field.Required(fldPath.Child("apiVersions").Index(i), ""))
		}
	}

	if len(rule.Kinds) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("kinds"), ""))
	}
	if len(rule.Kinds) > 1 && hasWildcard(rule.Kinds) {
		allErrors = append(allErrors, field.Invalid(fldPath.Child("kinds"), rule.Kinds, "if '*' is present, must not specify other kinds"))
	}
	for i, kind := range rule.Kinds {
		if kind == "" {
			allErrors = append(allErrors, field.Required(fldPath.Child("kinds").Index(i), ""))
		}
	}

	return allErrors
}
