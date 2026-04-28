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

package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

func TestIsCronFederatedHPARuleSuspend(t *testing.T) {
	tests := []struct {
		name     string
		rule     autoscalingv1alpha1.CronFederatedHPARule
		expected bool
	}{
		{
			name:     "suspend is nil",
			rule:     autoscalingv1alpha1.CronFederatedHPARule{},
			expected: false,
		},
		{
			name: "suspend is true",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				Suspend: ptr.To(true),
			},
			expected: true,
		},
		{
			name: "suspend is false",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				Suspend: ptr.To(false),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCronFederatedHPARuleSuspend(tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCronFederatedHPASuccessHistoryLimits(t *testing.T) {
	tests := []struct {
		name     string
		rule     autoscalingv1alpha1.CronFederatedHPARule
		expected int
	}{
		{
			name:     "returns default limit when history limit is unspecified",
			rule:     autoscalingv1alpha1.CronFederatedHPARule{},
			expected: DefaultHistoryLimit,
		},
		{
			name: "returns custom limit when specified",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				SuccessfulHistoryLimit: ptr.To[int32](5),
			},
			expected: 5,
		},
		{
			name: "returns zero when limit is explicitly set to zero",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				SuccessfulHistoryLimit: ptr.To[int32](0),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCronFederatedHPASuccessHistoryLimits(tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCronFederatedHPAFailedHistoryLimits(t *testing.T) {
	tests := []struct {
		name     string
		rule     autoscalingv1alpha1.CronFederatedHPARule
		expected int
	}{
		{
			name:     "returns default limit when history limit is unspecified",
			rule:     autoscalingv1alpha1.CronFederatedHPARule{},
			expected: DefaultHistoryLimit,
		},
		{
			name: "returns custom limit when specified",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				FailedHistoryLimit: ptr.To[int32](5),
			},
			expected: 5,
		},
		{
			name: "returns zero when limit is explicitly set to zero",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				FailedHistoryLimit: ptr.To[int32](0),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCronFederatedHPAFailedHistoryLimits(tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCronFederatedHPAKey(t *testing.T) {
	tests := []struct {
		name     string
		cronFHPA *autoscalingv1alpha1.CronFederatedHPA
		expected string
	}{
		{
			name: "default namespace",
			cronFHPA: &autoscalingv1alpha1.CronFederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
			},
			expected: "default/test-hpa",
		},
		{
			name: "custom namespace",
			cronFHPA: &autoscalingv1alpha1.CronFederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-hpa",
					Namespace: "karmada-system",
				},
			},
			expected: "karmada-system/custom-hpa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCronFederatedHPAKey(tt.cronFHPA)
			assert.Equal(t, tt.expected, result)
		})
	}
}
