/*
Copyright 2021 The Kubernetes Authors.

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
	"k8s.io/utils/ptr"
)

// This code is lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/autoscaling/v2/defaults_test.go

func TestGenerateScaleDownRules(t *testing.T) {
	type TestCase struct {
		rateDownPods                 int32
		rateDownPodsPeriodSeconds    int32
		rateDownPercent              int32
		rateDownPercentPeriodSeconds int32
		stabilizationSeconds         *int32
		selectPolicy                 *autoscalingv2.ScalingPolicySelect

		expectedPolicies      []autoscalingv2.HPAScalingPolicy
		expectedStabilization *int32
		expectedSelectPolicy  string
		annotation            string
	}
	maxPolicy := autoscalingv2.MaxChangePolicySelect
	minPolicy := autoscalingv2.MinChangePolicySelect
	tests := []TestCase{
		{
			annotation: "Default values",
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
			},
			expectedStabilization: nil,
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:                   "All parameters are specified",
			rateDownPods:                 1,
			rateDownPodsPeriodSeconds:    2,
			rateDownPercent:              3,
			rateDownPercentPeriodSeconds: 4,
			stabilizationSeconds:         ptr.To[int32](25),
			selectPolicy:                 &maxPolicy,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 1, PeriodSeconds: 2},
				{Type: autoscalingv2.PercentScalingPolicy, Value: 3, PeriodSeconds: 4},
			},
			expectedStabilization: ptr.To[int32](25),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:                   "Percent policy is specified",
			rateDownPercent:              1,
			rateDownPercentPeriodSeconds: 2,
			selectPolicy:                 &minPolicy,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PercentScalingPolicy, Value: 1, PeriodSeconds: 2},
			},
			expectedStabilization: nil,
			expectedSelectPolicy:  string(autoscalingv2.MinChangePolicySelect),
		},
		{
			annotation:                "Pods policy is specified",
			rateDownPods:              3,
			rateDownPodsPeriodSeconds: 4,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 3, PeriodSeconds: 4},
			},
			expectedStabilization: nil,
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
	}
	for _, tc := range tests {
		t.Run(tc.annotation, func(t *testing.T) {
			scaleDownRules := &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: tc.stabilizationSeconds,
				SelectPolicy:               tc.selectPolicy,
			}
			if tc.rateDownPods != 0 || tc.rateDownPodsPeriodSeconds != 0 {
				scaleDownRules.Policies = append(scaleDownRules.Policies, autoscalingv2.HPAScalingPolicy{
					Type: autoscalingv2.PodsScalingPolicy, Value: tc.rateDownPods, PeriodSeconds: tc.rateDownPodsPeriodSeconds,
				})
			}
			if tc.rateDownPercent != 0 || tc.rateDownPercentPeriodSeconds != 0 {
				scaleDownRules.Policies = append(scaleDownRules.Policies, autoscalingv2.HPAScalingPolicy{
					Type: autoscalingv2.PercentScalingPolicy, Value: tc.rateDownPercent, PeriodSeconds: tc.rateDownPercentPeriodSeconds,
				})
			}
			down := GenerateHPAScaleDownRules(scaleDownRules)
			assert.EqualValues(t, tc.expectedPolicies, down.Policies)
			if tc.expectedStabilization != nil {
				assert.Equal(t, *tc.expectedStabilization, *down.StabilizationWindowSeconds)
			} else {
				assert.Equal(t, tc.expectedStabilization, down.StabilizationWindowSeconds)
			}
			assert.Equal(t, autoscalingv2.ScalingPolicySelect(tc.expectedSelectPolicy), *down.SelectPolicy)
		})
	}
}

func TestGenerateScaleUpRules(t *testing.T) {
	type TestCase struct {
		rateUpPods                 int32
		rateUpPodsPeriodSeconds    int32
		rateUpPercent              int32
		rateUpPercentPeriodSeconds int32
		stabilizationSeconds       *int32
		selectPolicy               *autoscalingv2.ScalingPolicySelect

		expectedPolicies      []autoscalingv2.HPAScalingPolicy
		expectedStabilization *int32
		expectedSelectPolicy  string
		annotation            string
	}
	maxPolicy := autoscalingv2.MaxChangePolicySelect
	minPolicy := autoscalingv2.MinChangePolicySelect
	tests := []TestCase{
		{
			annotation: "Default values",
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 15},
				{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
			},
			expectedStabilization: ptr.To[int32](0),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:                 "All parameters are specified",
			rateUpPods:                 1,
			rateUpPodsPeriodSeconds:    2,
			rateUpPercent:              3,
			rateUpPercentPeriodSeconds: 4,
			stabilizationSeconds:       ptr.To[int32](25),
			selectPolicy:               &maxPolicy,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 1, PeriodSeconds: 2},
				{Type: autoscalingv2.PercentScalingPolicy, Value: 3, PeriodSeconds: 4},
			},
			expectedStabilization: ptr.To[int32](25),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:              "Pod policy is specified",
			rateUpPods:              1,
			rateUpPodsPeriodSeconds: 2,
			selectPolicy:            &minPolicy,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 1, PeriodSeconds: 2},
			},
			expectedStabilization: ptr.To[int32](0),
			expectedSelectPolicy:  string(autoscalingv2.MinChangePolicySelect),
		},
		{
			annotation:                 "Percent policy is specified",
			rateUpPercent:              7,
			rateUpPercentPeriodSeconds: 10,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PercentScalingPolicy, Value: 7, PeriodSeconds: 10},
			},
			expectedStabilization: ptr.To[int32](0),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:              "Pod policy and stabilization window are specified",
			rateUpPodsPeriodSeconds: 2,
			stabilizationSeconds:    ptr.To[int32](25),
			rateUpPods:              4,
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 2},
			},
			expectedStabilization: ptr.To[int32](25),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
		{
			annotation:                 "Percent policy and stabilization window are specified",
			rateUpPercent:              7,
			rateUpPercentPeriodSeconds: 60,
			stabilizationSeconds:       ptr.To[int32](25),
			expectedPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PercentScalingPolicy, Value: 7, PeriodSeconds: 60},
			},
			expectedStabilization: ptr.To[int32](25),
			expectedSelectPolicy:  string(autoscalingv2.MaxChangePolicySelect),
		},
	}
	for _, tc := range tests {
		t.Run(tc.annotation, func(t *testing.T) {
			scaleUpRules := &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: tc.stabilizationSeconds,
				SelectPolicy:               tc.selectPolicy,
			}
			if tc.rateUpPods != 0 || tc.rateUpPodsPeriodSeconds != 0 {
				scaleUpRules.Policies = append(scaleUpRules.Policies, autoscalingv2.HPAScalingPolicy{
					Type: autoscalingv2.PodsScalingPolicy, Value: tc.rateUpPods, PeriodSeconds: tc.rateUpPodsPeriodSeconds,
				})
			}
			if tc.rateUpPercent != 0 || tc.rateUpPercentPeriodSeconds != 0 {
				scaleUpRules.Policies = append(scaleUpRules.Policies, autoscalingv2.HPAScalingPolicy{
					Type: autoscalingv2.PercentScalingPolicy, Value: tc.rateUpPercent, PeriodSeconds: tc.rateUpPercentPeriodSeconds,
				})
			}
			up := GenerateHPAScaleUpRules(scaleUpRules)
			assert.Equal(t, tc.expectedPolicies, up.Policies)
			if tc.expectedStabilization != nil {
				assert.Equal(t, *tc.expectedStabilization, *up.StabilizationWindowSeconds)
			} else {
				assert.Equal(t, tc.expectedStabilization, up.StabilizationWindowSeconds)
			}

			assert.Equal(t, autoscalingv2.ScalingPolicySelect(tc.expectedSelectPolicy), *up.SelectPolicy)
		})
	}
}
