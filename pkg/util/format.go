package util

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RemoveIrrelevantField will delete irrelevant field from workload. such as uid, timestamp, status
func RemoveIrrelevantField(workload *unstructured.Unstructured) {
	unstructured.RemoveNestedField(workload.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(workload.Object, "metadata", "generation")
	unstructured.RemoveNestedField(workload.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(workload.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(workload.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(workload.Object, "metadata", "uid")
	unstructured.RemoveNestedField(workload.Object, "status")
}
