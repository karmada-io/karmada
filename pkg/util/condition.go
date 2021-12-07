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
