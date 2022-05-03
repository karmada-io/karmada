package util

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetLabelValue retrieves the value via 'labelKey' if exist, otherwise returns an empty string.
func GetLabelValue(labels map[string]string, labelKey string) string {
	if labels == nil {
		return ""
	}

	return labels[labelKey]
}

// MergeLabel adds label for the given object.
func MergeLabel(obj *unstructured.Unstructured, labelKey string, labelValue string) {
	workloadLabel := obj.GetLabels()
	if workloadLabel == nil {
		workloadLabel = make(map[string]string, 1)
	}
	workloadLabel[labelKey] = labelValue
	obj.SetLabels(workloadLabel)
}

// DedupeAndMergeLabels merges the new labels into exist labels.
func DedupeAndMergeLabels(existLabel, newLabel map[string]string) map[string]string {
	if existLabel == nil {
		return newLabel
	}

	for k, v := range newLabel {
		existLabel[k] = v
	}
	return existLabel
}
