/*
Copyright 2021 The Karmada Authors.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NewCondition returns a new condition object.
func NewCondition(conditionType, reason, message string, status metav1.ConditionStatus) metav1.Condition {
	return metav1.Condition{
		Type:    conditionType,
		Reason:  reason,
		Status:  status,
		Message: message,
	}
}

// IsConditionsEqual compares the given condition's Status, Reason and Message.
func IsConditionsEqual(newCondition, oldCondition metav1.Condition) bool {
	return newCondition.Status == oldCondition.Status &&
		newCondition.Reason == oldCondition.Reason &&
		newCondition.Message == oldCondition.Message
}
