package lifted

import (
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	pathvalidation "k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/autoscaling/validation/validation.go

const (
	// MaxPeriodSeconds is the largest allowed scaling policy period (in seconds)
	MaxPeriodSeconds int32 = 1800
	// MaxStabilizationWindowSeconds is the largest allowed stabilization window (in seconds)
	MaxStabilizationWindowSeconds int32 = 3600
)

// ValidateFederatedHPAName can be used to check whether the given autoscaler name is valid.
// Prefix indicates this name will be used as part of generation, in which case trailing dashes are allowed.
var ValidateFederatedHPAName = apivalidation.NameIsDNSSubdomain

func ValidateFederatedHPA(fhpa *autoscalingv1alpha1.FederatedHPA) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, ValidateObjectMeta(&fhpa.ObjectMeta, true, ValidateFederatedHPAName, field.NewPath("metadata"))...)

	// MinReplicasLowerBound represents a minimum value for minReplicas
	// 0 when HPA scale-to-zero feature is enabled
	// Karmada does not support HPA scale to zero temporarily
	minReplicasLowerBound := int32(1)
	errs = append(errs, validateFederatedHPASpec(&fhpa.Spec, field.NewPath("spec"), minReplicasLowerBound)...)

	errs = append(errs, validateFederatedHPAStatus(&fhpa.Status)...)
	return errs
}

func validateFederatedHPASpec(fhpaSpec *autoscalingv1alpha1.FederatedHPASpec, fldPath *field.Path, minReplicasLowerBound int32) field.ErrorList {
	allErrs := field.ErrorList{}

	if fhpaSpec.MinReplicas != nil && *fhpaSpec.MinReplicas < minReplicasLowerBound {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minReplicas"), *fhpaSpec.MinReplicas,
			fmt.Sprintf("must be greater than or equal to %d", minReplicasLowerBound)))
	}
	if fhpaSpec.MaxReplicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), fhpaSpec.MaxReplicas, "must be greater than 0"))
	}
	if fhpaSpec.MinReplicas != nil && fhpaSpec.MaxReplicas < *fhpaSpec.MinReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), fhpaSpec.MaxReplicas, "must be greater than or equal to `minReplicas`"))
	}
	if refErrs := ValidateCrossVersionObjectReference(fhpaSpec.ScaleTargetRef, fldPath.Child("scaleTargetRef")); len(refErrs) > 0 {
		allErrs = append(allErrs, refErrs...)
	}
	if refErrs := validateMetrics(fhpaSpec.Metrics, fldPath.Child("metrics"), fhpaSpec.MinReplicas); len(refErrs) > 0 {
		allErrs = append(allErrs, refErrs...)
	}
	if refErrs := validateBehavior(fhpaSpec.Behavior, fldPath.Child("behavior")); len(refErrs) > 0 {
		allErrs = append(allErrs, refErrs...)
	}
	return allErrs
}

// ValidateCrossVersionObjectReference validates a CrossVersionObjectReference and returns an
// ErrorList with any errors.
func ValidateCrossVersionObjectReference(ref autoscalingv2.CrossVersionObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	} else {
		for _, msg := range pathvalidation.IsValidPathSegmentName(ref.Kind) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), ref.Kind, msg))
		}
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	} else {
		for _, msg := range pathvalidation.IsValidPathSegmentName(ref.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, msg))
		}
	}

	return allErrs
}

// validateFederatedHPAStatus validates an update to status on a FederatedHPA and
// returns an ErrorList with any errors.
func validateFederatedHPAStatus(fhpaStatus *autoscalingv2.HorizontalPodAutoscalerStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(fhpaStatus.CurrentReplicas), field.NewPath("status", "currentReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(fhpaStatus.DesiredReplicas), field.NewPath("status", "desiredReplicas"))...)
	return allErrs
}

func validateMetrics(metrics []autoscalingv2.MetricSpec, fldPath *field.Path, minReplicas *int32) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, metricSpec := range metrics {
		idxPath := fldPath.Index(i)
		if targetErrs := validateMetricSpec(metricSpec, idxPath); len(targetErrs) > 0 {
			allErrs = append(allErrs, targetErrs...)
		}
	}

	return allErrs
}

func validateBehavior(behavior *autoscalingv2.HorizontalPodAutoscalerBehavior, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if behavior != nil {
		if scaleUpErrs := validateScalingRules(behavior.ScaleUp, fldPath.Child("scaleUp")); len(scaleUpErrs) > 0 {
			allErrs = append(allErrs, scaleUpErrs...)
		}
		if scaleDownErrs := validateScalingRules(behavior.ScaleDown, fldPath.Child("scaleDown")); len(scaleDownErrs) > 0 {
			allErrs = append(allErrs, scaleDownErrs...)
		}
	}
	return allErrs
}

var validSelectPolicyTypes = sets.NewString(string(autoscalingv2.MaxChangePolicySelect), string(autoscalingv2.MinChangePolicySelect), string(autoscalingv2.DisabledPolicySelect))
var validSelectPolicyTypesList = validSelectPolicyTypes.List()

func validateScalingRules(rules *autoscalingv2.HPAScalingRules, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if rules != nil {
		if rules.StabilizationWindowSeconds != nil && *rules.StabilizationWindowSeconds < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("stabilizationWindowSeconds"), rules.StabilizationWindowSeconds, "must be greater than or equal to zero"))
		}
		if rules.StabilizationWindowSeconds != nil && *rules.StabilizationWindowSeconds > MaxStabilizationWindowSeconds {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("stabilizationWindowSeconds"), rules.StabilizationWindowSeconds,
				fmt.Sprintf("must be less than or equal to %v", MaxStabilizationWindowSeconds)))
		}
		if rules.SelectPolicy != nil && !validSelectPolicyTypes.Has(string(*rules.SelectPolicy)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("selectPolicy"), rules.SelectPolicy, validSelectPolicyTypesList))
		}
		policiesPath := fldPath.Child("policies")
		if len(rules.Policies) == 0 {
			allErrs = append(allErrs, field.Required(policiesPath, "must specify at least one Policy"))
		}
		for i, policy := range rules.Policies {
			idxPath := policiesPath.Index(i)
			if policyErrs := validateScalingPolicy(policy, idxPath); len(policyErrs) > 0 {
				allErrs = append(allErrs, policyErrs...)
			}
		}
	}
	return allErrs
}

var validPolicyTypes = sets.NewString(string(autoscalingv2.PodsScalingPolicy), string(autoscalingv2.PercentScalingPolicy))
var validPolicyTypesList = validPolicyTypes.List()

func validateScalingPolicy(policy autoscalingv2.HPAScalingPolicy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if policy.Type != autoscalingv2.PodsScalingPolicy && policy.Type != autoscalingv2.PercentScalingPolicy {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), policy.Type, validPolicyTypesList))
	}
	if policy.Value <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("value"), policy.Value, "must be greater than zero"))
	}
	if policy.PeriodSeconds <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("periodSeconds"), policy.PeriodSeconds, "must be greater than zero"))
	}
	if policy.PeriodSeconds > MaxPeriodSeconds {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("periodSeconds"), policy.PeriodSeconds,
			fmt.Sprintf("must be less than or equal to %v", MaxPeriodSeconds)))
	}
	return allErrs
}

// TODO: Karmada only supports cpu/memory resource metrics
var validMetricSourceTypes = sets.NewString(
	string(autoscalingv2.ResourceMetricSourceType))

var validMetricSourceTypesList = validMetricSourceTypes.List()

func validateMetricSpec(spec autoscalingv2.MetricSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(string(spec.Type)) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a metric source type"))
	}

	if !validMetricSourceTypes.Has(string(spec.Type)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), spec.Type, validMetricSourceTypesList))
	}

	typesPresent := sets.NewString()
	if spec.Resource != nil {
		typesPresent.Insert("resource")
		if typesPresent.Len() == 1 {
			allErrs = append(allErrs, validateResourceSource(spec.Resource, fldPath.Child("resource"))...)
		}
	}

	var expectedField string
	switch spec.Type {

	// TODO: Karmada only support resource metrics temporarily
	case autoscalingv2.ResourceMetricSourceType:
		if spec.Resource == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("resource"), "must populate information for the given metric source"))
		}
		expectedField = "resource"
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), spec.Type, validMetricSourceTypesList))
	}

	if typesPresent.Len() != 1 {
		typesPresent.Delete(expectedField)
		for typ := range typesPresent {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(typ), "must populate the given metric source only"))
		}
	}

	return allErrs
}

func validateResourceSource(src *autoscalingv2.ResourceMetricSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(src.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must specify a resource name"))
	}

	allErrs = append(allErrs, validateMetricTarget(src.Target, fldPath.Child("target"))...)

	if src.Target.AverageUtilization == nil && src.Target.AverageValue == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("target").Child("averageUtilization"), "must set either a target raw value or a target utilization"))
	}

	if src.Target.AverageUtilization != nil && src.Target.AverageValue != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("target").Child("averageValue"), "may not set both a target raw value and a target utilization"))
	}

	return allErrs
}

func validateMetricTarget(mt autoscalingv2.MetricTarget, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(mt.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a metric target type"))
	}

	if mt.Type != autoscalingv2.UtilizationMetricType &&
		mt.Type != autoscalingv2.ValueMetricType &&
		mt.Type != autoscalingv2.AverageValueMetricType {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), mt.Type, "must be either Utilization, Value, or AverageValue"))
	}

	if mt.Value != nil && mt.Value.Sign() != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("value"), mt.Value, "must be positive"))
	}

	if mt.AverageValue != nil && mt.AverageValue.Sign() != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("averageValue"), mt.AverageValue, "must be positive"))
	}

	if mt.AverageUtilization != nil && *mt.AverageUtilization < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("averageUtilization"), mt.AverageUtilization, "must be greater than 0"))
	}

	return allErrs
}
