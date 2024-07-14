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

package util

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

const (
	fakeEventSuccessReason = "Success"
	fakeEventFailedReason  = "Failed"
	fakeSuccessMessage     = "Successful event message"
	fakeFailedMessage      = "Failed event message"
)

func TestIsConditionsEqual(t *testing.T) {
	tests := []struct {
		name                       string
		newCondition, oldCondition metav1.Condition
		expected                   bool
	}{
		{
			name:         "Case 1: not equal",
			newCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionTrue),
			oldCondition: NewCondition(workv1alpha2.Scheduled, fakeEventFailedReason, fakeFailedMessage, metav1.ConditionFalse),
			expected:     false,
		},
		{
			name:         "Case 2: not equal",
			newCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionTrue),
			oldCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeFailedMessage, metav1.ConditionFalse),
			expected:     false,
		},
		{
			name:         "Case 3: not equal",
			newCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionTrue),
			oldCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionFalse),
			expected:     false,
		},
		{
			name:         "equal",
			newCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionTrue),
			oldCondition: NewCondition(workv1alpha2.Scheduled, fakeEventSuccessReason, fakeSuccessMessage, metav1.ConditionTrue),
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := IsConditionsEqual(tt.newCondition, tt.oldCondition)
			if res != tt.expected {
				t.Errorf("IsConditionsEqual() = %v, want %v", res, tt.expected)
			}
		})
	}
}
