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
