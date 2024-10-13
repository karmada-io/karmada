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

package lifted

import (
	"testing"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

func TestValidateFederatedHPA(t *testing.T) {
	tests := []struct {
		name    string
		fhpa    *autoscalingv1alpha1.FederatedHPA
		wantErr bool
	}{
		{
			name: "valid FederatedHPA",
			fhpa: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
					MinReplicas: ptr.To[int32](1),
					MaxReplicas: 10,
					Metrics: []autoscalingv2.MetricSpec{
						{
							Type: autoscalingv2.ResourceMetricSourceType,
							Resource: &autoscalingv2.ResourceMetricSource{
								Name: "cpu",
								Target: autoscalingv2.MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: ptr.To[int32](50),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name",
			fhpa: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid/name",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
					MinReplicas: ptr.To[int32](1),
					MaxReplicas: 10,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid spec",
			fhpa: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					MinReplicas: ptr.To[int32](0),
					MaxReplicas: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "missing namespace",
			fhpa: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fhpa",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
					MinReplicas: ptr.To[int32](1),
					MaxReplicas: 10,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateFederatedHPA(tt.fhpa)
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateFederatedHPASpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *autoscalingv1alpha1.FederatedHPASpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 10,
				Metrics: []autoscalingv2.MetricSpec{
					{
						Type: autoscalingv2.ResourceMetricSourceType,
						Resource: &autoscalingv2.ResourceMetricSource{
							Name: "cpu",
							Target: autoscalingv2.MetricTarget{
								Type:               autoscalingv2.UtilizationMetricType,
								AverageUtilization: ptr.To[int32](50),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid minReplicas",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](0),
				MaxReplicas: 10,
			},
			wantErr: true,
		},
		{
			name: "maxReplicas less than minReplicas",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](5),
				MaxReplicas: 3,
			},
			wantErr: true,
		},
		{
			name: "invalid scaleTargetRef",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 10,
			},
			wantErr: true,
		},
		{
			name: "invalid metrics",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 10,
				Metrics: []autoscalingv2.MetricSpec{
					{
						Type: autoscalingv2.ResourceMetricSourceType,
						// Missing Resource field to trigger a validation error
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid behavior",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 10,
				Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
					ScaleDown: &autoscalingv2.HPAScalingRules{
						StabilizationWindowSeconds: ptr.To[int32](-1), // Invalid: negative value
					},
				},
			},
			wantErr: true,
		},
		{
			name: "maxReplicas less than 1",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 0,
			},
			wantErr: true,
		},
		{
			name: "minReplicas equals maxReplicas",
			spec: &autoscalingv1alpha1.FederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				MinReplicas: ptr.To[int32](5),
				MaxReplicas: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateFederatedHPASpec(tt.spec, field.NewPath("spec"), 1)
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
				// Check for specific errors
				if tt.name == "invalid minReplicas" {
					assert.Contains(t, errors.ToAggregate().Error(), "minReplicas", "Expected error related to minReplicas")
				}
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateCrossVersionObjectReference(t *testing.T) {
	tests := []struct {
		name    string
		ref     autoscalingv2.CrossVersionObjectReference
		wantErr bool
	}{
		{
			name: "valid reference",
			ref: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment",
				Name: "my-deployment",
			},
			wantErr: false,
		},
		{
			name: "missing kind",
			ref: autoscalingv2.CrossVersionObjectReference{
				Name: "my-deployment",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			ref: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment",
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			ref: autoscalingv2.CrossVersionObjectReference{
				Kind: "Invalid/Kind",
				Name: "my-deployment",
			},
			wantErr: true,
		},
		{
			name: "invalid name",
			ref: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment",
				Name: "my/deployment",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateCrossVersionObjectReference(tt.ref, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateFederatedHPAStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  *autoscalingv2.HorizontalPodAutoscalerStatus
		wantErr bool
	}{
		{
			name: "valid status",
			status: &autoscalingv2.HorizontalPodAutoscalerStatus{
				CurrentReplicas: 3,
				DesiredReplicas: 5,
			},
			wantErr: false,
		},
		{
			name: "negative current replicas",
			status: &autoscalingv2.HorizontalPodAutoscalerStatus{
				CurrentReplicas: -1,
				DesiredReplicas: 5,
			},
			wantErr: true,
		},
		{
			name: "negative desired replicas",
			status: &autoscalingv2.HorizontalPodAutoscalerStatus{
				CurrentReplicas: 3,
				DesiredReplicas: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateFederatedHPAStatus(tt.status)
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateBehavior(t *testing.T) {
	tests := []struct {
		name     string
		behavior *autoscalingv2.HorizontalPodAutoscalerBehavior
		wantErr  bool
	}{
		{
			name: "valid behavior",
			behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](60),
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         100,
							PeriodSeconds: 15,
						},
					},
				},
				ScaleDown: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](300),
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         100,
							PeriodSeconds: 15,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid scale up stabilization window",
			behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](-1),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid scale down policy",
			behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleDown: &autoscalingv2.HPAScalingRules{
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         -1,
							PeriodSeconds: 15,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "nil behavior",
			behavior: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateBehavior(tt.behavior, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}
func TestValidateScalingRules(t *testing.T) {
	validPolicy := autoscalingv2.HPAScalingPolicy{
		Type:          autoscalingv2.PercentScalingPolicy,
		Value:         100,
		PeriodSeconds: 15,
	}

	tests := []struct {
		name    string
		rules   *autoscalingv2.HPAScalingRules
		wantErr bool
	}{
		{
			name:    "nil rules",
			rules:   nil,
			wantErr: false,
		},
		{
			name: "valid rules with Max select policy",
			rules: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To[int32](300),
				SelectPolicy:               ptr.To[autoscalingv2.ScalingPolicySelect](autoscalingv2.MaxChangePolicySelect),
				Policies:                   []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: false,
		},
		{
			name: "valid rules with Min select policy",
			rules: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To[int32](300),
				SelectPolicy:               ptr.To[autoscalingv2.ScalingPolicySelect](autoscalingv2.MinChangePolicySelect),
				Policies:                   []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: false,
		},
		{
			name: "valid rules with Disabled select policy",
			rules: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To[int32](300),
				SelectPolicy:               ptr.To[autoscalingv2.ScalingPolicySelect](autoscalingv2.DisabledPolicySelect),
				Policies:                   []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: false,
		},
		{
			name: "negative stabilization window",
			rules: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To[int32](-1),
				Policies:                   []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: true,
		},
		{
			name: "stabilization window exceeding max",
			rules: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To[int32](MaxStabilizationWindowSeconds + 1),
				Policies:                   []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: true,
		},
		{
			name: "invalid select policy",
			rules: &autoscalingv2.HPAScalingRules{
				SelectPolicy: ptr.To[autoscalingv2.ScalingPolicySelect]("InvalidPolicy"),
				Policies:     []autoscalingv2.HPAScalingPolicy{validPolicy},
			},
			wantErr: true,
		},
		{
			name: "no policies",
			rules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{},
			},
			wantErr: true,
		},
		{
			name: "invalid policy",
			rules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{
						Type:          "InvalidType",
						Value:         0,
						PeriodSeconds: 0,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateScalingRules(tt.rules, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateScalingPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  autoscalingv2.HPAScalingPolicy
		wantErr bool
	}{
		{
			name: "valid pods scaling policy",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         1,
				PeriodSeconds: 15,
			},
			wantErr: false,
		},
		{
			name: "valid percent scaling policy",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PercentScalingPolicy,
				Value:         10,
				PeriodSeconds: 15,
			},
			wantErr: false,
		},
		{
			name: "invalid policy type",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          "InvalidType",
				Value:         1,
				PeriodSeconds: 15,
			},
			wantErr: true,
		},
		{
			name: "zero value",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         0,
				PeriodSeconds: 15,
			},
			wantErr: true,
		},
		{
			name: "negative value",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         -1,
				PeriodSeconds: 15,
			},
			wantErr: true,
		},
		{
			name: "zero period seconds",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         1,
				PeriodSeconds: 0,
			},
			wantErr: true,
		},
		{
			name: "negative period seconds",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         1,
				PeriodSeconds: -1,
			},
			wantErr: true,
		},
		{
			name: "period seconds exceeding max",
			policy: autoscalingv2.HPAScalingPolicy{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         1,
				PeriodSeconds: MaxPeriodSeconds + 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateScalingPolicy(tt.policy, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateMetricSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    autoscalingv2.MetricSpec
		wantErr bool
	}{
		{
			name: "valid resource metric",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: "cpu",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty metric type",
			spec:    autoscalingv2.MetricSpec{},
			wantErr: true,
		},
		{
			name: "invalid metric type",
			spec: autoscalingv2.MetricSpec{
				Type: "InvalidType",
			},
			wantErr: true,
		},
		{
			name: "object metric without object",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ObjectMetricSourceType,
			},
			wantErr: true,
		},
		{
			name: "pods metric without pods",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.PodsMetricSourceType,
			},
			wantErr: true,
		},
		{
			name: "resource metric without resource",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
			},
			wantErr: true,
		},
		{
			name: "container resource metric without container resource",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ContainerResourceMetricSourceType,
			},
			wantErr: true,
		},
		{
			name: "multiple metric sources",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: "cpu",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
				Pods: &autoscalingv2.PodsMetricSource{},
			},
			wantErr: true,
		}, {
			name: "valid object metric",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ObjectMetricSourceType,
				Object: &autoscalingv2.ObjectMetricSource{
					DescribedObject: autoscalingv2.CrossVersionObjectReference{
						Kind: "Service",
						Name: "my-service",
					},
					Metric: autoscalingv2.MetricIdentifier{
						Name: "requests-per-second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:  autoscalingv2.ValueMetricType,
						Value: ptr.To(resource.MustParse("100")),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid container resource metric",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ContainerResourceMetricSourceType,
				ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
					Name:      "cpu",
					Container: "app",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple metric sources - object and container resource",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ObjectMetricSourceType,
				Object: &autoscalingv2.ObjectMetricSource{
					DescribedObject: autoscalingv2.CrossVersionObjectReference{
						Kind: "Service",
						Name: "my-service",
					},
					Metric: autoscalingv2.MetricIdentifier{
						Name: "requests-per-second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:  autoscalingv2.ValueMetricType,
						Value: ptr.To(resource.MustParse("100")),
					},
				},
				ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
					Name:      "cpu",
					Container: "app",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple metric sources - all types",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ObjectMetricSourceType,
				Object: &autoscalingv2.ObjectMetricSource{
					DescribedObject: autoscalingv2.CrossVersionObjectReference{
						Kind: "Service",
						Name: "my-service",
					},
					Metric: autoscalingv2.MetricIdentifier{
						Name: "requests-per-second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:  autoscalingv2.ValueMetricType,
						Value: ptr.To(resource.MustParse("100")),
					},
				},
				Pods: &autoscalingv2.PodsMetricSource{
					Metric: autoscalingv2.MetricIdentifier{
						Name: "packets-per-second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:         autoscalingv2.AverageValueMetricType,
						AverageValue: ptr.To(resource.MustParse("1k")),
					},
				},
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: "cpu",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
				ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
					Name:      "memory",
					Container: "app",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](60),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mismatched type and source - object",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.PodsMetricSourceType,
				Object: &autoscalingv2.ObjectMetricSource{
					DescribedObject: autoscalingv2.CrossVersionObjectReference{
						Kind: "Service",
						Name: "my-service",
					},
					Metric: autoscalingv2.MetricIdentifier{
						Name: "requests-per-second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:  autoscalingv2.ValueMetricType,
						Value: ptr.To(resource.MustParse("100")),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mismatched type and source - container resource",
			spec: autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
					Name:      "cpu",
					Container: "app",
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: ptr.To[int32](50),
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateMetricSpec(tt.spec, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateObjectSource(t *testing.T) {
	tests := []struct {
		name    string
		src     autoscalingv2.ObjectMetricSource
		wantErr bool
	}{
		{
			name: "valid object metric",
			src: autoscalingv2.ObjectMetricSource{
				DescribedObject: autoscalingv2.CrossVersionObjectReference{
					Kind: "Service",
					Name: "my-service",
				},
				Metric: autoscalingv2.MetricIdentifier{
					Name: "requests-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type:  autoscalingv2.ValueMetricType,
					Value: ptr.To(resource.MustParse("100")),
				},
			},
			wantErr: false,
		},
		{
			name: "missing described object",
			src: autoscalingv2.ObjectMetricSource{
				Metric: autoscalingv2.MetricIdentifier{
					Name: "requests-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type:  autoscalingv2.ValueMetricType,
					Value: ptr.To(resource.MustParse("100")),
				},
			},
			wantErr: true,
		},
		{
			name: "missing metric name",
			src: autoscalingv2.ObjectMetricSource{
				DescribedObject: autoscalingv2.CrossVersionObjectReference{
					Kind: "Service",
					Name: "my-service",
				},
				Target: autoscalingv2.MetricTarget{
					Type:  autoscalingv2.ValueMetricType,
					Value: ptr.To(resource.MustParse("100")),
				},
			},
			wantErr: true,
		},
		{
			name: "missing target value and average value",
			src: autoscalingv2.ObjectMetricSource{
				DescribedObject: autoscalingv2.CrossVersionObjectReference{
					Kind: "Service",
					Name: "my-service",
				},
				Metric: autoscalingv2.MetricIdentifier{
					Name: "requests-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type: autoscalingv2.ValueMetricType,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateObjectSource(&tt.src, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidatePodsSource(t *testing.T) {
	tests := []struct {
		name    string
		src     autoscalingv2.PodsMetricSource
		wantErr bool
	}{
		{
			name: "valid pods metric",
			src: autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{
					Name: "packets-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type:         autoscalingv2.AverageValueMetricType,
					AverageValue: ptr.To(resource.MustParse("1k")),
				},
			},
			wantErr: false,
		},
		{
			name: "valid pods metric with selector",
			src: autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{
					Name: "packets-per-second",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "web"},
					},
				},
				Target: autoscalingv2.MetricTarget{
					Type:         autoscalingv2.AverageValueMetricType,
					AverageValue: ptr.To(resource.MustParse("1k")),
				},
			},
			wantErr: false,
		},
		{
			name: "missing metric name",
			src: autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{},
				Target: autoscalingv2.MetricTarget{
					Type:         autoscalingv2.AverageValueMetricType,
					AverageValue: ptr.To(resource.MustParse("1k")),
				},
			},
			wantErr: true,
		},
		{
			name: "missing average value",
			src: autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{
					Name: "packets-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type: autoscalingv2.AverageValueMetricType,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid target type",
			src: autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{
					Name: "packets-per-second",
				},
				Target: autoscalingv2.MetricTarget{
					Type:  autoscalingv2.UtilizationMetricType,
					Value: ptr.To(resource.MustParse("1k")),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validatePodsSource(&tt.src, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateContainerResourceSource(t *testing.T) {
	tests := []struct {
		name    string
		src     autoscalingv2.ContainerResourceMetricSource
		wantErr bool
	}{
		{
			name: "valid container resource metric",
			src: autoscalingv2.ContainerResourceMetricSource{
				Name:      "cpu",
				Container: "app",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
				},
			},
			wantErr: false,
		},
		{
			name: "missing resource name",
			src: autoscalingv2.ContainerResourceMetricSource{
				Container: "app",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
				},
			},
			wantErr: true,
		},
		{
			name: "missing container name",
			src: autoscalingv2.ContainerResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
				},
			},
			wantErr: true,
		},
		{
			name: "both average utilization and average value set",
			src: autoscalingv2.ContainerResourceMetricSource{
				Name:      "cpu",
				Container: "app",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
					AverageValue:       ptr.To(resource.MustParse("100m")),
				},
			},
			wantErr: true,
		},
		{
			name: "neither average utilization nor average value set",
			src: autoscalingv2.ContainerResourceMetricSource{
				Name:      "cpu",
				Container: "app",
				Target: autoscalingv2.MetricTarget{
					Type: autoscalingv2.UtilizationMetricType,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContainerResourceSource(&tt.src, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateResourceSource(t *testing.T) {
	tests := []struct {
		name    string
		src     autoscalingv2.ResourceMetricSource
		wantErr bool
	}{
		{
			name: "valid utilization",
			src: autoscalingv2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
				},
			},
			wantErr: false,
		},
		{
			name: "valid average value",
			src: autoscalingv2.ResourceMetricSource{
				Name: "memory",
				Target: autoscalingv2.MetricTarget{
					Type:         autoscalingv2.AverageValueMetricType,
					AverageValue: ptr.To(resource.MustParse("100Mi")),
				},
			},
			wantErr: false,
		},
		{
			name: "empty resource name",
			src: autoscalingv2.ResourceMetricSource{
				Name: "",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
				},
			},
			wantErr: true,
		},
		{
			name: "missing target",
			src: autoscalingv2.ResourceMetricSource{
				Name: "cpu",
			},
			wantErr: true,
		},
		{
			name: "both utilization and value set",
			src: autoscalingv2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: ptr.To[int32](50),
					AverageValue:       ptr.To(resource.MustParse("100m")),
				},
			},
			wantErr: true,
		},
		{
			name: "neither utilization nor value set",
			src: autoscalingv2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type: autoscalingv2.UtilizationMetricType,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateResourceSource(&tt.src, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateMetricTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  autoscalingv2.MetricTarget
		wantErr bool
	}{
		{
			name: "valid utilization target",
			target: autoscalingv2.MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: ptr.To[int32](80),
			},
			wantErr: false,
		},
		{
			name: "valid value target",
			target: autoscalingv2.MetricTarget{
				Type:  autoscalingv2.ValueMetricType,
				Value: ptr.To(resource.MustParse("100")),
			},
			wantErr: false,
		},
		{
			name: "valid average value target",
			target: autoscalingv2.MetricTarget{
				Type:         autoscalingv2.AverageValueMetricType,
				AverageValue: ptr.To(resource.MustParse("50")),
			},
			wantErr: false,
		},
		{
			name: "missing type",
			target: autoscalingv2.MetricTarget{
				AverageUtilization: ptr.To[int32](80),
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			target: autoscalingv2.MetricTarget{
				Type:               "InvalidType",
				AverageUtilization: ptr.To[int32](80),
			},
			wantErr: true,
		},
		{
			name: "negative utilization",
			target: autoscalingv2.MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: ptr.To[int32](-1),
			},
			wantErr: true,
		},
		{
			name: "negative value",
			target: autoscalingv2.MetricTarget{
				Type:  autoscalingv2.ValueMetricType,
				Value: ptr.To(resource.MustParse("-100")),
			},
			wantErr: true,
		},
		{
			name: "negative average value",
			target: autoscalingv2.MetricTarget{
				Type:         autoscalingv2.AverageValueMetricType,
				AverageValue: ptr.To(resource.MustParse("-50")),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateMetricTarget(tt.target, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}

func TestValidateMetricIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		id      autoscalingv2.MetricIdentifier
		wantErr bool
	}{
		{
			name: "valid identifier",
			id: autoscalingv2.MetricIdentifier{
				Name: "my-metric",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			id: autoscalingv2.MetricIdentifier{
				Name: "",
			},
			wantErr: true,
		},
		{
			name: "invalid name",
			id: autoscalingv2.MetricIdentifier{
				Name: "my/metric",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateMetricIdentifier(tt.id, field.NewPath("test"))
			if tt.wantErr {
				assert.NotEmpty(t, errors, "Expected validation errors, but got none")
			} else {
				assert.Empty(t, errors, "Expected no validation errors, but got: %v", errors)
			}
		})
	}
}
