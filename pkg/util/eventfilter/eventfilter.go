package eventfilter

import (
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SpecificationChanged check if the specification of the given object change or not
func SpecificationChanged(oldObj, newObj *unstructured.Unstructured) bool {
	oldBackup := oldObj.DeepCopy()
	newBackup := newObj.DeepCopy()

	// Remove the status and some system defined mutable fields in metadata, including managedFields and resourceVersion.
	// Refer to https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta for more details.
	removedFields := [][]string{{"status"}, {"metadata", "managedFields"}, {"metadata", "resourceVersion"}}
	for _, r := range removedFields {
		unstructured.RemoveNestedField(oldBackup.Object, r...)
		unstructured.RemoveNestedField(newBackup.Object, r...)
	}

	return !reflect.DeepEqual(oldBackup, newBackup)
}
