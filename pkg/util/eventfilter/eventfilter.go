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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// labelOrAnnoKeyPrefixByKarmada defines the key prefix used for labels or annotations used by karmada's own components.
const labelOrAnnoKeyPrefixByKarmada = ".karmada.io"

// labelsForUserWithKarmadaPrefix enumerates special cases that labels use the karmada prefix but are really only for users to use.
var labelsForUserWithKarmadaPrefix = map[string]struct{}{
	policyv1alpha1.NamespaceSkipAutoPropagationLabel: {},
}

// SpecificationChanged check if the specification of the given object change or not
func SpecificationChanged(oldObj, newObj *unstructured.Unstructured) bool {
	oldBackup := oldObj.DeepCopy()
	newBackup := newObj.DeepCopy()

	removeIgnoredFields(oldBackup, newBackup)

	return !reflect.DeepEqual(oldBackup, newBackup)
}

// ResourceChangeByKarmada check if the change of the object is modified by Karmada.
// If oldObj deep equal to newObj after removing fields changed by Karmada, such change refers to ChangeByKarmada.
func ResourceChangeByKarmada(oldObj, newObj *unstructured.Unstructured) bool {
	oldBackup := oldObj.DeepCopy()
	newBackup := newObj.DeepCopy()

	removeIgnoredFields(oldBackup, newBackup)

	removeFieldsChangedByKarmada(oldBackup, newBackup)

	return reflect.DeepEqual(oldBackup, newBackup)
}

// removeIgnoredFields Remove the status and some system defined mutable fields in metadata, including managedFields and resourceVersion.
// Refer to https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta for more details.
func removeIgnoredFields(oldBackup, newBackup *unstructured.Unstructured) {
	removedFields := [][]string{{"status"}, {"metadata", "managedFields"}, {"metadata", "resourceVersion"}}
	for _, r := range removedFields {
		unstructured.RemoveNestedField(oldBackup.Object, r...)
		unstructured.RemoveNestedField(newBackup.Object, r...)
	}
}

// removeFieldsChangedByKarmada Remove the fields modified by karmada's own components.
func removeFieldsChangedByKarmada(oldBackup, newBackup *unstructured.Unstructured) {
	// Remove label or annotations changed by Karmada
	removeLabelOrAnnotationByKarmada(oldBackup)
	removeLabelOrAnnotationByKarmada(newBackup)

	// Karmada's change may result in some auto-generated fields being modified indirectly, e.g: metadata.generation
	// They should also be removed before comparing.
	removedFields := [][]string{{"metadata", "generation"}}
	for _, r := range removedFields {
		unstructured.RemoveNestedField(oldBackup.Object, r...)
		unstructured.RemoveNestedField(newBackup.Object, r...)
	}
}

// removeLabelOrAnnotationByKarmada remove the label or annotation modified by karmada's own components.
func removeLabelOrAnnotationByKarmada(unstructuredObj *unstructured.Unstructured) {
	for k := range unstructuredObj.GetLabels() {
		_, isLabelForUser := labelsForUserWithKarmadaPrefix[k]
		if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) && !isLabelForUser {
			unstructured.RemoveNestedField(unstructuredObj.Object, []string{"metadata", "labels", k}...)
		}
	}
	if len(unstructuredObj.GetLabels()) == 0 {
		unstructured.RemoveNestedField(unstructuredObj.Object, []string{"metadata", "labels"}...)
	}

	for k := range unstructuredObj.GetAnnotations() {
		if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) {
			unstructured.RemoveNestedField(unstructuredObj.Object, []string{"metadata", "annotations", k}...)
		}
	}
	if len(unstructuredObj.GetAnnotations()) == 0 {
		unstructured.RemoveNestedField(unstructuredObj.Object, []string{"metadata", "annotations"}...)
	}
}
