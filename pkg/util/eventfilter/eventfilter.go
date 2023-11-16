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
