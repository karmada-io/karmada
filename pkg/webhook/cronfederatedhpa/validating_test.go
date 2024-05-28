/*
Copyright 2023 The Karmada Authors.

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

package cronfederatedhpa

import (
	"fmt"
	"strings"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

func Test_validateCronFederatedHPASpec(t *testing.T) {
	specFld := field.NewPath("spec")

	tests := []struct {
		name        string
		spec        *autoscalingv1alpha1.CronFederatedHPASpec
		expectedErr string
	}{
		{
			name: "normal Deployment",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "foo",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "normal FederatedHPA",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       autoscalingv1alpha1.FederatedHPAKind,
					Name:       "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:              "foo",
						Schedule:          "0 0 13 * 1",
						TargetReplicas:    ptr.To[int32](2),
						TargetMinReplicas: ptr.To[int32](1),
						TargetMaxReplicas: ptr.To[int32](3),
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "unsupported kind in apiVersion: autoscaling.karmada.io/v1alpha1",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       "Test",
					Name:       "foo",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: fmt.Sprintf("invalid scaleTargetRef kind: %s, only support %s", "Test", autoscalingv1alpha1.FederatedHPAKind),
		},
		{
			name: "unspecified spec.scaleTargetRef.kind",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Name: "foo",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: "spec.scaleTargetRef.kind: Required value",
		},
		{
			name: "unspecified spec.scaleTargetRef.name",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: "spec.scaleTargetRef.name: Required value",
		},
		{
			name: "duplicated spec.rules[*].name",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
					Name: "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 5",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: `Duplicate value: "bar"`,
		},
		{
			name: "invalid cron",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
					Name: "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 *",
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: "invalid cron format",
		},
		{
			name: "invalid timezone",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
					Name: "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 1",
						TimeZone:       ptr.To("A/B"),
						TargetReplicas: ptr.To[int32](1),
					},
				},
			},
			expectedErr: "unknown time zone A/B",
		},
		{
			name: "nil targetReplicas",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
					Name: "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:     "bar",
						Schedule: "0 0 13 * 1",
					},
				},
			},
			expectedErr: "targetReplicas cannot be nil if you want to scale foo",
		},
		{
			name: "targetReplicas is less than 0",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "foo",
					Name: "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 1",
						TargetReplicas: ptr.To[int32](-1),
					},
				},
			},
			expectedErr: "targetReplicas should be larger than or equal to 0",
		},
		{
			name: "nil targetMinReplicas and targetMaxReplicas when scaling FHPA",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       autoscalingv1alpha1.FederatedHPAKind,
					Name:       "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:           "bar",
						Schedule:       "0 0 13 * 1",
						TargetReplicas: ptr.To[int32](-1),
					},
				},
			},
			expectedErr: "targetMinReplicas and targetMaxReplicas cannot be nil at the same time if you want to scale FederatedHPA",
		},
		{
			name: "targetMinReplicas is less than 0 when scaling FHPA",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       autoscalingv1alpha1.FederatedHPAKind,
					Name:       "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:              "bar",
						Schedule:          "0 0 13 * 1",
						TargetMinReplicas: ptr.To[int32](-1),
						TargetReplicas:    ptr.To[int32](1),
					},
				},
			},
			expectedErr: "targetMinReplicas should be larger than 0",
		},
		{
			name: "targetMaxReplicas is less than 0 when scaling FHPA",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       autoscalingv1alpha1.FederatedHPAKind,
					Name:       "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:              "bar",
						Schedule:          "0 0 13 * 1",
						TargetMaxReplicas: ptr.To[int32](-1),
						TargetReplicas:    ptr.To[int32](1),
					},
				},
			},
			expectedErr: "targetMaxReplicas should be larger than 0",
		},
		{
			name: "targetMaxReplicas is less than targetMinReplicas when scaling FHPA",
			spec: &autoscalingv1alpha1.CronFederatedHPASpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: autoscalingv1alpha1.GroupVersion.String(),
					Kind:       autoscalingv1alpha1.FederatedHPAKind,
					Name:       "bar",
				},
				Rules: []autoscalingv1alpha1.CronFederatedHPARule{
					{
						Name:              "bar",
						Schedule:          "0 0 13 * 1",
						TargetMinReplicas: ptr.To[int32](3),
						TargetMaxReplicas: ptr.To[int32](1),
						TargetReplicas:    ptr.To[int32](1),
					},
				},
			},
			expectedErr: "targetMaxReplicas should be larger than or equal to targetMinReplicas",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCronFederatedHPASpec(tt.spec, specFld)
			err := errs.ToAggregate()
			if err != nil {
				errStr := err.Error()
				if tt.expectedErr == "" {
					t.Errorf("expected no error:\n  but got:\n  %s", errStr)
				} else if !strings.Contains(errStr, tt.expectedErr) {
					t.Errorf("expected to contain:\n  %s\ngot:\n  %s", tt.expectedErr, errStr)
				}
			} else {
				if tt.expectedErr != "" {
					t.Errorf("unexpected no error, expected to contain:\n  %s", tt.expectedErr)
				}
			}
		})
	}
}
