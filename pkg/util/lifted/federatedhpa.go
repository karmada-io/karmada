package lifted

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

// This code is lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/autoscaling/v2/defaults.go

// DefaultCPUUtilization is the default value for CPU utilization, provided no other
// metrics are present.  This is here because it's used by both the v2beta1 defaulting
// logic, and the pseudo-defaulting done in v1 conversion.
const DefaultCPUUtilization = 80

var (
	// These constants repeats previous HPA behavior
	scaleUpLimitPercent         int32 = 100
	scaleUpLimitMinimumPods     int32 = 4
	scaleUpPeriod               int32 = 15
	scaleUpStabilizationSeconds int32
	maxPolicy                   = autoscalingv2.MaxChangePolicySelect
	defaultHPAScaleUpRules      = autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: &scaleUpStabilizationSeconds,
		SelectPolicy:               &maxPolicy,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         scaleUpLimitMinimumPods,
				PeriodSeconds: scaleUpPeriod,
			},
			{
				Type:          autoscalingv2.PercentScalingPolicy,
				Value:         scaleUpLimitPercent,
				PeriodSeconds: scaleUpPeriod,
			},
		},
	}
	scaleDownPeriod int32 = 15
	// Currently we can set the downscaleStabilizationWindow from the command line
	// So we can not rewrite the command line option from here
	scaleDownLimitPercent    int32 = 100
	defaultHPAScaleDownRules       = autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: nil,
		SelectPolicy:               &maxPolicy,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PercentScalingPolicy,
				Value:         scaleDownLimitPercent,
				PeriodSeconds: scaleDownPeriod,
			},
		},
	}
)

func SetDefaultsFederatedHPA(obj *autoscalingv1alpha1.FederatedHPA) {
	if obj.Spec.MinReplicas == nil {
		obj.Spec.MinReplicas = pointer.Int32(1)
	}

	if len(obj.Spec.Metrics) == 0 {
		utilizationDefaultVal := int32(DefaultCPUUtilization)
		obj.Spec.Metrics = []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &utilizationDefaultVal,
					},
				},
			},
		}
	}
	SetDefaultsHorizontalPodAutoscalerBehavior(obj)
}

// SetDefaultsHorizontalPodAutoscalerBehavior fills the behavior if it is not null
func SetDefaultsHorizontalPodAutoscalerBehavior(obj *autoscalingv1alpha1.FederatedHPA) {
	// if behavior is specified, we should fill all the 'nil' values with the default ones
	if obj.Spec.Behavior != nil {
		obj.Spec.Behavior.ScaleUp = GenerateHPAScaleUpRules(obj.Spec.Behavior.ScaleUp)
		obj.Spec.Behavior.ScaleDown = GenerateHPAScaleDownRules(obj.Spec.Behavior.ScaleDown)
	}
}

// GenerateHPAScaleUpRules returns a fully-initialized HPAScalingRules value
// We guarantee that no pointer in the structure will have the 'nil' value
func GenerateHPAScaleUpRules(scalingRules *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	defaultScalingRules := defaultHPAScaleUpRules.DeepCopy()
	return copyHPAScalingRules(scalingRules, defaultScalingRules)
}

// GenerateHPAScaleDownRules returns a fully-initialized HPAScalingRules value
// We guarantee that no pointer in the structure will have the 'nil' value
// EXCEPT StabilizationWindowSeconds, for reasoning check the comment for defaultHPAScaleDownRules
func GenerateHPAScaleDownRules(scalingRules *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	defaultScalingRules := defaultHPAScaleDownRules.DeepCopy()
	return copyHPAScalingRules(scalingRules, defaultScalingRules)
}

// copyHPAScalingRules copies all non-`nil` fields in HPA constraint structure
func copyHPAScalingRules(from, to *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	if from == nil {
		return to
	}
	if from.SelectPolicy != nil {
		to.SelectPolicy = from.SelectPolicy
	}
	if from.StabilizationWindowSeconds != nil {
		to.StabilizationWindowSeconds = from.StabilizationWindowSeconds
	}
	if from.Policies != nil {
		to.Policies = from.Policies
	}
	return to
}
